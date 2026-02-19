package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"walkscape-helper/internal/output"
	"walkscape-helper/internal/storage"

	"github.com/spf13/cobra"
)

const (
	wikiSourceBaseURL    = "https://wiki.walkscape.app"
	defaultWikiAPIBase   = "https://wiki.walkscape.app/api.php"
	defaultRequestRetry  = 3
	manifestStatusFailed = "failed"
	privateFileMode      = 0o600
)

var defaultWikiFocusCategories = []string{
	"Core Mechanics",
	"Skills",
	"Activities",
	"Recipes",
	"Achievements",
	"Attributes",
	"Job Boards",
	"Keywords",
	"Abilities",
	"Rumors",
	"Tips",
	"Equipment",
	"Materials",
	"Consumables",
	"Collectibles",
	"Chests",
	"Pet Eggs",
	"Cosmetics",
}

type wikiFetchClient struct {
	httpClient *http.Client
	apiBaseURL string
	userAgent  string
	maxLag     int
	retries    int
	minDelay   time.Duration
	lastReqAt  time.Time
}

type wikiAllPagesResponse struct {
	Continue map[string]string `json:"continue"`
	Query    struct {
		AllPages []struct {
			PageID int    `json:"pageid"`
			Title  string `json:"title"`
		} `json:"allpages"`
	} `json:"query"`
}

type wikiPagesResponse struct {
	Query struct {
		Pages map[string]*wikiAPIPage `json:"pages"`
	} `json:"query"`
}

type wikiRecentChangesResponse struct {
	Continue map[string]string `json:"continue"`
	Query    struct {
		RecentChanges []*wikiRecentChange `json:"recentchanges"`
	} `json:"query"`
}

type wikiRecentChange struct {
	RCID      int    `json:"rcid"`
	Type      string `json:"type"`
	Namespace int    `json:"ns"`
	Title     string `json:"title"`
	PageID    int    `json:"pageid"`
	Revid     int    `json:"revid"`
	OldRevid  int    `json:"old_revid"`
	Timestamp string `json:"timestamp"`
	Comment   string `json:"comment"`
	LogType   string `json:"logtype"`
	LogAction string `json:"logaction"`
}

type wikiAPIPage struct {
	PageID     int              `json:"pageid"`
	Namespace  int              `json:"ns"`
	Title      string           `json:"title"`
	Canonical  string           `json:"fullurl"`
	Redirect   string           `json:"redirect,omitempty"`
	Revisions  []wikiAPIRev     `json:"revisions"`
	Categories []wikiAPITitle   `json:"categories"`
	Templates  []wikiAPITitle   `json:"templates"`
	Links      []wikiAPITitle   `json:"links"`
	LangLinks  []wikiAPILangRef `json:"langlinks"`
}

type wikiAPIRev struct {
	RevisionID int    `json:"revid"`
	ParentID   int    `json:"parentid"`
	Timestamp  string `json:"timestamp"`
	Sha1       string `json:"sha1"`
	Size       int    `json:"size"`
	Comment    string `json:"comment"`
	Slots      struct {
		Main struct {
			ContentModel string `json:"contentmodel"`
			Content      string `json:"*"`
		} `json:"main"`
	} `json:"slots"`
}

type wikiAPITitle struct {
	Title string `json:"title"`
}

type wikiAPILangRef struct {
	Lang  string `json:"lang"`
	Title string `json:"*"`
}

type wikiPageRecord struct {
	PageID             int    `json:"page_id"`
	Namespace          int    `json:"namespace"`
	Title              string `json:"title"`
	CanonicalURL       string `json:"canonical_url,omitempty"`
	IsRedirect         bool   `json:"is_redirect"`
	LatestRevisionID   int    `json:"latest_revision_id,omitempty"`
	LatestRevisionSHA1 string `json:"latest_revision_sha1,omitempty"`
	LatestRevisionTS   string `json:"latest_revision_ts,omitempty"`
	LangCode           string `json:"lang_code,omitempty"`
	SourceSnapshotID   string `json:"source_snapshot_id"`
}

type wikiRevisionRecord struct {
	RevisionID       int    `json:"revision_id"`
	PageID           int    `json:"page_id"`
	ParentRevisionID int    `json:"parent_revision_id,omitempty"`
	Timestamp        string `json:"timestamp"`
	SHA1             string `json:"sha1,omitempty"`
	Size             int    `json:"size,omitempty"`
	ContentModel     string `json:"content_model,omitempty"`
	Wikitext         string `json:"wikitext,omitempty"`
	Comment          string `json:"comment,omitempty"`
}

type wikiEdgeRecord struct {
	PageID   int    `json:"page_id"`
	EdgeType string `json:"edge_type"`
	Target   string `json:"target"`
}

type wikiScrapeCounts struct {
	Pages      int `json:"wiki_pages"`
	Revisions  int `json:"wiki_revisions"`
	Edges      int `json:"wiki_page_edges"`
	Files      int `json:"wiki_files"`
	Tombstones int `json:"wiki_tombstones"`
}

func newWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "Manage wiki scraping workflows",
		Long:  "Scrape and maintain local WalkScape wiki snapshots.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newWikiScrapeCmd())
	cmd.AddCommand(newWikiStatusCmd())

	return cmd
}

func newWikiScrapeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scrape",
		Short: "Run wiki scrape operations",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newWikiScrapeFullCmd())
	cmd.AddCommand(newWikiScrapeUpdateCmd())

	return cmd
}

