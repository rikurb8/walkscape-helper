package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func TestGetGuideConfigNotFound(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	_, found, err := GetGuideConfig(context.Background(), db)
	if err != nil {
		t.Fatalf("GetGuideConfig() error = %v", err)
	}
	if found {
		t.Fatal("expected no guide config")
	}
}

func TestUpsertGuideConfigRoundTrip(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	cfg := &GuideConfig{
		LLMProvider:        "openai",
		LLMModel:           "gpt-5",
		LLMAPIKeyEnv:       "OPENAI_API_KEY",
		EmbeddingProvider:  "openai",
		EmbeddingModel:     "text-embedding-3-large",
		VectorDBProvider:   "qdrant",
		VectorDBURL:        "http://localhost:6333",
		VectorDBCollection: "walkscape",
		VectorDBAPIKeyEnv:  "QDRANT_API_KEY",
		PersonaName:        "Guide",
		PersonaPrompt:      "Help the player.",
	}

	if err := UpsertGuideConfig(context.Background(), db, cfg); err != nil {
		t.Fatalf("UpsertGuideConfig() error = %v", err)
	}

	stored, found, err := GetGuideConfig(context.Background(), db)
	if err != nil {
		t.Fatalf("GetGuideConfig() error = %v", err)
	}
	if !found {
		t.Fatal("expected guide config to exist")
	}
	if stored.LLMProvider != cfg.LLMProvider || stored.PersonaPrompt != cfg.PersonaPrompt {
		t.Fatalf("stored cfg = %+v, want fields from %+v", stored, cfg)
	}
	if stored.CreatedAt == "" || stored.UpdatedAt == "" {
		t.Fatalf("expected timestamps to be set, got created_at=%q updated_at=%q", stored.CreatedAt, stored.UpdatedAt)
	}
}

func TestUpsertGuideConfigPreservesCreatedAtOnConflict(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	first := &GuideConfig{LLMProvider: "openai", LLMModel: "gpt-5", PersonaName: "alpha"}
	if err := UpsertGuideConfig(context.Background(), db, first); err != nil {
		t.Fatalf("first UpsertGuideConfig() error = %v", err)
	}

	storedFirst, found, err := GetGuideConfig(context.Background(), db)
	if err != nil {
		t.Fatalf("GetGuideConfig() first read error = %v", err)
	}
	if !found {
		t.Fatal("expected first config to be found")
	}

	time.Sleep(1100 * time.Millisecond)

	second := &GuideConfig{LLMProvider: "anthropic", LLMModel: "claude-sonnet", PersonaName: "beta"}
	if err := UpsertGuideConfig(context.Background(), db, second); err != nil {
		t.Fatalf("second UpsertGuideConfig() error = %v", err)
	}

	storedSecond, found, err := GetGuideConfig(context.Background(), db)
	if err != nil {
		t.Fatalf("GetGuideConfig() second read error = %v", err)
	}
	if !found {
		t.Fatal("expected second config to be found")
	}

	if storedSecond.CreatedAt != storedFirst.CreatedAt {
		t.Fatalf("created_at changed on conflict update: first=%q second=%q", storedFirst.CreatedAt, storedSecond.CreatedAt)
	}
	if storedSecond.UpdatedAt == storedFirst.UpdatedAt {
		t.Fatalf("updated_at did not change: first=%q second=%q", storedFirst.UpdatedAt, storedSecond.UpdatedAt)
	}
	if storedSecond.LLMProvider != "anthropic" || storedSecond.PersonaName != "beta" {
		t.Fatalf("second update fields not applied: %+v", storedSecond)
	}
}

func TestInsertCharacterInvalidJSON(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	_, err := InsertCharacter(context.Background(), db, "raw", []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestInsertCharacterAndLookupByID(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	raw := []byte(`{"name":"Elder Rowan","level":18}`)
	inserted, err := InsertCharacter(context.Background(), db, "import", raw)
	if err != nil {
		t.Fatalf("InsertCharacter() error = %v", err)
	}

	foundByID, found, err := GetCharacterByID(context.Background(), db, inserted.ID)
	if err != nil {
		t.Fatalf("GetCharacterByID() error = %v", err)
	}
	if !found {
		t.Fatal("expected inserted character to be found")
	}
	if foundByID.ID != inserted.ID || foundByID.Name != "Elder Rowan" || foundByID.Source != "import" {
		t.Fatalf("found character mismatch: got %+v inserted %+v", foundByID, inserted)
	}

	expectedHashBytes := sha256.Sum256(raw)
	expectedHash := hex.EncodeToString(expectedHashBytes[:])
	if foundByID.ContentHash != expectedHash {
		t.Fatalf("content hash = %q, want %q", foundByID.ContentHash, expectedHash)
	}
}

func TestGetCharacterByIDNotFound(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	_, found, err := GetCharacterByID(context.Background(), db, "missing-id")
	if err != nil {
		t.Fatalf("GetCharacterByID() error = %v", err)
	}
	if found {
		t.Fatal("expected character lookup to be not found")
	}
}

func TestListCharactersAndGetLatestAreOrderedByImportedAtDesc(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	first, err := InsertCharacter(context.Background(), db, "import", []byte(`{"name":"A"}`))
	if err != nil {
		t.Fatalf("InsertCharacter() first error = %v", err)
	}

	if _, err := db.ExecContext(
		context.Background(),
		"UPDATE characters SET imported_at = ? WHERE id = ?",
		"2020-01-01T00:00:00Z",
		first.ID,
	); err != nil {
		t.Fatalf("forcing first imported_at failed: %v", err)
	}

	second, err := InsertCharacter(context.Background(), db, "import", []byte(`{"name":"B"}`))
	if err != nil {
		t.Fatalf("InsertCharacter() second error = %v", err)
	}

	if _, err := db.ExecContext(
		context.Background(),
		"UPDATE characters SET imported_at = ? WHERE id = ?",
		"2030-01-01T00:00:00Z",
		second.ID,
	); err != nil {
		t.Fatalf("forcing second imported_at failed: %v", err)
	}

	list, err := ListCharacters(context.Background(), db)
	if err != nil {
		t.Fatalf("ListCharacters() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(ListCharacters()) = %d, want 2", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("unexpected list order: got [%s %s], want [%s %s]", list[0].ID, list[1].ID, second.ID, first.ID)
	}

	latest, found, err := GetLatestCharacter(context.Background(), db)
	if err != nil {
		t.Fatalf("GetLatestCharacter() error = %v", err)
	}
	if !found {
		t.Fatal("expected latest character to be found")
	}
	if latest.ID != second.ID {
		t.Fatalf("latest.ID = %q, want %q", latest.ID, second.ID)
	}
}

func TestGetLatestCharacterNotFound(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	_, found, err := GetLatestCharacter(context.Background(), db)
	if err != nil {
		t.Fatalf("GetLatestCharacter() error = %v", err)
	}
	if found {
		t.Fatal("expected no latest character")
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

	db := openTestDB(t)

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want %d", stats.MaxOpenConnections, 1)
	}
}

func TestOpenSetsWALMode(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	var journalMode string
	err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	var foreignKeys int
	err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
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
