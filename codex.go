package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type CodexRolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type CodexSessionMeta struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	Cwd           string `json:"cwd"`
	ModelProvider string `json:"model_provider"`
}

func migrateCodexSessions(db *sql.DB, home string) error {
	sessionsDir := filepath.Join(home, ".codex", "sessions")
	
	return filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		fmt.Printf("Processing %s\n", path)
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		var sessionID string
		var items []CodexRolloutLine
		var meta *CodexSessionMeta

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			var rolloutLine CodexRolloutLine
			if err := json.Unmarshal([]byte(line), &rolloutLine); err != nil {
				// Try parsing as legacy format or just skip
				continue
			}
			
			// We need to re-serialize the whole item structure as payload for the DB
			// But the DB expects `RolloutItem` JSON structure as `payload`
			// My Go struct `CodexRolloutLine` has `Payload` as raw message, which maps to `payload` field in JSON?
			// No, the rust code serializes `RolloutLine` which flattens `RolloutItem`.
			// `{"timestamp": "...", "type": "session_meta", "payload": {...}}`
		
			// Re-construct the item for DB
			// The Rust DB expects the `item` part of `RolloutLine`.
			// `RolloutLine` in Rust is: `timestamp`, flattened `item`.
			// So `item` has `type` and `payload`.
		
			itemMap := map[string]interface{}{}
			if err := json.Unmarshal([]byte(line), &itemMap); err != nil {
				continue
			}
			delete(itemMap, "timestamp") // Remove timestamp from the item we store in 'payload' column? 
			// Wait, the Rust `DbWriter::write_item` stores `serde_json::to_string(item)`.
			// `item` is `RolloutItem`.
			// `RolloutLine` has `timestamp` and flattened `RolloutItem`.
			// So `line` IS the `RolloutItem` plus timestamp.
			// If I remove timestamp, I get `RolloutItem`.
		
			itemBytes, _ := json.Marshal(itemMap)
			
			// Extract Session Meta to get ID
			if rolloutLine.Type == "session_meta" {
				var m CodexSessionMeta
				if err := json.Unmarshal(rolloutLine.Payload, &m); err == nil {
					sessionID = m.ID
					meta = &m
				}
			}
			
			items = append(items, CodexRolloutLine{
				Timestamp: rolloutLine.Timestamp,
				Type: rolloutLine.Type, // just for internal logic
				Payload: itemBytes,      // This is the full item JSON (minus timestamp hopefully)
			})
		}

		if sessionID == "" {
			return nil // Skip if no session ID found
		}

		// 1. Write Meta
		if meta != nil {
			metaJson, _ := json.Marshal(meta)
			_, err = db.Exec(
				"INSERT OR REPLACE INTO sessions (id, timestamp, model, cwd, instructions, json_meta) VALUES (?, ?, ?, ?, ?, ?)",
				meta.ID, meta.Timestamp, meta.ModelProvider, meta.Cwd, "", string(metaJson),
			)
			if err != nil {
				return fmt.Errorf("failed to write meta: %w", err)
			}
		}

		// 2. Clear existing events
		_, err = db.Exec("DELETE FROM events WHERE session_id = ?", sessionID)
		if err != nil {
			return fmt.Errorf("failed to clear events: %w", err)
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

		for _, item := range items {
			// Convert item type (snake_case) to Rust enum variant name (PascalCase) if possible, 
			// or just use what we have. The Rust `DbWriter` converts enum to string.
			// "session_meta" -> "SessionMeta"
			// "response_item" -> "ResponseItem"
			// "event_msg" -> "EventMsg"
			
			eventType := toPascalCase(item.Type)
			if item.Type == "event_msg" { eventType = "EventMsg" } // specific override if needed
			
			// Timestamp: use the one from the file or now? Rust recorder uses `now_utc` when writing to DB.
			// But for migration, we should probably preserve the order.
			// The `events` table has a `timestamp` column.
			
			_, err = stmt.Exec(sessionID, item.Timestamp, eventType, string(item.Payload))
			if err != nil {
				tx.Rollback()
				return err
			}
		}
		
		fmt.Printf("  Migrated %d events for session %s\n", len(items), sessionID)
		return tx.Commit()
	})
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
