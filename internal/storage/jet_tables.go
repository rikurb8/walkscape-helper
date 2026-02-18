package storage

import "github.com/go-jet/jet/v2/sqlite"

var (
	guideConfigID                 = sqlite.IntegerColumn("id")
	guideConfigLLMProvider        = sqlite.StringColumn("llm_provider")
	guideConfigLLMModel           = sqlite.StringColumn("llm_model")
	guideConfigLLMAPIKeyEnv       = sqlite.StringColumn("llm_api_key_env")
	guideConfigEmbeddingProvider  = sqlite.StringColumn("embedding_provider")
	guideConfigEmbeddingModel     = sqlite.StringColumn("embedding_model")
	guideConfigVectorDBProvider   = sqlite.StringColumn("vectordb_provider")
	guideConfigVectorDBURL        = sqlite.StringColumn("vectordb_url")
	guideConfigVectorDBCollection = sqlite.StringColumn("vectordb_collection")
	guideConfigVectorDBAPIKeyEnv  = sqlite.StringColumn("vectordb_api_key_env")
	guideConfigPersonaName        = sqlite.StringColumn("persona_name")
	guideConfigPersonaPrompt      = sqlite.StringColumn("persona_prompt")
	guideConfigCreatedAt          = sqlite.StringColumn("created_at")
	guideConfigUpdatedAt          = sqlite.StringColumn("updated_at")

	guideConfigTable = sqlite.NewTable(
		"",
		"guide_config",
		"",
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
	)

	charactersID             = sqlite.StringColumn("id")
	charactersName           = sqlite.StringColumn("name")
	charactersSource         = sqlite.StringColumn("source")
	charactersRawJSON        = sqlite.StringColumn("raw_json")
	charactersNormalizedJSON = sqlite.StringColumn("normalized_json")
	charactersContentHash    = sqlite.StringColumn("content_hash")
	charactersImportedAt     = sqlite.StringColumn("imported_at")

	charactersTable = sqlite.NewTable(
		"",
		"characters",
		"",
		charactersID,
		charactersName,
		charactersSource,
		charactersRawJSON,
		charactersNormalizedJSON,
		charactersContentHash,
		charactersImportedAt,
	)
)
