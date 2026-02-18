package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"walkscape-helper/internal/output"
	"walkscape-helper/internal/storage"

	"github.com/spf13/cobra"
)

func newCharacterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "character",
		Short: "Manage imported character data",
	}
	cmd.AddCommand(newCharacterImportCmd())
	cmd.AddCommand(newCharacterListCmd())
	cmd.AddCommand(newCharacterShowCmd())
	return cmd
}

func newCharacterImportCmd() *cobra.Command {
	var fromStdin bool
	var fromFile string
	var rawJSON string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import character JSON data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()
			raw, source, err := readCharacterInput(fromStdin, fromFile, rawJSON, cmd.InOrStdin())
			if err != nil {
				return writeErr(cmd, ctx.JSON, "character import", err)
			}
			if err := validateCharacterJSON(raw); err != nil {
				return writeErr(cmd, ctx.JSON, "character import", err)
			}

			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "character import", appErr)
			}
			defer db.Close()

			ch, err := storage.InsertCharacter(execCtx, db, source, raw)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to store character data", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "character import", appErr)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{"character": ch}, commandMeta("character import"))
			}
			if ch.Name != "" {
				return output.WriteHuman(cmd.OutOrStdout(), "Imported character %s (%s)", ch.Name, ch.ID)
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Imported character %s", ch.ID)
		},
	}

	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read JSON from stdin")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read JSON from file path")
	cmd.Flags().StringVar(&rawJSON, "raw-json", "", "inline JSON string")
	return cmd
}

func newCharacterListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List imported characters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()
			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "character list", appErr)
			}
			defer db.Close()

			chars, err := storage.ListCharacters(execCtx, db)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to list characters", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "character list", appErr)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{"characters": chars}, commandMeta("character list"))
			}

			if len(chars) == 0 {
				return output.WriteHuman(cmd.OutOrStdout(), "No characters imported")
			}
			for _, ch := range chars {
				name := ch.Name
				if name == "" {
					name = "<unknown>"
				}
				if err := output.WriteHuman(cmd.OutOrStdout(), "%s %s %s", ch.ID, name, ch.ImportedAt); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newCharacterShowCmd() *cobra.Command {
	var id string
	var latest bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show one imported character",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()
			if !latest && id == "" {
				return writeErr(cmd, ctx.JSON, "character show", output.NewError("validation_error", "either --id or --latest is required", nil))
			}
			if latest && id != "" {
				return writeErr(cmd, ctx.JSON, "character show", output.NewError("validation_error", "use only one of --id or --latest", nil))
			}

			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "character show", appErr)
			}
			defer db.Close()

			var ch storage.Character
			var found bool
			if latest {
				ch, found, err = storage.GetLatestCharacter(execCtx, db)
			} else {
				ch, found, err = storage.GetCharacterByID(execCtx, db, id)
			}
			if err != nil {
				appErr := output.NewError("storage_error", "failed to fetch character", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "character show", appErr)
			}
			if !found {
				return writeErr(cmd, ctx.JSON, "character show", output.NewError("not_found", "character not found", nil))
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{"character": ch}, commandMeta("character show"))
			}
			if ch.Name != "" {
				if err := output.WriteHuman(cmd.OutOrStdout(), "Name: %s", ch.Name); err != nil {
					return err
				}
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "ID: %s", ch.ID); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Source: %s", ch.Source); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Imported: %s", ch.ImportedAt); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), ch.RawJSON)
			return err
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "character id")
	cmd.Flags().BoolVar(&latest, "latest", false, "show latest imported character")
	return cmd
}

func validateCharacterJSON(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return output.NewError("validation_error", "character json is empty", nil)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return output.NewError("validation_error", "character payload must be valid JSON object", map[string]any{"reason": err.Error()})
	}
	if payload == nil {
		return output.NewError("validation_error", "character payload must be JSON object", nil)
	}
	return nil
}

func readCharacterInput(fromStdin bool, fromFile, rawJSON string, stdin io.Reader) ([]byte, string, error) {
	provided := 0
	if fromStdin {
		provided++
	}
	if fromFile != "" {
		provided++
	}
	if rawJSON != "" {
		provided++
	}
	if provided != 1 {
		return nil, "", output.NewError("validation_error", "use exactly one of --from-stdin, --from-file, or --raw-json", nil)
	}

	if fromStdin {
		buf, err := io.ReadAll(stdin)
		if err != nil {
			return nil, "", output.NewError("validation_error", "failed reading stdin", map[string]any{"reason": err.Error()})
		}
		return buf, "stdin", nil
	}

	if rawJSON != "" {
		return []byte(rawJSON), "raw", nil
	}

	buf, err := os.ReadFile(fromFile)
	if err != nil {
		return nil, "", output.NewError("validation_error", "failed reading file", map[string]any{"reason": err.Error()})
	}
	return buf, "file", nil
}
