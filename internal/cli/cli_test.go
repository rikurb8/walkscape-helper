package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestWikiScrapeFullCreatesSnapshotSkeletonJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	outRoot := filepath.Join(t.TempDir(), "wiki-out")
	api := newWikiMockAPIServer(t)
	t.Cleanup(api.Close)

	out, _, code := Execute([]string{"wiki", "scrape", "full", "--db-path", dbPath, "--json", "--out", outRoot, "--api-base-url", api.URL, "--rate-limit-rps", "1000"}, nil)
	if code != 0 {
		t.Fatalf("wiki scrape full failed: %d %s", code, out)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", resp["data"])
	}
	namespaces, ok := data["namespaces"].([]any)
	if !ok {
		t.Fatalf("expected namespaces array, got %T", data["namespaces"])
	}
	if len(namespaces) != 1 {
		t.Fatalf("expected one default namespace, got %d (%v)", len(namespaces), namespaces)
	}
	if ns, ok := namespaces[0].(float64); !ok || int(ns) != 0 {
		t.Fatalf("expected default namespace 0, got %v", namespaces[0])
	}
	snapshotDir, ok := data["snapshot_dir"].(string)
	if !ok || snapshotDir == "" {
		t.Fatalf("expected snapshot_dir in response, got %v", data["snapshot_dir"])
	}
	counts, ok := data["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object, got %T", data["counts"])
	}
	if pages, ok := counts["wiki_pages"].(float64); !ok || int(pages) != 2 {
		t.Fatalf("expected wiki_pages count 2, got %v", counts["wiki_pages"])
	}

	expectedPaths := []string{
		filepath.Join(snapshotDir, "manifest.json"),
		filepath.Join(snapshotDir, "checksums.sha256"),
		filepath.Join(snapshotDir, "raw", "api", filepath.Base(snapshotDir)),
		filepath.Join(snapshotDir, "normalized", "wiki_pages.ndjson"),
		filepath.Join(snapshotDir, "normalized", "wiki_revisions.ndjson"),
		filepath.Join(snapshotDir, "normalized", "wiki_page_edges.ndjson"),
		filepath.Join(snapshotDir, "normalized", "wiki_files.ndjson"),
		filepath.Join(snapshotDir, "normalized", "wiki_tombstones.ndjson"),
	}

	for _, p := range expectedPaths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %s to exist: %v", p, err)
		}
	}
}

func TestWikiScrapeFullRejectsInvalidNamespace(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	out, _, code := Execute([]string{"wiki", "scrape", "full", "--db-path", dbPath, "--json", "--namespaces", "abc"}, nil)
	if code != 2 {
		t.Fatalf("expected code 2, got %d: %s", code, out)
	}
	if !strings.Contains(out, "validation_error") {
		t.Fatalf("expected validation_error in output: %s", out)
	}
}

func TestWikiScrapeFullAppliesCategoryFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	outRoot := filepath.Join(t.TempDir(), "wiki-out")
	api := newWikiMockAPIServer(t)
	t.Cleanup(api.Close)

	out, _, code := Execute([]string{"wiki", "scrape", "full", "--db-path", dbPath, "--json", "--out", outRoot, "--api-base-url", api.URL, "--rate-limit-rps", "1000", "--include-categories", "Nonexistent Category"}, nil)
	if code != 0 {
		t.Fatalf("wiki scrape full failed: %d %s", code, out)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object")
	}
	counts, ok := data["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object")
	}
	if pages, ok := counts["wiki_pages"].(float64); !ok || int(pages) != 0 {
		t.Fatalf("expected wiki_pages count 0, got %v", counts["wiki_pages"])
	}
}

func TestWikiStatusShowsInitializedCursor(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	outRoot := filepath.Join(t.TempDir(), "wiki-out")
	api := newWikiMockAPIServer(t)
	t.Cleanup(api.Close)

	_, _, code := Execute([]string{"wiki", "scrape", "full", "--db-path", dbPath, "--json", "--out", outRoot, "--api-base-url", api.URL, "--rate-limit-rps", "1000"}, nil)
	if code != 0 {
		t.Fatalf("wiki scrape full failed")
	}

	out, _, code := Execute([]string{"wiki", "status", "--db-path", dbPath, "--out", outRoot, "--json"}, nil)
	if code != 0 {
		t.Fatalf("wiki status failed: %d %s", code, out)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid status json: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object")
	}
	if ready, ok := data["incremental_ready"].(bool); !ok || !ready {
		t.Fatalf("expected incremental_ready=true, got %v", data["incremental_ready"])
	}
}

