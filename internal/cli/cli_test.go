package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuideSetupShowValidateJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	out, _, code := Execute([]string{
		"guide", "setup",
		"--db-path", dbPath,
		"--json",
		"--llm-provider", "openai",
		"--llm-model", "gpt-4o-mini",
		"--llm-api-key-env", "OPENAI_API_KEY",
		"--embedding-provider", "openai",
		"--embedding-model", "text-embedding-3-small",
		"--vectordb-provider", "qdrant",
		"--vectordb-url", "http://localhost:6333",
		"--vectordb-collection", "walkscape",
		"--persona-name", "helper",
	}, nil)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, out)
	}

	var setup map[string]any
	if err := json.Unmarshal([]byte(out), &setup); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	ok, okType := setup["ok"].(bool)
	if !okType || !ok {
		t.Fatalf("expected ok=true, got: %v", setup)
	}

	out, _, code = Execute([]string{"guide", "show", "--db-path", dbPath, "--json"}, nil)
	if code != 0 {
		t.Fatalf("guide show failed: %d %s", code, out)
	}

	out, _, code = Execute([]string{"guide", "validate", "--db-path", dbPath, "--json"}, nil)
	if code != 0 {
		t.Fatalf("guide validate failed: %d %s", code, out)
	}
}

func TestGuideSetupValidationErrorJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	out, _, code := Execute([]string{"guide", "setup", "--db-path", dbPath, "--json"}, nil)
	if code != 2 {
		t.Fatalf("expected code 2, got %d: %s", code, out)
	}
	if !strings.Contains(out, "validation_error") {
		t.Fatalf("expected validation_error in json output: %s", out)
	}
}

func TestCharacterImportListShowJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	importJSON := `{"name":"Ridge","level":42}`
	out, _, code := Execute([]string{"character", "import", "--db-path", dbPath, "--json", "--raw-json", importJSON}, nil)
	if code != 0 {
		t.Fatalf("character import failed: %d %s", code, out)
	}

	var importResp map[string]any
	if err := json.Unmarshal([]byte(out), &importResp); err != nil {
		t.Fatalf("invalid import output json: %v", err)
	}

	data, ok := importResp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got: %T", importResp["data"])
	}
	character, ok := data["character"].(map[string]any)
	if !ok {
		t.Fatalf("expected character object, got: %T", data["character"])
	}
	id, ok := character["id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected non-empty character id, got: %v", character["id"])
	}

	out, _, code = Execute([]string{"character", "list", "--db-path", dbPath, "--json"}, nil)
	if code != 0 {
		t.Fatalf("character list failed: %d %s", code, out)
	}
	if !strings.Contains(out, id) {
		t.Fatalf("expected list to contain id %s: %s", id, out)
	}

	out, _, code = Execute([]string{"character", "show", "--db-path", dbPath, "--json", "--id", id}, nil)
	if code != 0 {
		t.Fatalf("character show by id failed: %d %s", code, out)
	}

	out, _, code = Execute([]string{"character", "show", "--db-path", dbPath, "--json", "--latest"}, nil)
	if code != 0 {
		t.Fatalf("character show latest failed: %d %s", code, out)
	}
}

func TestCharacterImportFromStdin(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	out, _, code := Execute(
		[]string{"character", "import", "--db-path", dbPath, "--json", "--from-stdin"},
		strings.NewReader(`{"name":"stdin"}`),
	)
	if code != 0 {
		t.Fatalf("character import from stdin failed: %d %s", code, out)
	}
}

func TestCharacterShowValidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	out, _, code := Execute([]string{"character", "show", "--db-path", dbPath, "--json"}, nil)
	if code != 2 {
		t.Fatalf("expected validation code 2, got %d: %s", code, out)
	}
}

func TestHumanModeOutput(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	out, errOut, code := Execute([]string{
		"guide", "setup",
		"--db-path", dbPath,
		"--llm-provider", "openai",
		"--llm-model", "gpt-4o-mini",
		"--llm-api-key-env", "OPENAI_API_KEY",
		"--embedding-provider", "openai",
		"--embedding-model", "text-embedding-3-small",
		"--vectordb-provider", "qdrant",
		"--vectordb-url", "http://localhost:6333",
		"--vectordb-collection", "walkscape",
	}, nil)
	if code != 0 {
		t.Fatalf("guide setup failed: code=%d out=%s err=%s", code, out, errOut)
	}
	if !strings.Contains(out, "Guide configuration saved") {
		t.Fatalf("unexpected human output: %s", out)
	}

	out, _, code = Execute([]string{"character", "import", "--db-path", dbPath, "--raw-json", `{"name":"Human"}`}, nil)
	if code != 0 {
		t.Fatalf("character import failed in human mode: %d %s", code, out)
	}

	out, _, code = Execute([]string{"character", "list", "--db-path", dbPath}, nil)
	if code != 0 {
		t.Fatalf("character list failed in human mode: %d %s", code, out)
	}
	if !strings.Contains(out, "Human") {
		t.Fatalf("expected human-mode list output with imported character name: %s", out)
	}
}

func TestCompletionCommand(t *testing.T) {
	out, _, code := Execute([]string{"completion", "bash"}, nil)
	if code != 0 {
		t.Fatalf("completion bash failed: %d", code)
	}
	if !strings.Contains(out, "__start_wsh") {
		t.Fatalf("expected bash completion script output")
	}
}
