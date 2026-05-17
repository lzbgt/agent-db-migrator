# agent-db-migrator

[![Paid migration review](https://img.shields.io/badge/paid%20review-AI%20Session%20DB%20Migration-175c4c)](https://x2.brucelu.top/dbmigrate/?source=github-agent-db-migrator-top)
[![Ask first](https://img.shields.io/badge/ask%20first-pre--sales-44546a)](https://x2.brucelu.top/products/contact/?offer=dbmigrate&source=github-agent-db-migrator-top)
[![Sample review](https://img.shields.io/badge/sample-review-6b7280)](https://x2.brucelu.top/dbmigrate/sample/?source=github-agent-db-migrator-top)

Go utility for migrating Codex and Gemini session histories into a local SQLite database and generating HTML chat-history reports.

## Paid review

If you found this repo while trying to preserve or inspect agent session history, I offer a focused paid review:

- Review page: https://x2.brucelu.top/dbmigrate/?source=github-agent-db-migrator-top
- Sample deliverable: https://x2.brucelu.top/dbmigrate/sample/?source=github-agent-db-migrator-top
- Ask a pre-sales question: https://x2.brucelu.top/products/contact/?offer=dbmigrate&source=github-agent-db-migrator-top
- Checkout: https://x2.brucelu.top/dbmigrate/checkout/?source=github-agent-db-migrator-top

The review covers JSONL parsing, Codex/Gemini source discovery, SQLite schema compatibility, transaction boundaries, merge/dedupe policy, WAL/concurrency settings, HTML report generation, backups, and rollback flow.

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