func TestWikiScrapeUpdateFetchesRecentChanges(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	outRoot := filepath.Join(t.TempDir(), "wiki-out")
	api := newWikiMockAPIServer(t)
	t.Cleanup(api.Close)

	_, _, code := Execute([]string{"wiki", "scrape", "full", "--db-path", dbPath, "--json", "--out", outRoot, "--api-base-url", api.URL, "--rate-limit-rps", "1000"}, nil)
	if code != 0 {
		t.Fatalf("wiki scrape full failed")
	}

	out, _, code := Execute([]string{"wiki", "scrape", "update", "--db-path", dbPath, "--json", "--out", outRoot, "--api-base-url", api.URL, "--rate-limit-rps", "1000", "--since", "2026-02-19T00:00:00Z"}, nil)
	if code != 0 {
		t.Fatalf("wiki scrape update failed: %d %s", code, out)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid update json: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object")
	}
	if events, ok := data["event_count"].(float64); !ok || int(events) < 1 {
		t.Fatalf("expected event_count >= 1, got %v", data["event_count"])
	}
	if changedPages, ok := data["changed_page_count"].(float64); !ok || int(changedPages) != 1 {
		t.Fatalf("expected changed_page_count to be deduped to 1, got %v", data["changed_page_count"])
	}
}

func TestWikiScrapeUpdateRequiresInitializedCursor(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	outRoot := filepath.Join(t.TempDir(), "wiki-out")
	api := newWikiMockAPIServer(t)
	t.Cleanup(api.Close)

	out, _, code := Execute([]string{"wiki", "scrape", "update", "--db-path", dbPath, "--json", "--out", outRoot, "--api-base-url", api.URL}, nil)
	if code != 3 {
		t.Fatalf("expected code 3, got %d: %s", code, out)
	}
	if !strings.Contains(out, "not_found") {
		t.Fatalf("expected not_found response, got %s", out)
	}
}

func TestWikiStatusNoSnapshotRecommendsFull(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	outRoot := filepath.Join(t.TempDir(), "wiki-out")

	out, _, code := Execute([]string{"wiki", "status", "--db-path", dbPath, "--out", outRoot, "--json"}, nil)
	if code != 0 {
		t.Fatalf("wiki status failed: %d %s", code, out)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid status json: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object")
	}
	if cmd, ok := data["recommended_next_command"].(string); !ok || cmd != "wiki scrape full" {
		t.Fatalf("expected recommended_next_command to be wiki scrape full, got %v", data["recommended_next_command"])
	}
}

func newWikiMockAPIServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("action") != "query" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid action"}`))
			return
		}

		if q.Get("list") == "allpages" {
			_, _ = w.Write([]byte(`{"batchcomplete":"","query":{"allpages":[{"pageid":1,"ns":0,"title":"Skills"},{"pageid":2,"ns":0,"title":"Activities"}]}}`))
			return
		}

		if q.Get("list") == "recentchanges" {
			_, _ = w.Write([]byte(`{"batchcomplete":"","query":{"recentchanges":[{"rcid":200,"type":"edit","ns":0,"title":"Skills","pageid":1,"revid":103,"old_revid":101,"timestamp":"2026-02-19T00:02:00Z","comment":"update"}]}}`))
			return
		}

		if q.Get("prop") != "" {
			titles := q.Get("titles")
			skills := `"1":{"pageid":1,"ns":0,"title":"Skills","fullurl":"https://wiki.walkscape.app/wiki/Skills","revisions":[{"revid":101,"parentid":100,"timestamp":"2026-02-19T00:00:00Z","sha1":"abc","size":123,"comment":"seed","slots":{"main":{"contentmodel":"wikitext","*":"== Skills ==\ntext"}}}],"categories":[{"title":"Category:Skills"}],"templates":[{"title":"Template:Infobox"}],"links":[{"title":"Activities"}],"langlinks":[{"lang":"en","*":"Skills"}]}`
			activities := `"2":{"pageid":2,"ns":0,"title":"Activities","fullurl":"https://wiki.walkscape.app/wiki/Activities","revisions":[{"revid":102,"parentid":101,"timestamp":"2026-02-19T00:01:00Z","sha1":"def","size":99,"comment":"seed","slots":{"main":{"contentmodel":"wikitext","*":"== Activities ==\ntext"}}}],"categories":[{"title":"Category:Activities"}],"templates":[],"links":[],"langlinks":[]}`
			switch {
			case strings.Contains(titles, "Skills") && strings.Contains(titles, "Activities"):
				_, _ = w.Write([]byte(`{"batchcomplete":"","query":{"pages":{` + skills + `,` + activities + `}}}`))
			case strings.Contains(titles, "Skills"):
				_, _ = w.Write([]byte(`{"batchcomplete":"","query":{"pages":{` + skills + `}}}`))
			case strings.Contains(titles, "Activities"):
				_, _ = w.Write([]byte(`{"batchcomplete":"","query":{"pages":{` + activities + `}}}`))
			default:
				_, _ = w.Write([]byte(`{"batchcomplete":"","query":{"pages":{}}}`))
			}
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"unsupported"}`))
	}))
}