//nolint:gocyclo
func newWikiStatusCmd() *cobra.Command {
	var outDir string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show wiki sync status",
		Long:  "Show local wiki snapshot and incremental sync cursor status.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()

			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki status", appErr)
			}
			defer db.Close()

			lastSnapshotID, err := getAppMetaValue(execCtx, db, "wiki_last_full_snapshot_id")
			if err != nil {
				appErr := output.NewError("storage_error", "failed to read wiki metadata", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki status", appErr)
			}
			lastSyncTS, err := getAppMetaValue(execCtx, db, "wiki_last_sync_ts")
			if err != nil {
				appErr := output.NewError("storage_error", "failed to read wiki metadata", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki status", appErr)
			}
			recentChangesCursor, err := getAppMetaValue(execCtx, db, "wiki_recentchanges_cursor")
			if err != nil {
				appErr := output.NewError("storage_error", "failed to read wiki metadata", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki status", appErr)
			}

			incrementalReady := lastSyncTS != "" || recentChangesCursor != ""
			recommendedNext := "wiki scrape full"
			if incrementalReady {
				recommendedNext = "wiki scrape update"
			}

			result := map[string]any{
				"last_full_snapshot_id":    lastSnapshotID,
				"last_sync_ts":             lastSyncTS,
				"recentchanges_cursor":     recentChangesCursor,
				"wiki_data_root":           outDir,
				"last_snapshot_manifest":   nil,
				"last_snapshot_found":      false,
				"incremental_ready":        incrementalReady,
				"recommended_next_command": recommendedNext,
			}

			if lastSnapshotID != "" {
				manifestPath := filepath.Join(outDir, lastSnapshotID, "manifest.json")
				manifestRaw, readErr := os.ReadFile(manifestPath)
				if readErr == nil {
					var manifest map[string]any
					if unmarshalErr := json.Unmarshal(manifestRaw, &manifest); unmarshalErr == nil {
						result["last_snapshot_manifest"] = manifest
						result["last_snapshot_found"] = true
					}
				}
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), result, commandMeta("wiki status"))
			}

			if lastSnapshotID == "" {
				if err := output.WriteHuman(cmd.OutOrStdout(), "No wiki snapshot recorded yet"); err != nil {
					return err
				}
			} else {
				if err := output.WriteHuman(cmd.OutOrStdout(), "Last full snapshot: %s", lastSnapshotID); err != nil {
					return err
				}
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Last sync timestamp: %s", valueOrUnset(lastSyncTS)); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Recentchanges cursor: %s", valueOrUnset(recentChangesCursor)); err != nil {
				return err
			}
			if ready, readyOK := result["incremental_ready"].(bool); readyOK && ready {
				return output.WriteHuman(cmd.OutOrStdout(), "Incremental update is ready")
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Run full scrape first to initialize incremental cursor")
		},
	}

	cmd.Flags().StringVar(&outDir, "out", filepath.Join(".", "data", "wiki"), "wiki snapshot root directory")

	return cmd
}

