package cli

import (
	"errors"

	"walkscape-helper/internal/output"
	"walkscape-helper/internal/storage"

	"github.com/spf13/cobra"
)

func newGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Manage guide configuration",
	}
	cmd.AddCommand(newGuideSetupCmd())
	cmd.AddCommand(newGuideShowCmd())
	cmd.AddCommand(newGuideValidateCmd())
	return cmd
}

func newGuideSetupCmd() *cobra.Command {
	var cfg storage.GuideConfig

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set guide configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()
			if err := validateGuideConfig(cfg); err != nil {
				return writeErr(cmd, ctx.JSON, "guide setup", err)
			}

			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "guide setup", appErr)
			}
			defer db.Close()

			if err := storage.UpsertGuideConfig(execCtx, db, cfg); err != nil {
				appErr := output.NewError("storage_error", "failed to save guide configuration", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "guide setup", appErr)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{"saved": true, "config": cfg}, commandMeta("guide setup"))
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Guide configuration saved")
		},
	}

	cmd.Flags().StringVar(&cfg.LLMProvider, "llm-provider", "", "llm provider (for example: openai)")
	cmd.Flags().StringVar(&cfg.LLMModel, "llm-model", "", "llm model (for example: gpt-4o-mini)")
	cmd.Flags().StringVar(&cfg.LLMAPIKeyEnv, "llm-api-key-env", "", "env var name holding llm api key")
	cmd.Flags().StringVar(&cfg.EmbeddingProvider, "embedding-provider", "", "embedding provider")
	cmd.Flags().StringVar(&cfg.EmbeddingModel, "embedding-model", "", "embedding model")
	cmd.Flags().StringVar(&cfg.VectorDBProvider, "vectordb-provider", "", "vector db provider")
	cmd.Flags().StringVar(&cfg.VectorDBURL, "vectordb-url", "", "vector db url")
	cmd.Flags().StringVar(&cfg.VectorDBCollection, "vectordb-collection", "", "vector db collection")
	cmd.Flags().StringVar(&cfg.VectorDBAPIKeyEnv, "vectordb-api-key-env", "", "env var name holding vector db api key")
	cmd.Flags().StringVar(&cfg.PersonaName, "persona-name", "default", "persona profile name")
	cmd.Flags().StringVar(&cfg.PersonaPrompt, "persona-prompt", "", "persona prompt")

	return cmd
}

func newGuideShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show guide configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()
			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "guide show", appErr)
			}
			defer db.Close()

			cfg, found, err := storage.GetGuideConfig(execCtx, db)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to read guide configuration", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "guide show", appErr)
			}
			if !found {
				appErr := output.NewError("not_found", "guide configuration not found", nil)
				return writeErr(cmd, ctx.JSON, "guide show", appErr)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{"config": cfg}, commandMeta("guide show"))
			}

			_ = output.WriteHuman(cmd.OutOrStdout(), "LLM: %s / %s", cfg.LLMProvider, cfg.LLMModel)
			_ = output.WriteHuman(cmd.OutOrStdout(), "LLM API key env: %s", cfg.LLMAPIKeyEnv)
			_ = output.WriteHuman(cmd.OutOrStdout(), "Embedding: %s / %s", cfg.EmbeddingProvider, cfg.EmbeddingModel)
			_ = output.WriteHuman(cmd.OutOrStdout(), "Vector DB: %s @ %s (%s)", cfg.VectorDBProvider, cfg.VectorDBURL, cfg.VectorDBCollection)
			_ = output.WriteHuman(cmd.OutOrStdout(), "Persona: %s", cfg.PersonaName)
			return nil
		},
	}
}

func newGuideValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate guide configuration completeness",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()
			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "guide validate", appErr)
			}
			defer db.Close()

			cfg, found, err := storage.GetGuideConfig(execCtx, db)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to read guide configuration", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "guide validate", appErr)
			}
			if !found {
				appErr := output.NewError("not_found", "guide configuration not found", nil)
				return writeErr(cmd, ctx.JSON, "guide validate", appErr)
			}

			if err := validateGuideConfig(cfg); err != nil {
				return writeErr(cmd, ctx.JSON, "guide validate", err)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{"valid": true}, commandMeta("guide validate"))
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Guide configuration is valid")
		},
	}
}

func validateGuideConfig(cfg storage.GuideConfig) error {
	if cfg.LLMProvider == "" {
		return output.NewError("validation_error", "llm provider is required", map[string]any{"field": "llm-provider"})
	}
	if cfg.LLMModel == "" {
		return output.NewError("validation_error", "llm model is required", map[string]any{"field": "llm-model"})
	}
	if cfg.LLMAPIKeyEnv == "" {
		return output.NewError("validation_error", "llm api key env var name is required", map[string]any{"field": "llm-api-key-env"})
	}
	if cfg.EmbeddingProvider == "" {
		return output.NewError("validation_error", "embedding provider is required", map[string]any{"field": "embedding-provider"})
	}
	if cfg.EmbeddingModel == "" {
		return output.NewError("validation_error", "embedding model is required", map[string]any{"field": "embedding-model"})
	}
	if cfg.VectorDBProvider == "" {
		return output.NewError("validation_error", "vector db provider is required", map[string]any{"field": "vectordb-provider"})
	}
	if cfg.VectorDBURL == "" {
		return output.NewError("validation_error", "vector db url is required", map[string]any{"field": "vectordb-url"})
	}
	if cfg.VectorDBCollection == "" {
		return output.NewError("validation_error", "vector db collection is required", map[string]any{"field": "vectordb-collection"})
	}
	return nil
}

func writeErr(cmd *cobra.Command, jsonMode bool, command string, err error) error {
	if !jsonMode {
		return err
	}

	var appErr *output.AppError
	if errors.As(err, &appErr) {
		_ = output.WriteJSONError(cmd.OutOrStdout(), appErr, commandMeta(command))
	} else {
		_ = output.WriteJSONError(cmd.OutOrStdout(), output.NewError("internal_error", err.Error(), nil), commandMeta(command))
	}
	return output.MarkHandled(err)
}
