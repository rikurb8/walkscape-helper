package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/google/uuid"
)

type Character struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	Source         string `json:"source"`
	RawJSON        string `json:"raw_json"`
	NormalizedJSON string `json:"normalized_json"`
	ContentHash    string `json:"content_hash"`
	ImportedAt     string `json:"imported_at"`
}

func NormalizeCharacterJSON(raw []byte) (normalized []byte, name string, err error) {
	var payload map[string]any
	err = json.Unmarshal(raw, &payload)
	if err != nil {
		return nil, "", err
	}
	normalized, err = json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	name = ""
	if v, ok := payload["name"].(string); ok {
		name = v
	}
	return normalized, name, nil
}

func InsertCharacter(ctx context.Context, db *sql.DB, source string, raw []byte) (Character, error) {
	normalized, name, err := NormalizeCharacterJSON(raw)
	if err != nil {
		return Character{}, err
	}
	h := sha256.Sum256(raw)
	now := time.Now().UTC().Format(time.RFC3339)
	ch := Character{
		ID:             uuid.NewString(),
		Name:           name,
		Source:         source,
		RawJSON:        string(raw),
		NormalizedJSON: string(normalized),
		ContentHash:    hex.EncodeToString(h[:]),
		ImportedAt:     now,
	}

	stmt := charactersTable.
		INSERT(
			charactersID,
			charactersName,
			charactersSource,
			charactersRawJSON,
			charactersNormalizedJSON,
			charactersContentHash,
			charactersImportedAt,
		).
		VALUES(ch.ID, ch.Name, ch.Source, ch.RawJSON, ch.NormalizedJSON, ch.ContentHash, ch.ImportedAt)

	query, args := stmt.Sql()
	_, err = db.ExecContext(ctx, query, args...)
	if err != nil {
		return Character{}, err
	}
	return ch, nil
}

func ListCharacters(ctx context.Context, db *sql.DB) ([]Character, error) {
	stmt := sqlite.
		SELECT(
			charactersID,
			charactersName,
			charactersSource,
			charactersRawJSON,
			charactersNormalizedJSON,
			charactersContentHash,
			charactersImportedAt,
		).
		FROM(charactersTable).
		ORDER_BY(charactersImportedAt.DESC())

	query, args := stmt.Sql()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chars := make([]Character, 0)
	for rows.Next() {
		var ch Character
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Source, &ch.RawJSON, &ch.NormalizedJSON, &ch.ContentHash, &ch.ImportedAt); err != nil {
			return nil, err
		}
		chars = append(chars, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chars, nil
}

func GetCharacterByID(ctx context.Context, db *sql.DB, id string) (Character, bool, error) {
	var ch Character
	stmt := sqlite.
		SELECT(
			charactersID,
			charactersName,
			charactersSource,
			charactersRawJSON,
			charactersNormalizedJSON,
			charactersContentHash,
			charactersImportedAt,
		).
		FROM(charactersTable).
		WHERE(charactersID.EQ(sqlite.String(id))).
		LIMIT(1)

	query, args := stmt.Sql()
	err := db.QueryRowContext(ctx, query, args...).Scan(
		&ch.ID,
		&ch.Name,
		&ch.Source,
		&ch.RawJSON,
		&ch.NormalizedJSON,
		&ch.ContentHash,
		&ch.ImportedAt,
	)
	if err == sql.ErrNoRows {
		return Character{}, false, nil
	}
	if err != nil {
		return Character{}, false, err
	}
	return ch, true, nil
}

func GetLatestCharacter(ctx context.Context, db *sql.DB) (Character, bool, error) {
	var ch Character
	stmt := sqlite.
		SELECT(
			charactersID,
			charactersName,
			charactersSource,
			charactersRawJSON,
			charactersNormalizedJSON,
			charactersContentHash,
			charactersImportedAt,
		).
		FROM(charactersTable).
		ORDER_BY(charactersImportedAt.DESC()).
		LIMIT(1)

	query, args := stmt.Sql()
	err := db.QueryRowContext(ctx, query, args...).Scan(
		&ch.ID,
		&ch.Name,
		&ch.Source,
		&ch.RawJSON,
		&ch.NormalizedJSON,
		&ch.ContentHash,
		&ch.ImportedAt,
	)
	if err == sql.ErrNoRows {
		return Character{}, false, nil
	}
	if err != nil {
		return Character{}, false, err
	}
	return ch, true, nil
}
