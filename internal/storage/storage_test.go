package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGetGuideConfigNotFound(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, found, err := GetGuideConfig(context.Background(), db)
	if err != nil {
		t.Fatalf("GetGuideConfig() error = %v", err)
	}
	if found {
		t.Fatal("expected no guide config")
	}
}

func TestInsertCharacterInvalidJSON(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = InsertCharacter(context.Background(), db, "raw", []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestOpenHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	_, err := Open(ctx, dbPath)
	if err == nil {
		t.Fatal("expected Open() to fail for canceled context")
	}
}

func TestOpenUsesSingleConnection(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want %d", stats.MaxOpenConnections, 1)
	}
}

func TestOpenSetsWALMode(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var foreignKeys int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	if err != nil {
		t.Fatalf("failed to query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want %d", foreignKeys, 1)
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "test.db",
			expected: "file:test.db?_pragma=busy_timeout(5000)",
		},
		{
			name:     "path with directory",
			path:     "/tmp/test.db",
			expected: "file:/tmp/test.db?_pragma=busy_timeout(5000)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDSN(tt.path)
			if result != tt.expected {
				t.Errorf("buildDSN() = %q, want %q", result, tt.expected)
			}
		})
	}
}
