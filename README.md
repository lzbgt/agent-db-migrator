# agent-db-migrator

Go utility for migrating Codex and Gemini session histories into a local SQLite database and generating HTML chat-history reports.

## What the tool does

- Migrates Codex session JSONL files from `~/.codex/sessions`.
- Migrates Gemini session JSON files from `~/.gemini/tmp`.
- Writes sessions and events into SQLite.
- Supports merging source databases into a destination database.
- Lists sessions and generates an HTML report for one session ID.

## Example commands

```bash
go run . --migrate-codex --db ~/.codex/history.db
go run . --migrate-gemini --db ~/.codex/history.db
go run . --list --db ~/.codex/history.db
go run . --html-report SESSION_ID --db ~/.codex/history.db > report.html
```

## Safety notes

Run against a copied database first. Capture source counts, skipped files, parse errors, and inserted row counts before trusting a migration as complete.
