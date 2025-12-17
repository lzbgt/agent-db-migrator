package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Gemini structures matching actual JSON format
type GeminiSession struct {
	SessionID   string          `json:"sessionId"`
	StartTime   string          `json:"startTime"`
	LastUpdated string          `json:"lastUpdated"`
	Messages    []GeminiMessage `json:"messages"`
	Summary     string          `json:"summary"`
}

type GeminiMessage struct {
	ID        string           `json:"id"`
	Timestamp string           `json:"timestamp"`
	Type      string           `json:"type"` // "user" or "gemini"
	Content   string           `json:"content"`
	ToolCalls []GeminiToolCall `json:"toolCalls,omitempty"`
	Thoughts  []GeminiThought  `json:"thoughts,omitempty"`
}

type GeminiToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Args   map[string]interface{} `json:"args"`
	Result []GeminiToolResult     `json:"result"`
	Status string                 `json:"status"`
}

type GeminiToolResult struct {
	FunctionResponse GeminiFunctionResponse `json:"functionResponse"`
}

type GeminiFunctionResponse struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type GeminiThought struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
}

func migrateGeminiSessions(db *sql.DB, home string) error {
	geminiTmpDir := filepath.Join(home, ".gemini", "tmp")

	// Map sessionID -> list of file paths
	sessionFiles := make(map[string][]string)

	err := filepath.WalkDir(geminiTmpDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Warning: skipping %s due to error: %v\n", path, err)
			return nil // Skip permission denied errors
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") || !strings.Contains(d.Name(), "session-") {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != "chats" {
			return nil
		}

		// Read file to get Session ID (inefficient but accurate)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var partialSession struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(content, &partialSession); err != nil {
			return nil
		}
		if partialSession.SessionID != "" {
			sessionFiles[partialSession.SessionID] = append(sessionFiles[partialSession.SessionID], path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for sessionID, files := range sessionFiles {
		var bestSession *GeminiSession
		var bestPath string
		maxMessages := -1

		fmt.Printf("Analyzing %d files for session %s\n", len(files), sessionID)

		for _, path := range files {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var session GeminiSession
			if err := json.Unmarshal(content, &session); err != nil {
				continue
			}
			
			if len(session.Messages) > maxMessages {
				maxMessages = len(session.Messages)
				bestSession = &session
				bestPath = path
			}
		}

		if bestSession == nil {
			continue
		}

		// Extract project hash from path
		projectHash := filepath.Base(filepath.Dir(filepath.Dir(bestPath)))
		fmt.Printf("Processing Gemini session: %s (Project: %s, Messages: %d)\n", bestPath, projectHash, maxMessages)

		if err := migrateSingleGeminiSession(db, bestSession, projectHash); err != nil {
			fmt.Printf("  Error migrating session %s: %v\n", sessionID, err)
		}
	}
	return nil
}

func migrateSingleGeminiSession(db *sql.DB, session *GeminiSession, projectHash string) error {
	// 1. Write Meta
	cwd := fmt.Sprintf("gemini-cli:%s", projectHash)
	metaPayload := map[string]interface{}{
		"id":             session.SessionID,
		"timestamp":      session.StartTime,
		"cwd":            cwd,
		"model_provider": "gemini",
		"originator":     "gemini-cli",
	}
	metaJson, _ := json.Marshal(metaPayload)

	_, err := db.Exec(
		"INSERT OR REPLACE INTO sessions (id, timestamp, model, cwd, instructions, json_meta) VALUES (?, ?, ?, ?, ?, ?)",
		session.SessionID, session.StartTime, "gemini-pro", cwd, "", string(metaJson),
	)
	if err != nil {
		return fmt.Errorf("failed to write meta: %w", err)
	}

	// 2. Clear events
	_, err = db.Exec("DELETE FROM events WHERE session_id = ?", session.SessionID)
	if err != nil {
		return err
	}

	// 3. Insert events
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO events (session_id, timestamp, event_type, payload) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range session.Messages {
		ts := msg.Timestamp
		if ts == "" { ts = session.StartTime }

		if msg.Type == "user" {
			payload := map[string]interface{}{
				"type": "user_message",
				"message": msg.Content,
			}
			fullPayload := map[string]interface{}{
				"type": "event_msg",
				"payload": payload,
			}
			jsonBytes, _ := json.Marshal(fullPayload)
			
			_, err = stmt.Exec(session.SessionID, ts, "EventMsg", string(jsonBytes))
		} else {
			// 1. Thoughts
			if len(msg.Thoughts) > 0 {
				var thoughtTexts []string
				for _, t := range msg.Thoughts {
					thoughtTexts = append(thoughtTexts, fmt.Sprintf("**%s**\n%s", t.Subject, t.Description))
				}
				fullThought := strings.Join(thoughtTexts, "\n\n")
				fullPayload := map[string]interface{}{
					"type": "event_msg",
					"payload": map[string]interface{}{
						"type": "agent_reasoning_raw_content",
						"text": fullThought,
					},
				}
				jsonBytes, _ := json.Marshal(fullPayload)
				_, err = stmt.Exec(session.SessionID, ts, "EventMsg", string(jsonBytes))
			}

			// 2. Tool Calls
			for _, tool := range msg.ToolCalls {
				argsJson, _ := json.Marshal(tool.Args)
				callPayload := map[string]interface{}{
					"type": "function_call",
					"name": tool.Name,
					"arguments": string(argsJson),
					"call_id": tool.ID,
				}
				fullCallPayload := map[string]interface{}{
					"type": "response_item",
					"payload": callPayload,
				}
				callBytes, _ := json.Marshal(fullCallPayload)
				_, err = stmt.Exec(session.SessionID, ts, "ResponseItem", string(callBytes))
				
				outputContent := ""
				if len(tool.Result) > 0 {
					resp := tool.Result[0].FunctionResponse.Response
					if out, ok := resp["output"].(string); ok {
						outputContent = out
					} else {
						outBytes, _ := json.Marshal(resp)
						outputContent = string(outBytes)
					}
				}
				outputPayload := map[string]interface{}{
					"type": "function_call_output",
					"call_id": tool.ID,
					"output": map[string]interface{}{
						"content": outputContent,
						"success": tool.Status == "success",
					},
				}
				fullOutputPayload := map[string]interface{}{
					"type": "response_item",
					"payload": outputPayload,
				}
				outputBytes, _ := json.Marshal(fullOutputPayload)
				_, err = stmt.Exec(session.SessionID, ts, "ResponseItem", string(outputBytes))
			}

			// 3. Text Content
			if msg.Content != "" {
				contentItems := []map[string]string{
					{"type": "output_text", "text": msg.Content},
				}
				payload := map[string]interface{}{
					"type": "message",
					"role": "assistant",
					"content": contentItems,
				}
				fullPayload := map[string]interface{}{
					"type": "response_item",
					"payload": payload,
				}
				jsonBytes, _ := json.Marshal(fullPayload)
				_, err = stmt.Exec(session.SessionID, ts, "ResponseItem", string(jsonBytes))
			}
		}
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	fmt.Printf("  Migrated %d messages for session %s\n", len(session.Messages), session.SessionID)
	return tx.Commit()
}
		