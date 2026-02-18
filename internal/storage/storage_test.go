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
