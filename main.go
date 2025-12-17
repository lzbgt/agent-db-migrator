package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var (
	migrateCodex  = flag.Bool("migrate-codex", false, "Migrate Codex sessions from ~/.codex")
	migrateGemini = flag.Bool("migrate-gemini", false, "Migrate Gemini sessions from ~/.gemini")
	listSessions  = flag.Bool("list", false, "List all sessions in the database")
	htmlReport    = flag.String("html-report", "", "Generate HTML report for a session ID")
	mergeDbs      = flag.String("merge-dbs", "", "Comma-separated list of source DB files to merge into the destination DB")
	dbPath        = flag.String("db", "", "Path to the SQLite database (default: ~/.codex/history.db)")
)

func main() {
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home dir: %v", err)
	}

	defaultDbPath := filepath.Join(home, ".codex", "history.db")
	if *dbPath == "" {
		*dbPath = defaultDbPath
	}

	db, err := initDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize DB: %v", err)
	}
	defer db.Close()

	if *mergeDbs != "" {
		if err := mergeDatabases(db, *mergeDbs); err != nil {
			log.Printf("Error merging databases: %v", err)
		}
	}

	if *migrateCodex {
		if err := migrateCodexSessions(db, home); err != nil {
			log.Printf("Error migrating Codex sessions: %v", err)
		}
	}

	if *migrateGemini {
		if err := migrateGeminiSessions(db, home); err != nil {
			log.Printf("Error migrating Gemini sessions: %v", err)
		}
	}

	if *listSessions {
		if err := listAllSessions(db); err != nil {
			log.Printf("Error listing sessions: %v", err)
		}
	}

	if *htmlReport != "" {
		if err := generateHTML(db, *htmlReport); err != nil {
			log.Printf("Error generating HTML: %v", err)
		}
	}
}

func initDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Schema compatible with the Rust implementation
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		timestamp TEXT,
		model TEXT,
		cwd TEXT,
		instructions TEXT,
		json_meta TEXT
	);
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT,
		timestamp TEXT,
		event_type TEXT,
		payload TEXT,
		FOREIGN KEY(session_id) REFERENCES sessions(id)
	);
	`
	_, err = db.Exec(schema)
	if err != nil {
		return nil, err
	}
	// Enable WAL mode for concurrency compatibility with Rust CLI
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("PRAGMA synchronous=NORMAL;")
	if err != nil {
		return nil, err
	}
	
	return db, nil
}