package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func listAllSessions(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT 
			s.id, 
			s.timestamp, 
			s.model, 
			s.cwd, 
			(SELECT COUNT(*) FROM events e WHERE e.session_id = s.id) as event_count 
		FROM sessions s 
		ORDER BY s.timestamp DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("%-36s | %-25s | %-20s | %-10s | %s\n", "ID", "Timestamp", "Model", "Events", "CWD")
	fmt.Println(strings.Repeat("-", 120))

	for rows.Next() {
		var id, ts, model, cwd string
		var count int
		if err := rows.Scan(&id, &ts, &model, &cwd, &count); err != nil {
			return err
		}
		// Truncate timestamp for display
		if len(ts) > 25 { ts = ts[:25] }
		fmt.Printf("%-36s | %-25s | %-20s | %-10d | %s\n", id, ts, model, count, cwd)
	}
	return nil
}

func generateHTML(db *sql.DB, sessionID string) error {
	rows, err := db.Query("SELECT payload FROM events WHERE session_id = ? ORDER BY id ASC", sessionID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var events []string
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return err
		}
		events = append(events, payload)
	}

	fmt.Println(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Chat History</title>
<style>
    body { font-family: sans-serif; background-color: #f0f2f5; margin: 0; padding: 0; }
    .container { margin: 10px; display: flex; flex-direction: column; gap: 10px; }
    .message { max-width: 98%; padding: 10px 15px; border-radius: 18px; line-height: 1.4; position: relative; word-wrap: break-word; white-space: pre-wrap; }
    .user { align-self: flex-end; background-color: #0084ff; color: white; border-bottom-right-radius: 4px; }
    .agent { align-self: flex-start; background-color: #e4e6eb; color: black; border-bottom-left-radius: 4px; }
    .thinking { align-self: flex-start; background-color: #f0f0f0; color: #555; font-style: italic; border: 1px dashed #ccc; font-size: 0.9em; }
    .tool { align-self: flex-start; background-color: #fdfdfd; color: #333; border: 1px solid #ddd; font-family: monospace; font-size: 0.85em; white-space: pre-wrap; line-height: 1.1; }
    .plan { align-self: flex-start; background-color: #fff; border: 1px solid #e0e0e0; border-radius: 8px; padding: 10px; width: 98%; white-space: pre-wrap; }
    .plan ul { list-style-type: none; padding-left: 0; margin: 0; }
    .plan li { margin-bottom: 5px; }
    .plan .completed { text-decoration: line-through; color: #888; }
    .plan .in_progress { font-weight: bold; color: #0084ff; }
    .system { align-self: center; background-color: transparent; color: #65676b; font-size: 0.85em; text-align: center; margin: 10px 0; }
    .timestamp { font-size: 0.7em; opacity: 0.7; margin-top: 5px; display: block; text-align: right; }
    pre { background: rgba(0,0,0,0.05); padding: 5px; border-radius: 5px; white-space: pre-wrap; word-wrap: break-word; line-height: 1.1; margin: 2px 0; }
    .markdown-content { white-space: normal; line-height: 1.2; }
    .markdown-content p { margin: 0.2em 0; }
    .markdown-content ul, .markdown-content ol { margin: 0.2em 0; padding-left: 20px; }
    .toggle-btn { cursor: pointer; color: #0084ff; text-decoration: underline; font-size: 0.9em; border: none; background: none; padding: 0; margin-top: 5px; }
    .hidden { display: none; }
</style>
<link href="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/themes/prism.min.css" rel="stylesheet" />
<link href="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/plugins/line-numbers/prism-line-numbers.min.css" rel="stylesheet" />
<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/prism.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/plugins/line-numbers/prism-line-numbers.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-bash.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-json.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-rust.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-python.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-c.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-cpp.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-go.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-diff.min.js"></script>
<script>
    function toggleOutput(id) {
        var preview = document.getElementById('preview-' + id);
        var full = document.getElementById('full-' + id);
        if (preview.style.display === 'none') {
            preview.style.display = 'block';
            full.style.display = 'none';
        } else {
            preview.style.display = 'none';
            full.style.display = 'block';
        }
    }
    document.addEventListener("DOMContentLoaded", function() {
        var markdownElements = document.querySelectorAll(".markdown-content");
        markdownElements.forEach(function(el) {
            el.innerHTML = marked.parse(el.textContent, { breaks: true });
            // Add line-numbers class to all pre tags generated by marked
            el.querySelectorAll('pre').forEach(function(pre) {
                pre.classList.add('line-numbers');
            });
        });
        Prism.highlightAll();
    });
</script>
</head>
<body>
<div class="container">`)

	outputCounter := 0

	for _, payload := range events {
		var item map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &item); err != nil {
			continue
		}

		// Handle both wrapped and unwrapped payloads (Codex/Gemini migration differences)
		itemType, _ := item["type"].(string)
		innerPayload, ok := item["payload"].(map[string]interface{})
		if !ok {
			innerPayload = item
		}
		innerType, _ := innerPayload["type"].(string)

		// Normalize types to handle snake_case and PascalCase
		isResponseItem := strings.EqualFold(itemType, "ResponseItem") || strings.EqualFold(itemType, "response_item")
		isEventMsg := strings.EqualFold(itemType, "EventMsg") || strings.EqualFold(itemType, "event_msg")
		isSessionMeta := strings.EqualFold(itemType, "SessionMeta") || strings.EqualFold(itemType, "session_meta")

		if isResponseItem {
			if innerType == "message" {
				role, _ := innerPayload["role"].(string)
				contentList, _ := innerPayload["content"].([]interface{})
				text := ""
				for _, c := range contentList {
					cMap, _ := c.(map[string]interface{})
					if t, ok := cMap["text"].(string); ok {
						text += t + "\n"
					} else if t, ok := cMap["input_text"].(string); ok {
						text += t + "\n"
					} else if t, ok := cMap["output_text"].(string); ok {
						text += t + "\n"
					}
				}
				cssClass := "agent"
				if role == "user" { cssClass = "user" }
				fmt.Printf(`<div class="message %s"><div class="markdown-content">%s</div></div>`, cssClass, htmlEscape(text))
			} else if innerType == "reasoning" {
				// Handle reasoning summary/content
				text := ""
				if summary, ok := innerPayload["summary"].([]interface{}); ok {
					for _, s := range summary {
						if sMap, ok := s.(map[string]interface{}); ok {
							if summaryType, _ := sMap["type"].(string); summaryType == "summary_text" {
								text += fmt.Sprintf("Summary: %s\n", sMap["text"])
							}
						}
					}
				}
				if content, ok := innerPayload["content"].([]interface{}); ok {
					for _, c := range content {
						if cMap, ok := c.(map[string]interface{}); ok {
							text += fmt.Sprintf("%s", cMap["text"])
						}
					}
				}
				fmt.Printf(`<div class="message thinking"><strong>Thinking Process:</strong><br><div class="markdown-content">%s</div></div>`, htmlEscape(text))
			} else if innerType == "function_call" {
				name, _ := innerPayload["name"].(string)
				argsStr, _ := innerPayload["arguments"].(string)
				
				// Try to smart parse args like the Rust version
				var argsJSON interface{}
				displayContent := ""
				if err := json.Unmarshal([]byte(argsStr), &argsJSON); err == nil {
					argsMap, isMap := argsJSON.(map[string]interface{})
					if isMap {
						if plan, ok := argsMap["plan"].([]interface{}); ok {
							planHTML := `<div class="plan"><strong>Plan:</strong><ul>`
							for _, step := range plan {
								if stepMap, ok := step.(map[string]interface{}); ok {
									stepText, _ := stepMap["step"].(string)
									status, _ := stepMap["status"].(string)
									icon := "☐"
									class := "pending"
									if status == "completed" { icon = "☑"; class = "completed" }
									if status == "in_progress" { icon = "▶"; class = "in_progress" }
									planHTML += fmt.Sprintf(`<li class="%s">%s %s</li>`, class, icon, htmlEscape(stepText))
								}
							}
							planHTML += "</ul></div>"
							displayContent = planHTML
						} else if cmd, ok := argsMap["cmd"]; ok {
							displayContent = fmt.Sprintf(`<strong>Executing:</strong><br><pre><code class="language-bash">&gt; %s</code></pre>`, htmlEscape(fmt.Sprint(cmd)))
						} else if cmd, ok := argsMap["command"]; ok {
							displayContent = fmt.Sprintf(`<strong>Executing:</strong><br><pre><code class="language-bash">&gt; %s</code></pre>`, htmlEscape(fmt.Sprint(cmd)))
						}
					}
					
					if displayContent == "" {
						prettyJSON, _ := json.MarshalIndent(argsJSON, "", "  ")
						displayContent = fmt.Sprintf(`<strong>Tool Call: %s</strong><br><pre><code class="language-json">%s</code></pre>`, name, htmlEscape(string(prettyJSON)))
					}
				} else {
					displayContent = fmt.Sprintf(`<strong>Tool Call: %s</strong><br><pre>%s</pre>`, name, htmlEscape(argsStr))
				}
				
				if strings.Contains(displayContent, `class="plan"`) {
					fmt.Println(displayContent)
				} else {
					fmt.Printf(`<div class="message tool">%s</div>`, displayContent)
				}

			} else if innerType == "function_call_output" {
				outputCounter++
				var content string
				if outputStr, ok := innerPayload["output"].(string); ok {
					content = outputStr
				} else if outputMap, ok := innerPayload["output"].(map[string]interface{}); ok {
					if c, ok := outputMap["content"].(string); ok {
						content = c
					} else {
						// Fallback: Dump the map
						bytes, _ := json.MarshalIndent(outputMap, "", "  ")
						content = string(bytes)
					}
				}
				
				formatted := formatCollapsibleOutput(content, outputCounter)
				fmt.Printf(`<div class="message tool"><strong>Tool Output:</strong><br>%s</div>`, formatted)
			}

		} else if isEventMsg {
			if innerType == "user_message" {
				msg, _ := innerPayload["message"].(string)
				fmt.Printf(`<div class="message user"><div class="markdown-content">%s</div></div>`, htmlEscape(msg))
			} else if innerType == "agent_reasoning" || innerType == "agent_reasoning_raw_content" {
				text, _ := innerPayload["text"].(string)
				fmt.Printf(`<div class="message thinking"><strong>Thinking Process:</strong><br><div class="markdown-content">%s</div></div>`, htmlEscape(text))
			} else if innerType == "exec_command_begin" {
				// Reconstruct command string
				var cmdStr string
				if cmdArr, ok := innerPayload["command"].([]interface{}); ok {
					var parts []string
					for _, p := range cmdArr { parts = append(parts, fmt.Sprint(p)) }
					cmdStr = strings.Join(parts, " ")
				}
				fmt.Printf(`<div class="message tool"><strong>Executing:</strong><br><pre><code class="language-bash">&gt; %s</code></pre></div>`, htmlEscape(cmdStr))
			} else if innerType == "exec_command_end" {
				outputCounter++
				exitCode, _ := innerPayload["exit_code"].(float64)
				stdout, _ := innerPayload["stdout"].(string)
				stderr, _ := innerPayload["stderr"].(string)
				output := stdout
				if int(exitCode) != 0 { output = stderr }
				formatted := formatCollapsibleOutput(output, outputCounter)
				fmt.Printf(`<div class="message tool"><strong>Command Result (Exit %d):</strong><br>%s</div>`, int(exitCode), formatted)
			} else if innerType == "mcp_tool_call_begin" {
				inv, _ := innerPayload["invocation"].(map[string]interface{})
				server, _ := inv["server"].(string)
				tool, _ := inv["tool"].(string)
				argsJSON, _ := json.MarshalIndent(inv["arguments"], "", "  ")
				fmt.Printf(`<div class="message tool"><strong>MCP Tool: %s.%s</strong><br><pre><code class="language-json">%s</code></pre></div>`, server, tool, htmlEscape(string(argsJSON)))
			} else if innerType == "mcp_tool_call_end" {
				outputCounter++
				// Assuming simple success/error string result for report
				// Real result structure is complex (CallToolResult)
				// We'll dump the result JSON or a summary
				resultJSON, _ := json.MarshalIndent(innerPayload["result"], "", "  ")
				formatted := formatCollapsibleOutput(string(resultJSON), outputCounter)
				fmt.Printf(`<div class="message tool"><strong>MCP Result:</strong> %s</div>`, formatted)
			} else if innerType == "plan_update" {
				planHTML := `<div class="plan"><strong>Plan Update:</strong><ul>`
				if plan, ok := innerPayload["plan"].([]interface{}); ok {
					for _, item := range plan {
						if itemMap, ok := item.(map[string]interface{}); ok {
							step, _ := itemMap["step"].(string)
							status, _ := itemMap["status"].(string)
							icon := "☐"
							class := "pending"
							if status == "completed" { icon = "☑"; class = "completed" }
							if status == "in_progress" { icon = "▶"; class = "in_progress" }
							planHTML += fmt.Sprintf(`<li class="%s">%s %s</li>`, class, icon, htmlEscape(step))
										}
						}
				}
				planHTML += "</ul></div>"
				fmt.Println(planHTML)
			}

		} else if isSessionMeta {
             timestamp, _ := innerPayload["timestamp"].(string)
             provider, _ := innerPayload["model_provider"].(string)
             fmt.Printf(`<div class="system">Session Started: %s<br>Model: %s</div>`, timestamp, provider)
        }
	}

	fmt.Println("</div></body></html>")
	return nil
}

func htmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	).Replace(s)
}

func formatCollapsibleOutput(text string, id int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 20 {
		return fmt.Sprintf("<pre>%s</pre>", htmlEscape(text))
	}

	head := strings.Join(lines[:10], "\n")
	tail := strings.Join(lines[len(lines)-10:], "\n")
	skipped := len(lines) - 20

	previewHTML := fmt.Sprintf(
		"<pre>%s</pre><br><em>... %d lines skipped ...</em><br><pre>%s</pre>",
		htmlEscape(head),
		skipped,
		htmlEscape(tail),
	)
	fullHTML := fmt.Sprintf("<pre>%s</pre>", htmlEscape(text))

	return fmt.Sprintf(`<div id="preview-%d">%s<button class="toggle-btn" onclick="toggleOutput(%d)">Show full output</button></div><div id="full-%d" class="hidden">%s<button class="toggle-btn" onclick="toggleOutput(%d)">Show less</button></div>`, id, previewHTML, id, id, fullHTML, id)
}