//nolint:gocyclo
func newWikiScrapeUpdateCmd() *cobra.Command {
	var namespaces []string
	var outDir string
	var apiBaseURL string
	var rateLimitRPS int
	var since string
	var maxChanges int
	var includeCategories []string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Fetch incremental wiki changes",
		Long:  "Fetch changes since the last cursor using MediaWiki recentchanges and store a new snapshot.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()

			nsIDs, err := parseNamespaceIDs(namespaces)
			if err != nil {
				return writeErr(cmd, ctx.JSON, "wiki scrape update", err)
			}
			if rateLimitRPS <= 0 {
				return writeErr(cmd, ctx.JSON, "wiki scrape update", output.NewError("validation_error", "rate-limit-rps must be greater than zero", map[string]any{"field": "rate-limit-rps"}))
			}
			if maxChanges <= 0 {
				return writeErr(cmd, ctx.JSON, "wiki scrape update", output.NewError("validation_error", "max-changes must be greater than zero", map[string]any{"field": "max-changes"}))
			}

			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}
			defer db.Close()

			startTS, startRCID, err := resolveUpdateCursor(execCtx, db, since)
			if err != nil {
				return writeErr(cmd, ctx.JSON, "wiki scrape update", err)
			}

			snapshotID := time.Now().UTC().Format("20060102T150405Z")
			snapshotDir, err := createWikiSnapshotSkeleton(outDir, snapshotID, false)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to create wiki snapshot directory", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}

			manifest := map[string]any{
				"snapshot_id":     snapshotID,
				"source_base_url": wikiSourceBaseURL,
				"api_base_url":    apiBaseURL,
				"namespaces":      nsIDs,
				"created_at":      time.Now().UTC().Format(time.RFC3339),
				"mode":            "incremental_update",
				"base_cursor": map[string]any{
					"timestamp": startTS,
					"rcid":      startRCID,
				},
				"status":             "running",
				"scrape_in_progress": true,
				"counts":             map[string]any{"wiki_pages": 0, "wiki_revisions": 0, "wiki_page_edges": 0, "wiki_files": 0, "wiki_tombstones": 0},
			}
			if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
				appErr := output.NewError("storage_error", "failed to write snapshot manifest", map[string]any{"reason": manifestErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}

			client := &wikiFetchClient{
				httpClient: &http.Client{Timeout: 60 * time.Second},
				apiBaseURL: apiBaseURL,
				userAgent:  fmt.Sprintf("walkscape-helper/%s", appVersion()),
				maxLag:     5,
				retries:    defaultRequestRetry,
				minDelay:   time.Second / time.Duration(rateLimitRPS),
			}

			categoryFilter := buildCategoryFilter(includeCategories)
			counts, eventCount, changedPageCount, cursorTS, cursorRCID, updateErr := runWikiIncrementalUpdate(execCtx, client, snapshotID, snapshotDir, nsIDs, startTS, startRCID, maxChanges, categoryFilter)
			if updateErr != nil {
				manifest["status"] = manifestStatusFailed
				manifest["scrape_in_progress"] = false
				manifest["error"] = updateErr.Error()
				if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
					manifest["manifest_write_error"] = manifestErr.Error()
				}
				appErr := output.NewError("storage_error", "wiki incremental update failed", map[string]any{"reason": updateErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}

			if integrityErr := verifyLatestRevisionIntegrity(snapshotDir); integrityErr != nil {
				manifest["status"] = manifestStatusFailed
				manifest["scrape_in_progress"] = false
				manifest["error"] = integrityErr.Error()
				if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
					manifest["manifest_write_error"] = manifestErr.Error()
				}
				appErr := output.NewError("storage_error", "wiki update integrity check failed", map[string]any{"reason": integrityErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}

			if eventCount > 0 {
				if syncErr := setAppMetaValue(execCtx, db, "wiki_last_sync_ts", cursorTS); syncErr != nil {
					return writeErr(cmd, ctx.JSON, "wiki scrape update", output.NewError("storage_error", "failed updating wiki sync timestamp", map[string]any{"reason": syncErr.Error()}))
				}
				if cursorErr := setAppMetaValue(execCtx, db, "wiki_recentchanges_cursor", formatCursor(cursorTS, cursorRCID)); cursorErr != nil {
					return writeErr(cmd, ctx.JSON, "wiki scrape update", output.NewError("storage_error", "failed updating wiki cursor", map[string]any{"reason": cursorErr.Error()}))
				}
			}

			manifest["status"] = "completed"
			manifest["scrape_in_progress"] = false
			manifest["completed_at"] = time.Now().UTC().Format(time.RFC3339)
			manifest["counts"] = map[string]any{
				"wiki_pages":      counts.Pages,
				"wiki_revisions":  counts.Revisions,
				"wiki_page_edges": counts.Edges,
				"wiki_files":      counts.Files,
				"wiki_tombstones": counts.Tombstones,
			}
			manifest["update"] = map[string]any{
				"event_count":        eventCount,
				"changed_page_count": changedPageCount,
				"cursor_timestamp":   cursorTS,
				"cursor_rcid":        cursorRCID,
			}
			if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
				appErr := output.NewError("storage_error", "failed to finalize snapshot manifest", map[string]any{"reason": manifestErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}

			filesForChecksum, err := collectSnapshotFiles(snapshotDir)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to enumerate snapshot files", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}
			if err := writeChecksums(snapshotDir, filesForChecksum); err != nil {
				appErr := output.NewError("storage_error", "failed to write checksums", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape update", appErr)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{
					"snapshot_id":        snapshotID,
					"snapshot_dir":       snapshotDir,
					"event_count":        eventCount,
					"changed_page_count": changedPageCount,
					"counts": map[string]any{
						"wiki_pages":      counts.Pages,
						"wiki_revisions":  counts.Revisions,
						"wiki_page_edges": counts.Edges,
						"wiki_tombstones": counts.Tombstones,
					},
					"cursor": map[string]any{"timestamp": cursorTS, "rcid": cursorRCID},
				}, commandMeta("wiki scrape update"))
			}

			if err := output.WriteHuman(cmd.OutOrStdout(), "Completed wiki update snapshot %s", snapshotID); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Events: %d Changed pages: %d", eventCount, changedPageCount); err != nil {
				return err
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Cursor: %s (%d)", cursorTS, cursorRCID)
		},
	}

	cmd.Flags().StringSliceVar(&namespaces, "namespaces", []string{"0"}, "namespace ids to include")
	cmd.Flags().StringVar(&outDir, "out", filepath.Join(".", "data", "wiki"), "output directory root for snapshots")
	cmd.Flags().StringVar(&apiBaseURL, "api-base-url", defaultWikiAPIBase, "MediaWiki API endpoint")
	cmd.Flags().IntVar(&rateLimitRPS, "rate-limit-rps", 2, "maximum requests per second")
	cmd.Flags().StringVar(&since, "since", "", "override cursor start timestamp (RFC3339)")
	cmd.Flags().IntVar(&maxChanges, "max-changes", 5000, "maximum recentchanges events to process in one run")
	cmd.Flags().StringSliceVar(&includeCategories, "include-categories", defaultWikiFocusCategories, "category names to include in update snapshots")

	return cmd
}

