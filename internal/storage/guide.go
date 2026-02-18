package storage

import (
	"context"
	"database/sql"
	"time"
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

	_, err := db.ExecContext(ctx, `
		INSERT INTO guide_config (
			id, llm_provider, llm_model, llm_api_key_env,
			embedding_provider, embedding_model,
			vectordb_provider, vectordb_url, vectordb_collection, vectordb_api_key_env,
			persona_name, persona_prompt, created_at, updated_at
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			llm_provider=excluded.llm_provider,
			llm_model=excluded.llm_model,
			llm_api_key_env=excluded.llm_api_key_env,
			embedding_provider=excluded.embedding_provider,
			embedding_model=excluded.embedding_model,
			vectordb_provider=excluded.vectordb_provider,
			vectordb_url=excluded.vectordb_url,
			vectordb_collection=excluded.vectordb_collection,
			vectordb_api_key_env=excluded.vectordb_api_key_env,
			persona_name=excluded.persona_name,
			persona_prompt=excluded.persona_prompt,
			updated_at=excluded.updated_at
	`,
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
	)
	return err
}

func GetGuideConfig(ctx context.Context, db *sql.DB) (GuideConfig, bool, error) {
	var cfg GuideConfig
	err := db.QueryRowContext(ctx, `
		SELECT llm_provider, llm_model, llm_api_key_env,
			embedding_provider, embedding_model,
			vectordb_provider, vectordb_url, vectordb_collection, vectordb_api_key_env,
			persona_name, persona_prompt, created_at, updated_at
		FROM guide_config WHERE id = 1
	`).Scan(
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
