package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS guide_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			llm_provider TEXT,
			llm_model TEXT,
			llm_api_key_env TEXT,
			embedding_provider TEXT,
			embedding_model TEXT,
			vectordb_provider TEXT,
			vectordb_url TEXT,
			vectordb_collection TEXT,
			vectordb_api_key_env TEXT,
			persona_name TEXT,
			persona_prompt TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS characters (
			id TEXT PRIMARY KEY,
			name TEXT,
			source TEXT NOT NULL,
			raw_json TEXT NOT NULL,
			normalized_json TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			imported_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS app_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`INSERT OR IGNORE INTO app_meta (key, value) VALUES ('schema_version', '1')`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
