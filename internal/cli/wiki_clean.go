package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"walkscape-helper/internal/output"

	"github.com/spf13/cobra"
)

const wikiCleanerVersion = "wiki-clean-v1"

var (
	wikiHeadingPattern      = regexp.MustCompile(`^\s*(={2,6})\s*(.*?)\s*(={2,6})\s*$`)
	wikiInternalLinkPattern = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
)

type wikiTaxonomyTopic struct {
	Group   string
	Topic   string
	Landing string
	Labels  []string
}

type wikiTaxonomyMatch struct {
	Group      string
	Topic      string
	Landing    string
	Source     string
	Confidence float64
	Aliases    []string
}

type wikiCleanConfig struct {
	CleanerVersion string   `json:"cleaner_version"`
	SnapshotDir    string   `json:"snapshot_dir"`
	OutputDir      string   `json:"output_dir"`
	Namespaces     []int    `json:"namespaces"`
	Langs          []string `json:"langs,omitempty"`
	EligiblePages  int      `json:"eligible_pages"`
}

type wikiCleanedPageRecord struct {
	PageID             int      `json:"page_id"`
	Title              string   `json:"title"`
	Namespace          int      `json:"namespace"`
	LangCode           string   `json:"lang_code,omitempty"`
	RevisionID         int      `json:"revision_id"`
	RevisionTS         string   `json:"revision_ts"`
	ContentSHA1        string   `json:"content_sha1,omitempty"`
	MarkdownPath       string   `json:"markdown_path"`
	CleanStatus        string   `json:"clean_status"`
	Warnings           []string `json:"warnings,omitempty"`
	CleanerVersion     string   `json:"cleaner_version"`
	DomainGroup        string   `json:"domain_group"`
	DomainTopic        string   `json:"domain_topic"`
	LandingPath        string   `json:"landing_path,omitempty"`
	TaxonomySource     string   `json:"taxonomy_source"`
	TaxonomyConfidence float64  `json:"taxonomy_confidence"`
	TopicAliases       []string `json:"topic_aliases,omitempty"`
}

