package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
)

type GuideConfig struct {
	LLMProvider        string `json:"llm_provider"`
	LLMModel           string `json:"llm_model"`
	LLMAPIKeyEnv       string `json:"llm_api_key_env"`
	EmbeddingProvider  string `json:"embedding_provider"`
	EmbeddingModel     string `json:"embedding_model"`
	VectorDBProvider   string `json:"vectordb_provider"`
	VectorDBURL        string `json:"vectordb_url"`
	VectorDBCollection string `json:"vectordb_collection"`
	VectorDBAPIKeyEnv  string `json:"vectordb_api_key_env"`
	PersonaName        string `json:"persona_name"`
	PersonaPrompt      string `json:"persona_prompt"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

func UpsertGuideConfig(ctx context.Context, db *sql.DB, cfg *GuideConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if cfg.CreatedAt == "" {
		cfg.CreatedAt = now
	}
	cfg.UpdatedAt = now

	stmt := guideConfigTable.
		INSERT(
			guideConfigID,
			guideConfigLLMProvider,
			guideConfigLLMModel,
			guideConfigLLMAPIKeyEnv,
			guideConfigEmbeddingProvider,
			guideConfigEmbeddingModel,
			guideConfigVectorDBProvider,
			guideConfigVectorDBURL,
			guideConfigVectorDBCollection,
			guideConfigVectorDBAPIKeyEnv,
			guideConfigPersonaName,
			guideConfigPersonaPrompt,
			guideConfigCreatedAt,
			guideConfigUpdatedAt,
		).
		VALUES(
			1,
			cfg.LLMProvider,
			cfg.LLMModel,
			cfg.LLMAPIKeyEnv,
			cfg.EmbeddingProvider,
			cfg.EmbeddingModel,
			cfg.VectorDBProvider,
			cfg.VectorDBURL,
			cfg.VectorDBCollection,
			cfg.VectorDBAPIKeyEnv,
			cfg.PersonaName,
			cfg.PersonaPrompt,
			cfg.CreatedAt,
			cfg.UpdatedAt,
		).
		ON_CONFLICT(guideConfigID).
		DO_UPDATE(
			sqlite.SET(
				guideConfigLLMProvider.SET(sqlite.String(cfg.LLMProvider)),
				guideConfigLLMModel.SET(sqlite.String(cfg.LLMModel)),
				guideConfigLLMAPIKeyEnv.SET(sqlite.String(cfg.LLMAPIKeyEnv)),
				guideConfigEmbeddingProvider.SET(sqlite.String(cfg.EmbeddingProvider)),
				guideConfigEmbeddingModel.SET(sqlite.String(cfg.EmbeddingModel)),
				guideConfigVectorDBProvider.SET(sqlite.String(cfg.VectorDBProvider)),
				guideConfigVectorDBURL.SET(sqlite.String(cfg.VectorDBURL)),
				guideConfigVectorDBCollection.SET(sqlite.String(cfg.VectorDBCollection)),
				guideConfigVectorDBAPIKeyEnv.SET(sqlite.String(cfg.VectorDBAPIKeyEnv)),
				guideConfigPersonaName.SET(sqlite.String(cfg.PersonaName)),
				guideConfigPersonaPrompt.SET(sqlite.String(cfg.PersonaPrompt)),
				guideConfigUpdatedAt.SET(sqlite.String(cfg.UpdatedAt)),
			),
		)

	query, args := stmt.Sql()
	_, err := db.ExecContext(ctx, query, args...)
	return err
}

func GetGuideConfig(ctx context.Context, db *sql.DB) (GuideConfig, bool, error) {
	var cfg GuideConfig
	stmt := sqlite.
		SELECT(
			guideConfigLLMProvider,
			guideConfigLLMModel,
			guideConfigLLMAPIKeyEnv,
			guideConfigEmbeddingProvider,
			guideConfigEmbeddingModel,
			guideConfigVectorDBProvider,
			guideConfigVectorDBURL,
			guideConfigVectorDBCollection,
			guideConfigVectorDBAPIKeyEnv,
			guideConfigPersonaName,
			guideConfigPersonaPrompt,
			guideConfigCreatedAt,
			guideConfigUpdatedAt,
		).
		FROM(guideConfigTable).
		WHERE(guideConfigID.EQ(sqlite.Int(1))).
		LIMIT(1)

	query, args := stmt.Sql()
	err := db.QueryRowContext(ctx, query, args...).Scan(
		&cfg.LLMProvider,
		&cfg.LLMModel,
		&cfg.LLMAPIKeyEnv,
		&cfg.EmbeddingProvider,
		&cfg.EmbeddingModel,
		&cfg.VectorDBProvider,
		&cfg.VectorDBURL,
		&cfg.VectorDBCollection,
		&cfg.VectorDBAPIKeyEnv,
		&cfg.PersonaName,
		&cfg.PersonaPrompt,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return GuideConfig{}, false, nil
	}
	if err != nil {
		return GuideConfig{}, false, err
	}
	return cfg, true, nil
}
