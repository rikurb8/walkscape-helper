package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // register sqlite driver
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// CLI usage does not benefit from pooling; keep a single connection so
	// connection-scoped SQLite pragmas are consistently applied.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := setPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func buildDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
}

func setPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}
	return nil
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