type wikiCleaningReportRecord struct {
	PageID      int    `json:"page_id"`
	Title       string `json:"title"`
	Namespace   int    `json:"namespace"`
	RevisionID  int    `json:"revision_id,omitempty"`
	Level       string `json:"level"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	SnapshotDir string `json:"snapshot_dir"`
}

type wikiPageEdges struct {
	Categories []string
	Links      []string
}

type wikiCleanSummary struct {
	OutputDir     string
	CleanedPath   string
	ReportPath    string
	EligiblePages int
	CleanedPages  int
	StatusCounts  map[string]int
}

var wikiTaxonomyTopics = []wikiTaxonomyTopic{
	{Group: "game-mechanics", Topic: "core-mechanics", Landing: "Game Mechanics > Core Mechanics", Labels: []string{"core mechanics"}},
	{Group: "game-mechanics", Topic: "skills", Landing: "Game Mechanics > Skills", Labels: []string{"skills", "skill"}},
	{Group: "game-mechanics", Topic: "activities", Landing: "Game Mechanics > Activities", Labels: []string{"activities", "activity"}},
	{Group: "game-mechanics", Topic: "recipes", Landing: "Game Mechanics > Recipes", Labels: []string{"recipes", "recipe"}},
	{Group: "game-mechanics", Topic: "achievements", Landing: "Game Mechanics > Achievements", Labels: []string{"achievements", "achievement"}},
	{Group: "game-mechanics", Topic: "attributes", Landing: "Game Mechanics > Attributes", Labels: []string{"attributes", "attribute"}},
	{Group: "game-mechanics", Topic: "job-boards", Landing: "Game Mechanics > Job Boards", Labels: []string{"job boards", "job board"}},
	{Group: "game-mechanics", Topic: "keywords", Landing: "Game Mechanics > Keywords", Labels: []string{"keywords", "keyword"}},
	{Group: "game-mechanics", Topic: "abilities", Landing: "Game Mechanics > Abilities", Labels: []string{"abilities", "ability"}},
	{Group: "game-mechanics", Topic: "rumors", Landing: "Game Mechanics > Rumors", Labels: []string{"rumors", "rumor"}},
	{Group: "game-mechanics", Topic: "tips", Landing: "Game Mechanics > Tips", Labels: []string{"tips", "tip"}},
	{Group: "items", Topic: "equipment", Landing: "Items > Equipment", Labels: []string{"equipment"}},
	{Group: "items", Topic: "materials", Landing: "Items > Materials", Labels: []string{"materials", "material"}},
	{Group: "items", Topic: "consumables", Landing: "Items > Consumables", Labels: []string{"consumables", "consumable"}},
	{Group: "items", Topic: "collectibles", Landing: "Items > Collectibles", Labels: []string{"collectibles", "collectible"}},
	{Group: "items", Topic: "chests", Landing: "Items > Chests", Labels: []string{"chests", "chest"}},
	{Group: "items", Topic: "pet-eggs", Landing: "Items > Pet Eggs", Labels: []string{"pet eggs", "pet egg"}},
	{Group: "items", Topic: "cosmetics", Landing: "Items > Cosmetics", Labels: []string{"cosmetics", "cosmetic"}},
}

func newWikiCleanCmd() *cobra.Command {
	var snapshotDir string
	var outDir string
	var namespaces []string
	var langs []string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean wiki wikitext into markdown",
		Long:  "Transform normalized wiki revisions into cleaned markdown with deterministic metadata and taxonomy paths.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			nsIDs, err := parseNamespaceIDs(namespaces)
			if err != nil {
				return writeErr(cmd, ctx.JSON, "wiki clean", err)
			}

			if strings.TrimSpace(snapshotDir) == "" {
				return writeErr(cmd, ctx.JSON, "wiki clean", output.NewError("validation_error", "snapshot is required", map[string]any{"field": "snapshot"}))
			}

			resolvedOut := strings.TrimSpace(outDir)
			if resolvedOut == "" {
				resolvedOut = snapshotDir
			}

			summary, err := runWikiClean(snapshotDir, resolvedOut, nsIDs, langs)
			if err != nil {
				return writeErr(cmd, ctx.JSON, "wiki clean", err)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{
					"snapshot_dir":    snapshotDir,
					"output_dir":      summary.OutputDir,
					"eligible_pages":  summary.EligiblePages,
					"cleaned_pages":   summary.CleanedPages,
					"status_counts":   summary.StatusCounts,
					"cleaned_records": summary.CleanedPath,
					"report_path":     summary.ReportPath,
					"cleaner_version": wikiCleanerVersion,
				}, commandMeta("wiki clean"))
			}

			if err := output.WriteHuman(cmd.OutOrStdout(), "Cleaned %d/%d pages", summary.CleanedPages, summary.EligiblePages); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Records: %s", summary.CleanedPath); err != nil {
				return err
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Report: %s", summary.ReportPath)
		},
	}

	cmd.Flags().StringVar(&snapshotDir, "snapshot", "", "snapshot directory to clean")
	cmd.Flags().StringVar(&outDir, "out", "", "output directory root (defaults to --snapshot)")
	cmd.Flags().StringSliceVar(&namespaces, "namespaces", []string{"0"}, "namespace ids to include")
	cmd.Flags().StringSliceVar(&langs, "lang", nil, "language codes to include (empty means all)")

	cmd.AddCommand(newWikiCleanValidateCmd())

	return cmd
}

func newWikiCleanValidateCmd() *cobra.Command {
	var snapshotDir string
	var outDir string
	var warningBudget float64

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate cleaned wiki markdown corpus",
		Long:  "Run integrity and quality checks for cleaned wiki markdown outputs.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			if strings.TrimSpace(snapshotDir) == "" {
				return writeErr(cmd, ctx.JSON, "wiki clean validate", output.NewError("validation_error", "snapshot is required", map[string]any{"field": "snapshot"}))
			}
			if warningBudget < 0 || warningBudget > 1 {
				return writeErr(cmd, ctx.JSON, "wiki clean validate", output.NewError("validation_error", "warning-budget must be between 0 and 1", map[string]any{"field": "warning-budget"}))
			}

			resolvedOut := strings.TrimSpace(outDir)
			if resolvedOut == "" {
				resolvedOut = snapshotDir
			}

			result, err := runWikiCleanValidate(snapshotDir, resolvedOut, warningBudget)
			if err != nil {
				return writeErr(cmd, ctx.JSON, "wiki clean validate", err)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), result, commandMeta("wiki clean validate"))
			}

			if err := output.WriteHuman(cmd.OutOrStdout(), "Validation passed for cleaned wiki corpus"); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Coverage: %v", result["coverage"]); err != nil {
				return err
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Warning ratio: %.4f", result["warning_ratio"])
		},
	}

	cmd.Flags().StringVar(&snapshotDir, "snapshot", "", "snapshot directory to validate")
	cmd.Flags().StringVar(&outDir, "out", "", "clean output directory root (defaults to --snapshot)")
	cmd.Flags().Float64Var(&warningBudget, "warning-budget", 0.25, "maximum allowed partial/failed ratio")

	return cmd
}

//nolint:gocyclo,gocritic
func runWikiClean(snapshotDir, outDir string, nsIDs []int, langs []string) (wikiCleanSummary, error) {
	summary := wikiCleanSummary{StatusCounts: map[string]int{"ok": 0, "partial": 0, "failed": 0, "skipped": 0}}
	if strings.TrimSpace(snapshotDir) == "" {
		return summary, output.NewError("validation_error", "snapshot is required", map[string]any{"field": "snapshot"})
	}

	pagesPath := filepath.Join(snapshotDir, "normalized", "wiki_pages.ndjson")
	revsPath := filepath.Join(snapshotDir, "normalized", "wiki_revisions.ndjson")
	edgesPath := filepath.Join(snapshotDir, "normalized", "wiki_page_edges.ndjson")

	pages, err := readWikiPageRecords(pagesPath)
	if err != nil {
		return summary, output.NewError("storage_error", "failed to read wiki pages", map[string]any{"reason": err.Error()})
	}
	revisions, err := readWikiRevisionRecords(revsPath)
	if err != nil {
		return summary, output.NewError("storage_error", "failed to read wiki revisions", map[string]any{"reason": err.Error()})
	}
	edges, err := readWikiEdgeRecords(edgesPath)
	if err != nil {
		return summary, output.NewError("storage_error", "failed to read wiki edges", map[string]any{"reason": err.Error()})
	}

	nsSet := make(map[int]struct{}, len(nsIDs))
	for _, ns := range nsIDs {
		nsSet[ns] = struct{}{}
	}
	langSet := normalizeLangFilter(langs)

	revisionsByID := make(map[int]wikiRevisionRecord, len(revisions))
	for _, revision := range revisions {
		revisionsByID[revision.RevisionID] = revision
	}

	edgesByPage := make(map[int]wikiPageEdges)
	for _, edge := range edges {
		bucket := edgesByPage[edge.PageID]
		switch edge.EdgeType {
		case "category":
			bucket.Categories = append(bucket.Categories, edge.Target)
		case "link":
			bucket.Links = append(bucket.Links, edge.Target)
		}
		edgesByPage[edge.PageID] = bucket
	}

	eligiblePages := make([]wikiPageRecord, 0, len(pages))
	for _, page := range pages {
		if _, ok := nsSet[page.Namespace]; !ok {
			continue
		}
		if !matchesLangFilter(page.LangCode, langSet) {
			continue
		}
		if page.LatestRevisionID == 0 {
			continue
		}
		if _, ok := revisionsByID[page.LatestRevisionID]; !ok {
			continue
		}
		eligiblePages = append(eligiblePages, page)
	}

	sort.Slice(eligiblePages, func(i, j int) bool {
		if eligiblePages[i].Namespace != eligiblePages[j].Namespace {
			return eligiblePages[i].Namespace < eligiblePages[j].Namespace
		}
		if eligiblePages[i].Title != eligiblePages[j].Title {
			return eligiblePages[i].Title < eligiblePages[j].Title
		}
		return eligiblePages[i].PageID < eligiblePages[j].PageID
	})

	cleanedRoot := filepath.Join(outDir, "cleaned")
	markdownRoot := filepath.Join(cleanedRoot, "markdown")
	mkdirErr := os.MkdirAll(markdownRoot, 0o755)
	if mkdirErr != nil {
		return summary, output.NewError("storage_error", "failed creating cleaned markdown directory", map[string]any{"reason": mkdirErr.Error()})
	}

	cleanedPath := filepath.Join(cleanedRoot, "cleaned_pages.ndjson")
	reportPath := filepath.Join(cleanedRoot, "cleaning_report.ndjson")
	writeErr := os.WriteFile(cleanedPath, nil, privateFileMode)
	if writeErr != nil {
		return summary, output.NewError("storage_error", "failed initializing cleaned pages file", map[string]any{"reason": writeErr.Error()})
	}
	writeErr = os.WriteFile(reportPath, nil, privateFileMode)
	if writeErr != nil {
		return summary, output.NewError("storage_error", "failed initializing cleaning report file", map[string]any{"reason": writeErr.Error()})
	}

	for _, page := range eligiblePages {
		revision := revisionsByID[page.LatestRevisionID]
		classification := classifyWikiPage(page, edgesByPage[page.PageID])
		body, bodyWarnings := cleanWikiWikitext(revision.Wikitext)
		status := "ok"
		warnings := append([]string{}, bodyWarnings...)

		if strings.TrimSpace(revision.Wikitext) == "" {
			status = "skipped"
			warnings = append(warnings, "source_wikitext_empty")
		}
		if strings.TrimSpace(body) == "" && status == "ok" {
			status = "partial"
			warnings = append(warnings, "cleaned_markdown_empty")
			body = plainTextFallback(revision.Wikitext)
		}

		relMarkdownPath := wikiMarkdownRelPath(page, classification)
		absMarkdownPath := filepath.Join(outDir, relMarkdownPath)
		mkdirErr = os.MkdirAll(filepath.Dir(absMarkdownPath), 0o755)
		if mkdirErr != nil {
			return summary, output.NewError("storage_error", "failed creating markdown output path", map[string]any{"reason": mkdirErr.Error()})
		}

		markdownDoc := buildWikiMarkdownDocument(page, revision, classification, body)
		writeErr = os.WriteFile(absMarkdownPath, []byte(markdownDoc), privateFileMode)
		if writeErr != nil {
			return summary, output.NewError("storage_error", "failed writing cleaned markdown file", map[string]any{"reason": writeErr.Error(), "path": relMarkdownPath})
		}

		record := wikiCleanedPageRecord{
			PageID:             page.PageID,
			Title:              page.Title,
			Namespace:          page.Namespace,
			LangCode:           page.LangCode,
			RevisionID:         revision.RevisionID,
			RevisionTS:         revision.Timestamp,
			ContentSHA1:        revision.SHA1,
			MarkdownPath:       relMarkdownPath,
			CleanStatus:        status,
			Warnings:           warnings,
			CleanerVersion:     wikiCleanerVersion,
			DomainGroup:        classification.Group,
			DomainTopic:        classification.Topic,
			LandingPath:        classification.Landing,
			TaxonomySource:     classification.Source,
			TaxonomyConfidence: classification.Confidence,
			TopicAliases:       classification.Aliases,
		}

		appendErr := appendNDJSON(cleanedPath, record)
		if appendErr != nil {
			return summary, output.NewError("storage_error", "failed writing cleaned record", map[string]any{"reason": appendErr.Error()})
		}

		summary.CleanedPages++
		summary.StatusCounts[status]++

		for _, warningCode := range warnings {
			reportRecord := wikiCleaningReportRecord{
				PageID:      page.PageID,
				Title:       page.Title,
				Namespace:   page.Namespace,
				RevisionID:  revision.RevisionID,
				Level:       "warning",
				Code:        warningCode,
				Message:     warningCode,
				SnapshotDir: snapshotDir,
			}
			appendErr = appendNDJSON(reportPath, reportRecord)
			if appendErr != nil {
				return summary, output.NewError("storage_error", "failed writing cleaning report", map[string]any{"reason": appendErr.Error()})
			}
		}
	}

	summary.EligiblePages = len(eligiblePages)
	summary.OutputDir = outDir
	summary.CleanedPath = cleanedPath
	summary.ReportPath = reportPath

	cleanCfg := wikiCleanConfig{
		CleanerVersion: wikiCleanerVersion,
		SnapshotDir:    snapshotDir,
		OutputDir:      outDir,
		Namespaces:     nsIDs,
		Langs:          sortedKeys(langSet),
		EligiblePages:  len(eligiblePages),
	}
	configRaw, err := json.MarshalIndent(cleanCfg, "", "  ")
	if err != nil {
		return summary, output.NewError("storage_error", "failed encoding clean config", map[string]any{"reason": err.Error()})
	}
	writeErr = os.WriteFile(filepath.Join(cleanedRoot, "clean_config.json"), append(configRaw, '\n'), privateFileMode)
	if writeErr != nil {
		return summary, output.NewError("storage_error", "failed writing clean config", map[string]any{"reason": writeErr.Error()})
	}

	return summary, nil
}

//nolint:gocritic
func runWikiCleanValidate(snapshotDir, outDir string, warningBudget float64) (map[string]any, error) {
	cleanedRoot := filepath.Join(outDir, "cleaned")
	cleanedPath := filepath.Join(cleanedRoot, "cleaned_pages.ndjson")
	configPath := filepath.Join(cleanedRoot, "clean_config.json")

	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, output.NewError("not_found", "clean config not found; run wiki clean first", map[string]any{"reason": err.Error(), "path": configPath})
	}

	var cfg wikiCleanConfig
	unmarshalErr := json.Unmarshal(configRaw, &cfg)
	if unmarshalErr != nil {
		return nil, output.NewError("validation_error", "invalid clean config file", map[string]any{"reason": unmarshalErr.Error(), "path": configPath})
	}

	cleanedRecords, err := readWikiCleanedPageRecords(cleanedPath)
	if err != nil {
		return nil, output.NewError("storage_error", "failed reading cleaned records", map[string]any{"reason": err.Error(), "path": cleanedPath})
	}

	eligiblePages, revsByID, err := eligibleWikiPagesWithRevisions(snapshotDir, cfg.Namespaces, cfg.Langs)
	if err != nil {
		return nil, err
	}

	if len(cleanedRecords) != len(eligiblePages) {
		return nil, output.NewError("validation_error", "coverage mismatch between eligible and cleaned pages", map[string]any{"eligible_pages": len(eligiblePages), "cleaned_pages": len(cleanedRecords)})
	}

	statusCounts := map[string]int{"ok": 0, "partial": 0, "failed": 0, "skipped": 0}
	for _, rec := range cleanedRecords {
		statusCounts[rec.CleanStatus]++
		fullPath := filepath.Join(outDir, rec.MarkdownPath)
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, output.NewError("validation_error", "cleaned markdown file missing", map[string]any{"path": rec.MarkdownPath, "reason": err.Error()})
		}
		body := strings.TrimSpace(markdownBody(raw))
		if rec.CleanStatus == "ok" && body == "" {
			return nil, output.NewError("validation_error", "ok cleaned page has empty markdown body", map[string]any{"path": rec.MarkdownPath, "page_id": rec.PageID})
		}

		revision, ok := revsByID[rec.RevisionID]
		if ok && hasWikiSectionMarkers(revision.Wikitext) && !strings.Contains(body, "#") {
			return nil, output.NewError("validation_error", "section headers missing in cleaned markdown", map[string]any{"path": rec.MarkdownPath, "page_id": rec.PageID})
		}
	}

	warningRatio := 0.0
	total := len(cleanedRecords)
	if total > 0 {
		warningRatio = float64(statusCounts["partial"]+statusCounts["failed"]) / float64(total)
	}
	if warningRatio > warningBudget {
		return nil, output.NewError("validation_error", "warning ratio exceeds budget", map[string]any{"warning_ratio": warningRatio, "warning_budget": warningBudget})
	}

	return map[string]any{
		"snapshot_dir":    snapshotDir,
		"output_dir":      outDir,
		"cleaner_version": cfg.CleanerVersion,
		"coverage": map[string]any{
			"eligible_pages": len(eligiblePages),
			"cleaned_pages":  len(cleanedRecords),
		},
		"status_counts":  statusCounts,
		"warning_ratio":  warningRatio,
		"warning_budget": warningBudget,
	}, nil
}

func readWikiPageRecords(path string) ([]wikiPageRecord, error) {
	lines, err := readNDJSONLines(path)
	if err != nil {
		return nil, err
	}
	records := make([]wikiPageRecord, 0, len(lines))
	for _, line := range lines {
		var record wikiPageRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func readWikiRevisionRecords(path string) ([]wikiRevisionRecord, error) {
	lines, err := readNDJSONLines(path)
	if err != nil {
		return nil, err
	}
	records := make([]wikiRevisionRecord, 0, len(lines))
	for _, line := range lines {
		var record wikiRevisionRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func readWikiEdgeRecords(path string) ([]wikiEdgeRecord, error) {
	lines, err := readNDJSONLines(path)
	if err != nil {
		return nil, err
	}
	records := make([]wikiEdgeRecord, 0, len(lines))
	for _, line := range lines {
		var record wikiEdgeRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func readWikiCleanedPageRecords(path string) ([]wikiCleanedPageRecord, error) {
	lines, err := readNDJSONLines(path)
	if err != nil {
		return nil, err
	}
	records := make([]wikiCleanedPageRecord, 0, len(lines))
	for _, line := range lines {
		var record wikiCleanedPageRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func readNDJSONLines(path string) ([][]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := make([][]byte, 0)
	for _, line := range bytes.Split(raw, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines, nil
}

//nolint:gocritic
func eligibleWikiPagesWithRevisions(snapshotDir string, nsIDs []int, langs []string) (map[int]wikiPageRecord, map[int]wikiRevisionRecord, error) {
	pagesPath := filepath.Join(snapshotDir, "normalized", "wiki_pages.ndjson")
	revsPath := filepath.Join(snapshotDir, "normalized", "wiki_revisions.ndjson")

	pages, err := readWikiPageRecords(pagesPath)
	if err != nil {
		return nil, nil, output.NewError("storage_error", "failed to read wiki pages", map[string]any{"reason": err.Error()})
	}
	revisions, err := readWikiRevisionRecords(revsPath)
	if err != nil {
		return nil, nil, output.NewError("storage_error", "failed to read wiki revisions", map[string]any{"reason": err.Error()})
	}

	nsSet := make(map[int]struct{}, len(nsIDs))
	for _, ns := range nsIDs {
		nsSet[ns] = struct{}{}
	}
	langSet := normalizeLangFilter(langs)

	revsByID := make(map[int]wikiRevisionRecord, len(revisions))
	for _, rev := range revisions {
		revsByID[rev.RevisionID] = rev
	}

	eligible := make(map[int]wikiPageRecord)
	for _, page := range pages {
		if _, ok := nsSet[page.Namespace]; !ok {
			continue
		}
		if !matchesLangFilter(page.LangCode, langSet) {
			continue
		}
		if page.LatestRevisionID == 0 {
			continue
		}
		if _, ok := revsByID[page.LatestRevisionID]; !ok {
			continue
		}
		eligible[page.PageID] = page
	}

	return eligible, revsByID, nil
}

//nolint:gocritic
func classifyWikiPage(page wikiPageRecord, edges wikiPageEdges) wikiTaxonomyMatch {
	matched := make([]wikiTaxonomyMatch, 0, 4)

	for _, category := range edges.Categories {
		topic, ok := wikiTopicFromText(category)
		if !ok {
			continue
		}
		matched = appendUniqueMatch(matched, wikiTaxonomyMatch{
			Group:      topic.Group,
			Topic:      topic.Topic,
			Landing:    topic.Landing,
			Source:     "category",
			Confidence: 1,
		})
	}

	for _, link := range edges.Links {
		topic, ok := wikiTopicFromText(link)
		if !ok {
			continue
		}
		matched = appendUniqueMatch(matched, wikiTaxonomyMatch{
			Group:      topic.Group,
			Topic:      topic.Topic,
			Landing:    topic.Landing,
			Source:     "link_graph",
			Confidence: 0.8,
		})
	}

	titleTopic, ok := wikiTopicFromText(page.Title)
	if ok {
		matched = appendUniqueMatch(matched, wikiTaxonomyMatch{
			Group:      titleTopic.Group,
			Topic:      titleTopic.Topic,
			Landing:    titleTopic.Landing,
			Source:     "heuristic",
			Confidence: 0.6,
		})
	}

	if len(matched) == 0 {
		return wikiTaxonomyMatch{
			Group:      "uncategorized",
			Topic:      "uncategorized",
			Landing:    "Uncategorized",
			Source:     "fallback",
			Confidence: 0,
			Aliases:    nil,
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Confidence != matched[j].Confidence {
			return matched[i].Confidence > matched[j].Confidence
		}
		idxI := wikiTopicIndex(matched[i].Group, matched[i].Topic)
		idxJ := wikiTopicIndex(matched[j].Group, matched[j].Topic)
		if idxI != idxJ {
			return idxI < idxJ
		}
		return matched[i].Topic < matched[j].Topic
	})

	canonical := matched[0]
	aliasSet := map[string]struct{}{}
	for _, item := range matched[1:] {
		if item.Topic == canonical.Topic {
			continue
		}
		aliasSet[item.Topic] = struct{}{}
	}
	canonical.Aliases = sortedKeys(aliasSet)
	return canonical
}

//nolint:gocritic
func appendUniqueMatch(matches []wikiTaxonomyMatch, candidate wikiTaxonomyMatch) []wikiTaxonomyMatch {
	for i := range matches {
		if matches[i].Topic == candidate.Topic {
			if candidate.Confidence > matches[i].Confidence {
				matches[i] = candidate
			}
			return matches
		}
	}
	return append(matches, candidate)
}

func wikiTopicFromText(value string) (wikiTaxonomyTopic, bool) {
	normalized := normalizeTaxonomyText(value)
	for _, topic := range wikiTaxonomyTopics {
		for _, label := range topic.Labels {
			if normalized == normalizeTaxonomyText(label) {
				return topic, true
			}
		}
	}
	for _, topic := range wikiTaxonomyTopics {
		for _, label := range topic.Labels {
			needle := normalizeTaxonomyText(label)
			if strings.Contains(normalized, needle) {
				return topic, true
			}
		}
	}
	return wikiTaxonomyTopic{}, false
}

func normalizeTaxonomyText(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "Category:")
	trimmed = strings.TrimPrefix(trimmed, "category:")
	trimmed = strings.ReplaceAll(trimmed, "_", " ")
	trimmed = strings.ReplaceAll(trimmed, "-", " ")
	trimmed = strings.ToLower(trimmed)
	b := strings.Builder{}
	prevSpace := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if prevSpace {
			continue
		}
		b.WriteByte(' ')
		prevSpace = true
	}
	return strings.TrimSpace(b.String())
}

func wikiTopicIndex(group, topic string) int {
	for idx, item := range wikiTaxonomyTopics {
		if item.Group == group && item.Topic == topic {
			return idx
		}
	}
	return len(wikiTaxonomyTopics) + 1
}

//nolint:gocyclo,gocritic
func cleanWikiWikitext(raw string) (string, []string) {
	if strings.TrimSpace(raw) == "" {
		return "", []string{"source_wikitext_empty"}
	}

	content := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	warnings := make([]string, 0, 2)
	out := make([]string, 0, len(lines))
	templateSkipped := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if upper == "__TOC__" || upper == "__NOTOC__" {
			continue
		}
		if strings.HasPrefix(trimmed, "[[Category:") {
			continue
		}
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
			if !templateSkipped {
				warnings = append(warnings, "template_line_removed")
				templateSkipped = true
			}
			continue
		}

		if matches := wikiHeadingPattern.FindStringSubmatch(line); len(matches) == 4 {
			if len(matches[1]) != len(matches[3]) {
				out = append(out, strings.TrimRight(line, " \t"))
				continue
			}
			level := len(matches[1])
			if level < 1 {
				level = 1
			}
			if level > 6 {
				level = 6
			}
			title := strings.TrimSpace(matches[2])
			out = append(out, strings.Repeat("#", level)+" "+title)
			continue
		}

		converted := wikiInternalLinkPattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := wikiInternalLinkPattern.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			target := strings.TrimSpace(parts[1])
			if strings.HasPrefix(strings.ToLower(target), "category:") {
				return ""
			}
			label := target
			if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
				label = strings.TrimSpace(parts[2])
			}
			return "[" + label + "](" + canonicalWikiURL(target) + ")"
		})

		out = append(out, strings.TrimRight(converted, " \t"))
	}

	collapsed := collapseBlankLines(out)
	body := strings.TrimSpace(strings.Join(collapsed, "\n"))
	if body == "" {
		warnings = append(warnings, "cleaned_markdown_empty")
	}
	return body, warnings
}

func collapseBlankLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
			result = append(result, "")
			continue
		}
		prevBlank = false
		result = append(result, line)
	}
	return result
}

func plainTextFallback(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return trimmed
}

//nolint:gocritic
func wikiMarkdownRelPath(page wikiPageRecord, match wikiTaxonomyMatch) string {
	nsDir := fmt.Sprintf("ns%d", page.Namespace)
	fileName := sanitizeFileSlug(page.Title) + ".md"
	if match.Group == "uncategorized" {
		return filepath.Join("cleaned", "markdown", nsDir, "uncategorized", fileName)
	}
	return filepath.Join("cleaned", "markdown", nsDir, match.Group, match.Topic, fileName)
}

func sanitizeFileSlug(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "untitled"
	}
	b := strings.Builder{}
	prevUnderscore := false
	for _, r := range trimmed {
		isASCIILetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if isASCIILetter || isDigit {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if r == '-' || r == '_' {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if prevUnderscore {
			continue
		}
		b.WriteByte('_')
		prevUnderscore = true
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "untitled"
	}
	return slug
}

//nolint:gocritic
func buildWikiMarkdownDocument(page wikiPageRecord, revision wikiRevisionRecord, match wikiTaxonomyMatch, body string) string {
	b := strings.Builder{}
	b.WriteString("---\n")
	b.WriteString("page_id: " + strconv.Itoa(page.PageID) + "\n")
	b.WriteString("title: " + yamlQuote(page.Title) + "\n")
	b.WriteString("namespace: " + strconv.Itoa(page.Namespace) + "\n")
	b.WriteString("revision_id: " + strconv.Itoa(revision.RevisionID) + "\n")
	b.WriteString("revision_ts: " + yamlQuote(revision.Timestamp) + "\n")
	b.WriteString("source_url: " + yamlQuote(sourceURLForPage(page)) + "\n")
	b.WriteString("source_oldid_url: " + yamlQuote(oldIDURLForPage(page.Title, revision.RevisionID)) + "\n")
	b.WriteString("content_sha1: " + yamlQuote(revision.SHA1) + "\n")
	b.WriteString("domain_group: " + yamlQuote(match.Group) + "\n")
	b.WriteString("domain_topic: " + yamlQuote(match.Topic) + "\n")
	b.WriteString("landing_path: " + yamlQuote(match.Landing) + "\n")
	b.WriteString("taxonomy_source: " + yamlQuote(match.Source) + "\n")
	b.WriteString("taxonomy_confidence: " + strconv.FormatFloat(match.Confidence, 'f', 3, 64) + "\n")
	b.WriteString("cleaner_version: " + yamlQuote(wikiCleanerVersion) + "\n")
	if len(match.Aliases) > 0 {
		b.WriteString("topic_aliases:\n")
		for _, alias := range match.Aliases {
			b.WriteString("  - " + yamlQuote(alias) + "\n")
		}
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n")
	return b.String()
}

//nolint:gocritic
func sourceURLForPage(page wikiPageRecord) string {
	if strings.TrimSpace(page.CanonicalURL) != "" {
		return page.CanonicalURL
	}
	return canonicalWikiURL(page.Title)
}

func oldIDURLForPage(title string, revisionID int) string {
	normalizedTitle := strings.ReplaceAll(strings.TrimSpace(title), " ", "_")
	return fmt.Sprintf("%s/index.php?title=%s&oldid=%d", wikiSourceBaseURL, url.QueryEscape(normalizedTitle), revisionID)
}

func canonicalWikiURL(title string) string {
	normalizedTitle := strings.ReplaceAll(strings.TrimSpace(title), " ", "_")
	return wikiSourceBaseURL + "/wiki/" + url.PathEscape(normalizedTitle)
}

func yamlQuote(value string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "\"\""
	}
	return string(raw)
}

func markdownBody(raw []byte) string {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	parts := strings.SplitN(text, "\n---\n", 2)
	if len(parts) != 2 {
		return text
	}
	return parts[1]
}

func hasWikiSectionMarkers(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if wikiHeadingPattern.MatchString(line) {
			return true
		}
	}
	return false
}

func normalizeLangFilter(raw []string) map[string]struct{} {
	set := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func matchesLangFilter(lang string, filter map[string]struct{}) bool {
	if len(filter) == 0 {
		return true
	}
	normalizedLang := strings.TrimSpace(strings.ToLower(lang))
	if normalizedLang == "" {
		_, ok := filter["default"]
		return ok
	}
	_, ok := filter[normalizedLang]
	return ok
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
