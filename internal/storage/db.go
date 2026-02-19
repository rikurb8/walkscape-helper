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
		`CREATE TABLE IF NOT EXISTS wiki_sync_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS wiki_pages (
			page_id INTEGER PRIMARY KEY,
			namespace INTEGER NOT NULL,
			title TEXT NOT NULL,
			canonical_url TEXT,
			is_redirect INTEGER NOT NULL DEFAULT 0,
			latest_revision_id INTEGER,
			latest_revision_sha1 TEXT,
			latest_revision_ts TEXT,
			lang_code TEXT,
			source_snapshot_id TEXT,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_pages_namespace_title ON wiki_pages(namespace, title)`,
		`CREATE TABLE IF NOT EXISTS wiki_revisions (
			revision_id INTEGER PRIMARY KEY,
			page_id INTEGER NOT NULL,
			parent_revision_id INTEGER,
			timestamp TEXT NOT NULL,
			sha1 TEXT,
			size INTEGER,
			content_model TEXT,
			wikitext TEXT,
			comment TEXT,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(page_id) REFERENCES wiki_pages(page_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_revisions_page_id ON wiki_revisions(page_id)`,
		`CREATE TABLE IF NOT EXISTS wiki_page_edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			page_id INTEGER NOT NULL,
			edge_type TEXT NOT NULL,
			target TEXT NOT NULL,
			source_snapshot_id TEXT,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(page_id) REFERENCES wiki_pages(page_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_page_edges_page_type ON wiki_page_edges(page_id, edge_type)`,
		`CREATE TABLE IF NOT EXISTS wiki_files (
			file_page_id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			mime TEXT,
			size INTEGER,
			url TEXT,
			sha1 TEXT,
			timestamp TEXT,
			ext_metadata_json TEXT,
			source_snapshot_id TEXT,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS wiki_tombstones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			page_id INTEGER,
			title TEXT,
			event_type TEXT NOT NULL,
			event_ts TEXT NOT NULL,
			reason TEXT,
			source_snapshot_id TEXT,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_tombstones_page_id ON wiki_tombstones(page_id)`,
		`CREATE TABLE IF NOT EXISTS wiki_fetch_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT,
			request_url TEXT NOT NULL,
			http_status INTEGER NOT NULL,
			response_body TEXT NOT NULL,
			fetched_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_fetch_log_snapshot ON wiki_fetch_log(snapshot_id)`,
		`INSERT OR IGNORE INTO app_meta (key, value) VALUES ('schema_version', '1')`,
		`INSERT OR IGNORE INTO app_meta (key, value) VALUES ('wiki_last_full_snapshot_id', '')`,
		`INSERT OR IGNORE INTO app_meta (key, value) VALUES ('wiki_recentchanges_cursor', '')`,
		`INSERT OR IGNORE INTO app_meta (key, value) VALUES ('wiki_last_sync_ts', '')`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