//nolint:gocyclo
func newWikiScrapeFullCmd() *cobra.Command {
	var namespaces []string
	var outDir string
	var downloadMedia bool
	var includeCategories []string
	var apiBaseURL string
	var rateLimitRPS int

	cmd := &cobra.Command{
		Use:   "full",
		Short: "Create local wiki snapshot",
		Long:  "Run a phase-1 full scrape and persist local raw and normalized wiki records.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := contextFromCommand(cmd)
			execCtx := cmd.Context()

			nsIDs, err := parseNamespaceIDs(namespaces)
			if err != nil {
				return writeErr(cmd, ctx.JSON, "wiki scrape full", err)
			}
			if rateLimitRPS <= 0 {
				return writeErr(cmd, ctx.JSON, "wiki scrape full", output.NewError("validation_error", "rate-limit-rps must be greater than zero", map[string]any{"field": "rate-limit-rps"}))
			}

			db, err := storage.Open(execCtx, ctx.DBPath)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to open sqlite database", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}
			defer db.Close()

			snapshotID := time.Now().UTC().Format("20060102T150405Z")
			snapshotDir, err := createWikiSnapshotSkeleton(outDir, snapshotID, downloadMedia)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to create wiki snapshot directory", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			manifest := map[string]any{
				"snapshot_id":        snapshotID,
				"source_base_url":    wikiSourceBaseURL,
				"api_base_url":       apiBaseURL,
				"namespaces":         nsIDs,
				"include_categories": normalizeNonEmptyValues(includeCategories),
				"download_media":     downloadMedia,
				"created_at":         time.Now().UTC().Format(time.RFC3339),
				"request_config": map[string]any{
					"rate_limit_rps": rateLimitRPS,
					"burst":          1,
					"maxlag":         5,
					"retries":        defaultRequestRetry,
				},
				"schema_versions":    map[string]any{"raw_schema": 1, "normalized_schema": 1, "vector_schema": 1},
				"counts":             map[string]any{"wiki_pages": 0, "wiki_revisions": 0, "wiki_page_edges": 0, "wiki_files": 0, "wiki_tombstones": 0},
				"tool":               map[string]any{"version": appVersion(), "commit": commit},
				"phase":              "phase_1",
				"status":             "running",
				"scrape_in_progress": true,
			}
			if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
				appErr := output.NewError("storage_error", "failed to write snapshot manifest", map[string]any{"reason": manifestErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			client := &wikiFetchClient{
				httpClient: &http.Client{Timeout: 60 * time.Second},
				apiBaseURL: apiBaseURL,
				userAgent:  fmt.Sprintf("walkscape-helper/%s", appVersion()),
				maxLag:     5,
				retries:    defaultRequestRetry,
				minDelay:   time.Second / time.Duration(rateLimitRPS),
			}

			categoryFilter := buildCategoryFilter(includeCategories)
			counts, scrapeErr := runFullWikiScrape(execCtx, client, snapshotID, snapshotDir, nsIDs, categoryFilter)
			if scrapeErr != nil {
				manifest["status"] = manifestStatusFailed
				manifest["scrape_in_progress"] = false
				manifest["error"] = scrapeErr.Error()
				if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
					manifest["manifest_write_error"] = manifestErr.Error()
				}
				appErr := output.NewError("storage_error", "wiki scrape failed", map[string]any{"reason": scrapeErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			if integrityErr := verifyLatestRevisionIntegrity(snapshotDir); integrityErr != nil {
				manifest["status"] = manifestStatusFailed
				manifest["scrape_in_progress"] = false
				manifest["error"] = integrityErr.Error()
				if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
					manifest["manifest_write_error"] = manifestErr.Error()
				}
				appErr := output.NewError("storage_error", "wiki scrape integrity check failed", map[string]any{"reason": integrityErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			manifest["status"] = "completed"
			manifest["scrape_in_progress"] = false
			manifest["completed_at"] = time.Now().UTC().Format(time.RFC3339)
			manifest["counts"] = map[string]any{
				"wiki_pages":      counts.Pages,
				"wiki_revisions":  counts.Revisions,
				"wiki_page_edges": counts.Edges,
				"wiki_files":      counts.Files,
				"wiki_tombstones": counts.Tombstones,
			}
			if manifestErr := writeManifest(snapshotDir, manifest); manifestErr != nil {
				appErr := output.NewError("storage_error", "failed to finalize snapshot manifest", map[string]any{"reason": manifestErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			nowCursorTS := time.Now().UTC().Format(time.RFC3339)
			if snapshotErr := setWikiLastSnapshot(execCtx, db, snapshotID); snapshotErr != nil {
				appErr := output.NewError("storage_error", "failed to update wiki snapshot metadata", map[string]any{"reason": snapshotErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}
			if syncErr := setAppMetaValue(execCtx, db, "wiki_last_sync_ts", nowCursorTS); syncErr != nil {
				appErr := output.NewError("storage_error", "failed to update wiki sync metadata", map[string]any{"reason": syncErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}
			if cursorErr := setAppMetaValue(execCtx, db, "wiki_recentchanges_cursor", formatCursor(nowCursorTS, 0)); cursorErr != nil {
				appErr := output.NewError("storage_error", "failed to update wiki sync cursor", map[string]any{"reason": cursorErr.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			filesForChecksum, err := collectSnapshotFiles(snapshotDir)
			if err != nil {
				appErr := output.NewError("storage_error", "failed to enumerate snapshot files", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}
			if err := writeChecksums(snapshotDir, filesForChecksum); err != nil {
				appErr := output.NewError("storage_error", "failed to write checksums", map[string]any{"reason": err.Error()})
				return writeErr(cmd, ctx.JSON, "wiki scrape full", appErr)
			}

			if ctx.JSON {
				return output.WriteJSONSuccess(cmd.OutOrStdout(), map[string]any{
					"snapshot_id":    snapshotID,
					"snapshot_dir":   snapshotDir,
					"namespaces":     nsIDs,
					"download_media": downloadMedia,
					"counts": map[string]any{
						"wiki_pages":      counts.Pages,
						"wiki_revisions":  counts.Revisions,
						"wiki_page_edges": counts.Edges,
					},
				}, commandMeta("wiki scrape full"))
			}

			if err := output.WriteHuman(cmd.OutOrStdout(), "Completed wiki snapshot %s", snapshotID); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Snapshot directory: %s", snapshotDir); err != nil {
				return err
			}
			if err := output.WriteHuman(cmd.OutOrStdout(), "Pages: %d Revisions: %d Edges: %d", counts.Pages, counts.Revisions, counts.Edges); err != nil {
				return err
			}
			return output.WriteHuman(cmd.OutOrStdout(), "Namespaces: %v", nsIDs)
		},
	}

	cmd.Flags().StringSliceVar(&namespaces, "namespaces", []string{"0"}, "namespace ids to include")
	cmd.Flags().StringVar(&outDir, "out", filepath.Join(".", "data", "wiki"), "output directory root for snapshots")
	cmd.Flags().BoolVar(&downloadMedia, "download-media", false, "download media binaries")
	cmd.Flags().StringSliceVar(&includeCategories, "include-categories", defaultWikiFocusCategories, "category names to prioritize for phase-1 scope")
	cmd.Flags().StringVar(&apiBaseURL, "api-base-url", defaultWikiAPIBase, "MediaWiki API endpoint")
	cmd.Flags().IntVar(&rateLimitRPS, "rate-limit-rps", 2, "maximum requests per second")

	return cmd
}

//nolint:gocyclo
func runFullWikiScrape(execCtx context.Context, client *wikiFetchClient, snapshotID, snapshotDir string, nsIDs []int, categoryFilter map[string]struct{}) (wikiScrapeCounts, error) {
	counts := wikiScrapeCounts{}
	rawDir := filepath.Join(snapshotDir, "raw", "api", snapshotID)
	pagesPath := filepath.Join(snapshotDir, "normalized", "wiki_pages.ndjson")
	revsPath := filepath.Join(snapshotDir, "normalized", "wiki_revisions.ndjson")
	edgesPath := filepath.Join(snapshotDir, "normalized", "wiki_page_edges.ndjson")

	batchIndex := 1
	for _, ns := range nsIDs {
		allPagesBatch := 1
		allTitles := make([]string, 0, 512)
		cont := map[string]string{}
		for {
			params := map[string]string{
				"action":      "query",
				"list":        "allpages",
				"apnamespace": strconv.Itoa(ns),
				"aplimit":     "500",
			}
			for k, v := range cont {
				params[k] = v
			}

			raw, payload, err := client.fetchAllPages(execCtx, params)
			if err != nil {
				return counts, err
			}
			rawName := fmt.Sprintf("allpages_ns%d_%04d.json", ns, allPagesBatch)
			if err := os.WriteFile(filepath.Join(rawDir, rawName), raw, privateFileMode); err != nil {
				return counts, err
			}

			for _, page := range payload.Query.AllPages {
				allTitles = append(allTitles, page.Title)
			}

			if len(payload.Continue) == 0 {
				break
			}
			cont = stripContinueKey(payload.Continue)
			allPagesBatch++
		}

		for start := 0; start < len(allTitles); start += 50 {
			end := start + 50
			if end > len(allTitles) {
				end = len(allTitles)
			}
			titleBatch := allTitles[start:end]
			raw, payload, err := client.fetchPageBatch(execCtx, titleBatch)
			if err != nil {
				return counts, err
			}

			rawName := fmt.Sprintf("page_batch_%04d.json", batchIndex)
			if err := os.WriteFile(filepath.Join(rawDir, rawName), raw, privateFileMode); err != nil {
				return counts, err
			}
			batchIndex++

			for _, apiPage := range payload.Query.Pages {
				if apiPage == nil {
					continue
				}
				if apiPage.PageID <= 0 {
					continue
				}
				if !matchesCategoryFilter(apiPage, categoryFilter) {
					continue
				}

				pageRecord := wikiPageRecord{
					PageID:           apiPage.PageID,
					Namespace:        apiPage.Namespace,
					Title:            apiPage.Title,
					CanonicalURL:     apiPage.Canonical,
					IsRedirect:       apiPage.Redirect != "",
					SourceSnapshotID: snapshotID,
				}

				if len(apiPage.Revisions) > 0 {
					rev := apiPage.Revisions[0]
					pageRecord.LatestRevisionID = rev.RevisionID
					pageRecord.LatestRevisionSHA1 = rev.Sha1
					pageRecord.LatestRevisionTS = rev.Timestamp

					revision := wikiRevisionRecord{
						RevisionID:       rev.RevisionID,
						PageID:           apiPage.PageID,
						ParentRevisionID: rev.ParentID,
						Timestamp:        rev.Timestamp,
						SHA1:             rev.Sha1,
						Size:             rev.Size,
						ContentModel:     rev.Slots.Main.ContentModel,
						Wikitext:         rev.Slots.Main.Content,
						Comment:          rev.Comment,
					}
					if err := appendNDJSON(revsPath, revision); err != nil {
						return counts, err
					}
					counts.Revisions++
				}

				if err := appendNDJSON(pagesPath, pageRecord); err != nil {
					return counts, err
				}
				counts.Pages++

				edgeRecords := buildWikiEdges(apiPage)
				for _, edge := range edgeRecords {
					if err := appendNDJSON(edgesPath, edge); err != nil {
						return counts, err
					}
					counts.Edges++
				}
			}
		}
	}

	return counts, nil
}

//nolint:gocyclo,gocritic
func runWikiIncrementalUpdate(
	execCtx context.Context,
	client *wikiFetchClient,
	snapshotID, snapshotDir string,
	nsIDs []int,
	startTS string,
	startRCID int,
	maxChanges int,
	categoryFilter map[string]struct{},
) (wikiScrapeCounts, int, int, string, int, error) {
	counts := wikiScrapeCounts{}
	rawDir := filepath.Join(snapshotDir, "raw", "api", snapshotID)
	pagesPath := filepath.Join(snapshotDir, "normalized", "wiki_pages.ndjson")
	revsPath := filepath.Join(snapshotDir, "normalized", "wiki_revisions.ndjson")
	edgesPath := filepath.Join(snapshotDir, "normalized", "wiki_page_edges.ndjson")
	tombstonesPath := filepath.Join(snapshotDir, "normalized", "wiki_tombstones.ndjson")

	nsSet := make(map[int]struct{}, len(nsIDs))
	for _, ns := range nsIDs {
		nsSet[ns] = struct{}{}
	}

	cont := map[string]string{}
	recentBatch := 1
	eventCount := 0
	changedTitles := make(map[string]struct{})
	cursorTS := startTS
	cursorRCID := startRCID

	for {
		params := map[string]string{
			"action":  "query",
			"list":    "recentchanges",
			"rclimit": "500",
			"rcdir":   "newer",
			"rcstart": startTS,
			"rctype":  "edit|new|log|categorize|external",
			"rcprop":  "title|ids|sizes|flags|timestamp|loginfo|sha1|comment|tags|user",
		}
		for k, v := range cont {
			params[k] = v
		}

		raw, payload, err := client.fetchRecentChanges(execCtx, params)
		if err != nil {
			return counts, 0, 0, "", 0, err
		}
		rawName := fmt.Sprintf("recentchanges_%04d.json", recentBatch)
		if err := os.WriteFile(filepath.Join(rawDir, rawName), raw, privateFileMode); err != nil {
			return counts, 0, 0, "", 0, err
		}
		recentBatch++

		for _, change := range payload.Query.RecentChanges {
			if change == nil {
				continue
			}
			if change.Timestamp == startTS && change.RCID <= startRCID {
				continue
			}
			eventCount++
			if eventCount > maxChanges {
				return counts, 0, 0, "", 0, fmt.Errorf("max changes limit reached (%d)", maxChanges)
			}

			if isNewerCursor(change.Timestamp, change.RCID, cursorTS, cursorRCID) {
				cursorTS = change.Timestamp
				cursorRCID = change.RCID
			}

			if _, ok := nsSet[change.Namespace]; !ok {
				continue
			}

			if change.Title != "" {
				changedTitles[change.Title] = struct{}{}
			}

			if change.Type == "log" && (change.LogType == "delete" || change.LogType == "move" || change.LogType == "restore") {
				tombstone := map[string]any{
					"page_id":    change.PageID,
					"title":      change.Title,
					"event_type": change.LogType,
					"event_ts":   change.Timestamp,
					"reason":     change.Comment,
				}
				if err := appendNDJSON(tombstonesPath, tombstone); err != nil {
					return counts, 0, 0, "", 0, err
				}
				counts.Tombstones++
			}
		}

		if len(payload.Continue) == 0 {
			break
		}
		cont = stripContinueKey(payload.Continue)
	}

	titles := make([]string, 0, len(changedTitles))
	for title := range changedTitles {
		titles = append(titles, title)
	}
	sort.Strings(titles)
	seenPageIDs := make(map[int]struct{}, len(titles))

	batchIndex := 1
	for start := 0; start < len(titles); start += 50 {
		end := start + 50
		if end > len(titles) {
			end = len(titles)
		}

		raw, payload, err := client.fetchPageBatch(execCtx, titles[start:end])
		if err != nil {
			return counts, 0, 0, "", 0, err
		}
		rawName := fmt.Sprintf("update_page_batch_%04d.json", batchIndex)
		if err := os.WriteFile(filepath.Join(rawDir, rawName), raw, privateFileMode); err != nil {
			return counts, 0, 0, "", 0, err
		}
		batchIndex++

		for _, apiPage := range payload.Query.Pages {
			if apiPage == nil {
				continue
			}
			if apiPage.PageID <= 0 {
				continue
			}
			if !matchesCategoryFilter(apiPage, categoryFilter) {
				continue
			}
			if _, exists := seenPageIDs[apiPage.PageID]; exists {
				continue
			}
			seenPageIDs[apiPage.PageID] = struct{}{}

			pageRecord := wikiPageRecord{
				PageID:           apiPage.PageID,
				Namespace:        apiPage.Namespace,
				Title:            apiPage.Title,
				CanonicalURL:     apiPage.Canonical,
				IsRedirect:       apiPage.Redirect != "",
				SourceSnapshotID: snapshotID,
			}

			if len(apiPage.Revisions) > 0 {
				rev := apiPage.Revisions[0]
				pageRecord.LatestRevisionID = rev.RevisionID
				pageRecord.LatestRevisionSHA1 = rev.Sha1
				pageRecord.LatestRevisionTS = rev.Timestamp

				revision := wikiRevisionRecord{
					RevisionID:       rev.RevisionID,
					PageID:           apiPage.PageID,
					ParentRevisionID: rev.ParentID,
					Timestamp:        rev.Timestamp,
					SHA1:             rev.Sha1,
					Size:             rev.Size,
					ContentModel:     rev.Slots.Main.ContentModel,
					Wikitext:         rev.Slots.Main.Content,
					Comment:          rev.Comment,
				}
				if err := appendNDJSON(revsPath, revision); err != nil {
					return counts, 0, 0, "", 0, err
				}
				counts.Revisions++
			}

			if err := appendNDJSON(pagesPath, pageRecord); err != nil {
				return counts, 0, 0, "", 0, err
			}
			counts.Pages++

			edgeRecords := buildWikiEdges(apiPage)
			for _, edge := range edgeRecords {
				if err := appendNDJSON(edgesPath, edge); err != nil {
					return counts, 0, 0, "", 0, err
				}
				counts.Edges++
			}
		}
	}

	changedPageCount := len(seenPageIDs)
	if eventCount == 0 {
		return counts, eventCount, changedPageCount, startTS, startRCID, nil
	}

	return counts, eventCount, changedPageCount, cursorTS, cursorRCID, nil
}

func buildWikiEdges(apiPage *wikiAPIPage) []wikiEdgeRecord {
	edges := make([]wikiEdgeRecord, 0, len(apiPage.Categories)+len(apiPage.Templates)+len(apiPage.Links)+len(apiPage.LangLinks))
	for _, category := range apiPage.Categories {
		edges = append(edges, wikiEdgeRecord{PageID: apiPage.PageID, EdgeType: "category", Target: category.Title})
	}
	for _, template := range apiPage.Templates {
		edges = append(edges, wikiEdgeRecord{PageID: apiPage.PageID, EdgeType: "template", Target: template.Title})
	}
	for _, link := range apiPage.Links {
		edges = append(edges, wikiEdgeRecord{PageID: apiPage.PageID, EdgeType: "link", Target: link.Title})
	}
	for _, langLink := range apiPage.LangLinks {
		target := langLink.Title
		if langLink.Lang != "" {
			target = langLink.Lang + ":" + langLink.Title
		}
		edges = append(edges, wikiEdgeRecord{PageID: apiPage.PageID, EdgeType: "langlink", Target: target})
	}
	return edges
}

func (c *wikiFetchClient) fetchAllPages(ctx context.Context, params map[string]string) ([]byte, wikiAllPagesResponse, error) {
	raw, err := c.doRequest(ctx, params)
	if err != nil {
		return nil, wikiAllPagesResponse{}, err
	}
	var payload wikiAllPagesResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, wikiAllPagesResponse{}, err
	}
	if err := checkAPIError(raw); err != nil {
		return nil, wikiAllPagesResponse{}, err
	}
	return raw, payload, nil
}

func (c *wikiFetchClient) fetchPageBatch(ctx context.Context, titles []string) ([]byte, wikiPagesResponse, error) {
	params := map[string]string{
		"action":  "query",
		"prop":    "info|revisions|categories|templates|links|langlinks",
		"inprop":  "url",
		"rvprop":  "ids|timestamp|sha1|size|contentmodel|comment|content",
		"rvslots": "main",
		"cllimit": "max",
		"tllimit": "max",
		"pllimit": "max",
		"lllimit": "max",
		"titles":  strings.Join(titles, "|"),
	}

	raw, err := c.doRequest(ctx, params)
	if err != nil {
		return nil, wikiPagesResponse{}, err
	}
	var payload wikiPagesResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, wikiPagesResponse{}, err
	}
	if err := checkAPIError(raw); err != nil {
		return nil, wikiPagesResponse{}, err
	}
	return raw, payload, nil
}

func (c *wikiFetchClient) fetchRecentChanges(ctx context.Context, params map[string]string) ([]byte, wikiRecentChangesResponse, error) {
	raw, err := c.doRequest(ctx, params)
	if err != nil {
		return nil, wikiRecentChangesResponse{}, err
	}
	var payload wikiRecentChangesResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, wikiRecentChangesResponse{}, err
	}
	if err := checkAPIError(raw); err != nil {
		return nil, wikiRecentChangesResponse{}, err
	}
	return raw, payload, nil
}

func (c *wikiFetchClient) doRequest(ctx context.Context, params map[string]string) ([]byte, error) {
	if err := c.waitRateLimit(ctx); err != nil {
		return nil, err
	}

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	q.Set("format", "json")
	q.Set("maxlag", strconv.Itoa(c.maxLag))

	requestURL := c.apiBaseURL + "?" + q.Encode()
	var lastErr error
	for attempt := 1; attempt <= c.retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
			} else if resp.StatusCode >= 400 {
				return nil, fmt.Errorf("wiki api request failed with status %d", resp.StatusCode)
			} else {
				c.lastReqAt = time.Now()
				return body, nil
			}
		}

		if attempt < c.retries {
			delay := time.Duration(200*attempt) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("wiki api request failed")
	}
	return nil, lastErr
}

func (c *wikiFetchClient) waitRateLimit(ctx context.Context) error {
	if c.lastReqAt.IsZero() || c.minDelay <= 0 {
		return nil
	}
	elapsed := time.Since(c.lastReqAt)
	if elapsed >= c.minDelay {
		return nil
	}
	waitFor := c.minDelay - elapsed
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitFor):
		return nil
	}
}

func checkAPIError(raw []byte) error {
	var envelope struct {
		Error *struct {
			Code string `json:"code"`
			Info string `json:"info"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	if envelope.Error == nil {
		return nil
	}
	if envelope.Error.Code == "accessdenied" {
		return fmt.Errorf("wiki api access denied: %s", envelope.Error.Info)
	}
	return fmt.Errorf("wiki api error (%s): %s", envelope.Error.Code, envelope.Error.Info)
}

func setWikiLastSnapshot(ctx context.Context, db *sql.DB, snapshotID string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO app_meta (key, value) VALUES ('wiki_last_full_snapshot_id', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, snapshotID)
	return err
}

func setAppMetaValue(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO app_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func getAppMetaValue(ctx context.Context, db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_meta WHERE key = ? LIMIT 1`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func resolveUpdateCursor(ctx context.Context, db *sql.DB, sinceOverride string) (cursorTS string, cursorRCID int, err error) {
	if strings.TrimSpace(sinceOverride) != "" {
		ts := strings.TrimSpace(sinceOverride)
		if _, parseErr := time.Parse(time.RFC3339, ts); parseErr != nil {
			return "", 0, output.NewError("validation_error", "since must be RFC3339 timestamp", map[string]any{"field": "since", "reason": parseErr.Error()})
		}
		return ts, 0, nil
	}

	cursor, err := getAppMetaValue(ctx, db, "wiki_recentchanges_cursor")
	if err != nil {
		return "", 0, err
	}
	if strings.TrimSpace(cursor) != "" {
		ts, rcid, parseErr := parseCursor(cursor)
		if parseErr != nil {
			return "", 0, output.NewError("validation_error", "invalid stored wiki cursor", map[string]any{"reason": parseErr.Error()})
		}
		return ts, rcid, nil
	}

	lastSync, err := getAppMetaValue(ctx, db, "wiki_last_sync_ts")
	if err != nil {
		return "", 0, err
	}
	if strings.TrimSpace(lastSync) != "" {
		if _, parseErr := time.Parse(time.RFC3339, lastSync); parseErr != nil {
			return "", 0, output.NewError("validation_error", "invalid stored wiki sync timestamp", map[string]any{"reason": parseErr.Error()})
		}
		return lastSync, 0, nil
	}

	return "", 0, output.NewError("not_found", "wiki incremental cursor not initialized; run wiki scrape full first", nil)
}

func parseCursor(raw string) (ts string, rcid int, err error) {
	parts := strings.Split(strings.TrimSpace(raw), "|")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("cursor must be <timestamp>|<rcid>")
	}
	ts = parts[0]
	if _, parseErr := time.Parse(time.RFC3339, ts); parseErr != nil {
		return "", 0, parseErr
	}
	rcid, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, err
	}
	if rcid < 0 {
		return "", 0, fmt.Errorf("rcid must be non-negative")
	}
	return ts, rcid, nil
}

func formatCursor(ts string, rcid int) string {
	return fmt.Sprintf("%s|%d", ts, rcid)
}

func valueOrUnset(v string) string {
	if strings.TrimSpace(v) == "" {
		return "<unset>"
	}
	return v
}

func isNewerCursor(ts string, rcid int, currentTS string, currentRCID int) bool {
	if ts == "" {
		return false
	}
	if currentTS == "" {
		return true
	}
	if ts > currentTS {
		return true
	}
	if ts == currentTS && rcid > currentRCID {
		return true
	}
	return false
}

func appendNDJSON(path string, value any) error {
	buf, err := json.Marshal(value)
	if err != nil {
		return err
	}
	buf = append(buf, '\n')

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, privateFileMode)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(buf)
	return err
}

func verifyLatestRevisionIntegrity(snapshotDir string) error {
	pagesPath := filepath.Join(snapshotDir, "normalized", "wiki_pages.ndjson")
	revsPath := filepath.Join(snapshotDir, "normalized", "wiki_revisions.ndjson")

	revisionIDs := map[int]struct{}{}
	revData, err := os.ReadFile(revsPath)
	if err != nil {
		return err
	}
	for _, line := range bytes.Split(revData, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec wikiRevisionRecord
		if unmarshalErr := json.Unmarshal(line, &rec); unmarshalErr != nil {
			return unmarshalErr
		}
		revisionIDs[rec.RevisionID] = struct{}{}
	}

	pageData, err := os.ReadFile(pagesPath)
	if err != nil {
		return err
	}
	for _, line := range bytes.Split(pageData, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec wikiPageRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return err
		}
		if rec.LatestRevisionID == 0 {
			continue
		}
		if _, exists := revisionIDs[rec.LatestRevisionID]; !exists {
			return fmt.Errorf("latest revision %d missing for page %d", rec.LatestRevisionID, rec.PageID)
		}
	}

	return nil
}

func writeManifest(snapshotDir string, manifest map[string]any) error {
	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, append(manifestRaw, '\n'), privateFileMode)
}

func collectSnapshotFiles(snapshotDir string) ([]string, error) {
	files := make([]string, 0, 64)
	err := filepath.WalkDir(snapshotDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(snapshotDir, path)
		if err != nil {
			return err
		}
		if rel == "checksums.sha256" {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func parseNamespaceIDs(raw []string) ([]int, error) {
	if len(raw) == 0 {
		return nil, output.NewError("validation_error", "at least one namespace is required", map[string]any{"field": "namespaces"})
	}

	seen := make(map[int]struct{}, len(raw))
	nsIDs := make([]int, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return nil, output.NewError("validation_error", "namespace id must not be empty", map[string]any{"field": "namespaces"})
		}

		id, err := strconv.Atoi(trimmed)
		if err != nil || id < 0 {
			return nil, output.NewError("validation_error", "namespace id must be a non-negative integer", map[string]any{"field": "namespaces", "value": item})
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		nsIDs = append(nsIDs, id)
	}

	sort.Ints(nsIDs)
	return nsIDs, nil
}

func stripContinueKey(raw map[string]string) map[string]string {
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		if k == "continue" {
			continue
		}
		result[k] = v
	}
	return result
}

func createWikiSnapshotSkeleton(outDir, snapshotID string, downloadMedia bool) (string, error) {
	root := filepath.Join(outDir, snapshotID)
	if err := os.MkdirAll(filepath.Join(root, "raw", "api", snapshotID), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(root, "normalized"), 0o755); err != nil {
		return "", err
	}
	if downloadMedia {
		if err := os.MkdirAll(filepath.Join(root, "media"), 0o755); err != nil {
			return "", err
		}
	}

	normalizedFiles := []string{
		filepath.Join("normalized", "wiki_pages.ndjson"),
		filepath.Join("normalized", "wiki_revisions.ndjson"),
		filepath.Join("normalized", "wiki_page_edges.ndjson"),
		filepath.Join("normalized", "wiki_files.ndjson"),
		filepath.Join("normalized", "wiki_tombstones.ndjson"),
	}

	for _, relPath := range normalizedFiles {
		if err := os.WriteFile(filepath.Join(root, relPath), nil, privateFileMode); err != nil {
			return "", err
		}
	}

	return root, nil
}

func writeChecksums(snapshotDir string, relPaths []string) error {
	var b strings.Builder
	for _, relPath := range relPaths {
		content, err := os.ReadFile(filepath.Join(snapshotDir, relPath))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		if _, err := b.WriteString(hex.EncodeToString(sum[:]) + "  " + relPath + "\n"); err != nil {
			return err
		}
	}

	return os.WriteFile(filepath.Join(snapshotDir, "checksums.sha256"), []byte(b.String()), privateFileMode)
}

func normalizeNonEmptyValues(raw []string) []string {
	normalized := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func buildCategoryFilter(raw []string) map[string]struct{} {
	filter := make(map[string]struct{})
	for _, name := range raw {
		normalized := normalizeCategoryName(name)
		if normalized == "" {
			continue
		}
		filter[normalized] = struct{}{}
	}
	return filter
}

func matchesCategoryFilter(page *wikiAPIPage, filter map[string]struct{}) bool {
	if len(filter) == 0 {
		return true
	}
	for _, category := range page.Categories {
		normalized := normalizeCategoryName(category.Title)
		if normalized == "" {
			continue
		}
		if _, ok := filter[normalized]; ok {
			return true
		}
	}
	return false
}

func normalizeCategoryName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "category:") {
		trimmed = strings.TrimSpace(trimmed[len("Category:"):])
	}
	trimmed = strings.ReplaceAll(trimmed, "_", " ")
	trimmed = strings.ToLower(strings.TrimSpace(trimmed))
	return trimmed
}
