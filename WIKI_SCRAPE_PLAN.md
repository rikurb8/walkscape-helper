# WalkScape Wiki Scrape Plan

## Goals

- Scrape the full WalkScape wiki content from `https://wiki.walkscape.app` in a way that is reproducible and incremental.
- Store the scraped data in a portable snapshot format that can be zipped and shared.
- Produce a cleaned markdown corpus from scraped wiki revisions for human review and downstream chunking.
- Transform snapshots into chunked embeddings for vector DB indexing (Qdrant first, provider-agnostic design).

## Recon Notes (MediaWiki specifics)

- Site appears to be MediaWiki `1.44.0` (`api.php?action=query&meta=siteinfo`).
- Public API endpoints are available at `https://wiki.walkscape.app/api.php` and `https://wiki.walkscape.app/rest.php`.
- Current stats snapshot shows roughly:
  - `pages`: 17024
  - `articles`: 1771
  - `images`: 2293
- Custom namespaces exist (`Guide`, `Gear`, `Staff`, `Data`, `Module`, `Translations`).
- Some pages in `Data` namespace appear access-restricted via API (`accessdenied` when requesting content); treat `106` as optional and continue on permission errors in phase 1.
- API high limits are permission-gated; anonymous clients should assume `500` item batch limits.
- `https://wiki.walkscape.app/robots.txt` currently redirects to the main page, so treat crawl etiquette as explicit policy in scraper config (rate limit + backoff + contact string in User-Agent).

## Scope Definition

### In scope (content for retrieval)

- Main content namespaces:
  - `0` (main)
  - `100` (`Guide`)
  - `102` (`Gear`)
  - `104` (`Staff`)
  - `106` (`Data`) if useful as structured knowledge
  - `828` (`Module`) for parser logic references (optional for retrieval, useful for debugging)
- Metadata for all pages:
  - page id/title/namespace
  - latest revision id + timestamp + sha1 + size
  - categories/templates/links
  - redirects and move history
- File metadata for namespace `6` (`File`) and optional binary media download.

### Out of scope for first pass

- Talk pages, user pages, and moderation logs for retrieval corpus.
- Full historic revision backfill for every page (only latest revision initially).

## Scraping Strategy

### 1) Full seed snapshot

1. Enumerate pages by namespace using `list=allpages` with continuation:
   - `action=query&list=allpages&apnamespace=<ns>&aplimit=max&format=json`
2. For each batch of titles (up to 50 per request for `titles=`):
   - fetch page data with:
     - `prop=info|revisions|categories|templates|links|langlinks`
     - `inprop=url`
     - `rvprop=ids|timestamp|sha1|size|contentmodel|comment|content`
     - `rvslots=main`
3. Persist raw API responses and normalized records.
4. For files (`ns=6`), fetch:
   - `prop=imageinfo`
   - `iiprop=url|size|mime|sha1|timestamp|extmetadata`
5. Optional media binary download:
   - only when `--download-media` is enabled
   - use imageinfo URL, checksum verify against known hash if available.

### 2) Incremental sync

1. Store sync cursor in local DB (`app_meta`) as UTC timestamp + last `rcid`.
2. Poll recent changes:
   - `action=query&list=recentchanges&rcprop=title|ids|sizes|flags|timestamp|loginfo|sha1|comment|tags|user&rctype=edit|new|log|categorize|external`
3. For changed page ids/titles, refetch latest page payload (same `prop=...` as full snapshot).
4. Handle page moves/deletes via `type=log` events (`move`, `delete`, `restore`).
5. Mark removed pages as tombstones in normalized dataset rather than hard delete.

### 3) Reliability and etiquette

- Request pacing: start at `2 req/sec`, burst max `4`, configurable.
- Retries: exponential backoff with jitter for `429`/`5xx`/timeouts.
- Respect `maxlag` parameter (`maxlag=5`) in all requests.
- Set descriptive `User-Agent`: `walkscape-helper/<version> (+repo/contact)`.
- Checkpoint continuation tokens (`apcontinue`, `continue`, `rccontinue`) every N pages.

## Data Model (Normalized)

### Core entities

- `wiki_pages`
  - `page_id` (int)
  - `namespace` (int)
  - `title` (string)
  - `canonical_url` (string)
  - `is_redirect` (bool)
  - `latest_revision_id` (int)
  - `latest_revision_sha1` (string)
  - `latest_revision_ts` (RFC3339)
  - `lang_code` (parsed from suffix like `/en` when applicable)
  - `source_snapshot_id` (string)
- `wiki_revisions`
  - `revision_id` (int)
  - `page_id` (int)
  - `parent_revision_id` (int)
  - `timestamp` (RFC3339)
  - `sha1` (string)
  - `size` (int)
  - `content_model` (string)
  - `wikitext` (string)
  - `comment` (string)
- `wiki_page_edges`
  - `page_id`, `edge_type` (`link|category|template|langlink`), `target`
- `wiki_files`
  - `file_page_id`, `title`, `mime`, `size`, `url`, `sha1`, `timestamp`, `ext_metadata_json`
- `wiki_tombstones`
  - `page_id|title`, `event_type`, `event_ts`, `reason`

### Raw preservation

- Keep raw API payloads exactly as returned for replay/debug:
  - request URL
  - response body
  - HTTP status
  - fetched timestamp

## Portable Snapshot Format (Zip-shareable)

Create a deterministic directory bundle, then zip it.

```text
walkscape-wiki-snapshot/
  manifest.json
  checksums.sha256
  raw/
    api/
      2026-02-18T210000Z/
        allpages_ns0_0001.json
        page_batch_0001.json
  normalized/
    wiki_pages.ndjson
    wiki_revisions.ndjson
    wiki_page_edges.ndjson
    wiki_files.ndjson
    wiki_tombstones.ndjson
  cleaned/
    markdown/
      ns0/
        Skills.md
      ns100/
        Guide_Getting_Started.md
    cleaned_pages.ndjson
    cleaning_report.ndjson
  vector/
    chunks.ndjson
    embeddings.f32.bin   (optional)
    embeddings.index.json
  media/                 (optional; only if --download-media)
    File_WalkscapeBanner.png
```

### Why this format

- NDJSON is easy to stream and language-agnostic.
- Raw + normalized separation keeps provenance and reproducibility.
- A `cleaned/markdown` tree provides readable artifacts and stable text inputs before chunking.
- `manifest.json` enables validation and exact rebuild.

### `manifest.json` minimum fields

- snapshot id (`YYYYMMDDTHHMMSSZ`)
- source base URL
- namespaces scraped
- request config (rate, retries, maxlag)
- schema versions (`raw_schema`, `normalized_schema`, `vector_schema`)
- item counts per file
- cleaned corpus metadata (`cleaned_schema`, pages cleaned, pages skipped, cleaner version)
- tool version / git commit

## Clean Markdown Refinement Plan (next step)

### Objective

- Transform `wiki_revisions.wikitext` into clean, deterministic markdown files suitable for both human consumption and retrieval chunking.
- Keep a strict mapping from cleaned markdown back to source page/revision for provenance.

### Command surface (planned)

- `wsh wiki clean --snapshot <path> [--out <path>] [--namespaces ...] [--lang ...]`
- `wsh wiki clean validate --snapshot <path>`

Both commands should support `--json` and stable error codes.

### Input/output contract

- Input:
  - `normalized/wiki_pages.ndjson`
  - `normalized/wiki_revisions.ndjson` (latest revision content)
  - optional edge files for template/category context
- Output:
  - `cleaned/markdown/<namespace>/<sanitized_title>.md`
  - `cleaned/cleaned_pages.ndjson` with one record per cleaned page
  - `cleaned/cleaning_report.ndjson` with warnings/skips/fallback reasons

### Cleaning rules (v1 deterministic)

1. Expand MediaWiki markup into readable markdown where safe:
   - preserve section hierarchy as `#`/`##` headers
   - convert internal links to markdown links using canonical wiki URLs
   - keep category/template references in metadata, not inline noise
2. Reduce retrieval noise:
   - remove nav/footer boilerplate and edit-only artifacts
   - collapse repeated whitespace and strip empty sections
   - keep short infobox-style key/value data as bullet lists or definition lines
3. Preserve provenance and reproducibility:
   - include frontmatter (or top metadata block) with `page_id`, `title`, `namespace`, `revision_id`, `revision_ts`, `source_url`, `source_oldid_url`, `content_sha1`
   - deterministic file naming from namespace + title slug
   - deterministic cleaning version hash in records

### Cleaned page record schema (`cleaned_pages.ndjson`)

- `page_id`, `title`, `namespace`, `lang_code`
- `revision_id`, `revision_ts`, `content_sha1`
- `markdown_path`
- `clean_status` (`ok|partial|failed|skipped`)
- `warnings` (array)
- `cleaner_version`

### Validation for clean corpus

- Coverage: cleaned pages count matches eligible source pages for selected namespaces.
- Integrity: each cleaned record points to an existing markdown file.
- Determinism: rerun on same snapshot yields identical checksums.
- Quality gates:
  - non-empty markdown body for `ok` pages
  - section header presence for pages that had section markers in source
  - warning budget thresholds (alert if partial/failed ratio exceeds configured limit)

### Failure handling policy

- Do not fail the entire run for single-page parse errors; emit `partial`/`failed` records and continue.
- Fail the command only when systemic issues occur (missing inputs, schema mismatch, output write failures).
- Preserve raw wikitext for all failures so parser improvements can replay deterministically.

## Local Persistence During Scrape

Use SQLite as working state (fits current project architecture):

- Add tables for `wiki_sync_state`, `wiki_pages`, `wiki_revisions`, `wiki_page_edges`, `wiki_files`, `wiki_tombstones`, `wiki_fetch_log`.
- Keep existing `app_meta` and store:
  - `wiki_last_full_snapshot_id`
  - `wiki_recentchanges_cursor`
  - `wiki_last_sync_ts`
- Export command materializes SQLite state into the portable snapshot layout.

## Embedding and Vector DB Plan

### Chunking

1. Convert wikitext to clean retrieval text:
   - remove template noise where possible
   - preserve section headers
   - keep infobox key fields as structured lines
2. Chunk by semantic boundaries first (section-based), then size cap:
   - target ~700-1000 tokens
   - overlap ~80-120 tokens
3. Deterministic chunk id:
   - `chunk_id = sha1(page_id + revision_id + section_path + chunk_index + chunk_text)`

### Chunk metadata (required for citations)

- `chunk_id`
- `page_id`, `title`, `namespace`
- `revision_id`, `revision_ts`
- `section_path`
- `source_url` (`.../wiki/<Title>`)
- `source_oldid_url` (`.../index.php?title=<Title>&oldid=<revid>`)
- `content_sha1`
- `lang_code`

### Embedding job

- Read `vector/chunks.ndjson`.
- Batch by provider token limits.
- Store outputs in `vector/embeddings.f32.bin` + index map file.
- Upsert into vector DB collection named by config (`vectordb_collection`).
- Use `chunk_id` as vector point id for idempotent reindex.

### Vector payload (Qdrant example)

- `id`: `chunk_id`
- `vector`: embedding array
- `payload`:
  - title, namespace, page_id, revision_id
  - section_path
  - source_url, source_oldid_url
  - lang_code
  - content_hash
  - snapshot_id

## End-to-End Commands (planned)

- `wsh wiki status`
- `wsh wiki scrape full [--namespaces ...] [--download-media]`
- `wsh wiki scrape update`
- `wsh wiki export --snapshot-id <id> --out <path.zip>`
- `wsh wiki clean --snapshot <path>`
- `wsh wiki chunk --snapshot <path>`
- `wsh wiki embed --snapshot <path>`
- `wsh wiki index --snapshot <path>`

All commands should support `--json` envelopes and stable error codes following existing CLI conventions.

## Validation and QA

- Completeness checks:
  - compare scraped page counts per namespace against `siteinfo`/`allpages` totals
  - ensure each `wiki_pages.latest_revision_id` exists in `wiki_revisions`
- Integrity checks:
  - verify `checksums.sha256`
  - verify no duplicate `chunk_id`
- Retrieval smoke checks:
  - sample known pages (e.g., Skills, Activities, FAQs) and assert top-k recalls include expected page titles.

## Risks and Mitigations

- Template-heavy pages may be noisy in raw wikitext.
  - Mitigation: maintain both raw wikitext and cleaned text pipeline.
- Page moves/deletions can break stale links.
  - Mitigation: process `recentchanges` logs + tombstones.
- Language variants (`/en`, `/fi`, etc.) can duplicate meaning.
  - Mitigation: explicit `lang_code`, configurable indexing policy (`all`, `en-only`, `primary+en`).
- Large media can bloat snapshots.
  - Mitigation: media download opt-in; default metadata-only for files.

## Phase 1 Assumptions (Local scrape MVP)

This phase is intentionally limited to "get wiki data locally with metadata".

- Primary goal: reproducible local snapshot directory suitable for later chunking/indexing.
- Output target: directory bundle first, zip export can follow as a convenience step.
- Revision scope: latest revision only per page (no full history backfill).
- Namespace default: `0` (main namespace) enabled by default; `100,102,104,106,828` opt-in via flag.
- Initial topical focus is category-driven within namespace `0`, prioritizing:
  - Game mechanics: Core Mechanics, Skills, Activities, Recipes, Achievements, Attributes, Job Boards, Keywords, Abilities, Rumors, Tips
  - Items: Equipment, Materials, Consumables, Collectibles, Chests, Pet Eggs, Cosmetics
- File handling: collect file metadata (`wiki_files`) but do not download binaries unless `--download-media` is set.
- Incremental sync: out of scope for phase 1; full scrape only.
- Embeddings/vector DB: out of scope for phase 1.
- Raw preservation: store raw API request/response payloads for replay/debug.
- Idempotency expectation: re-running creates a new `snapshot_id` directory; no in-place mutation of prior snapshots.
- Reliability baseline: enforce rate limit, retries, `maxlag`, and checkpoint continuation tokens.

### Phase 1 concrete deliverables

1. New CLI command: `wsh wiki scrape full [--namespaces ...] [--out <dir>] [--download-media]`.
2. SQLite working schema for wiki entities + fetch log/sync metadata tables.
3. Deterministic snapshot directory output with:
   - `manifest.json`
   - `checksums.sha256`
   - `raw/api/...` payload files
   - `normalized/*.ndjson` files
4. Validation pass at end of scrape:
   - per-namespace counts
   - latest revision referential integrity
5. JSON-mode compatibility and stable error codes following existing CLI conventions.

## Future Phases (Explicit)

### Phase 2: Clean markdown refinement

- Add `wsh wiki clean --snapshot <path> [--out <path>]`.
- Build deterministic wikitext-to-markdown cleaning pipeline.
- Emit `cleaned/markdown/*` plus `cleaned_pages.ndjson` and `cleaning_report.ndjson`.
- Add cleaning validation command and quality metrics.

### Phase 3: Snapshot packaging and operational polish

- Add `wsh wiki export --snapshot-id <id> --out <path.zip>`.
- Ensure deterministic zip creation and checksum verification.
- Add `wsh wiki status` for local inventory (latest snapshot, counts, last run metadata).

### Phase 4: Chunk pipeline (no embeddings yet)

- Add `wsh wiki chunk --snapshot <path>`.
- Use cleaned markdown as primary chunk input (fallback to raw text when cleaning failed).
- Emit `vector/chunks.ndjson` with deterministic `chunk_id` and required citation metadata.
- Add chunk integrity checks (duplicate chunk ids, missing source refs).

### Phase 5: Embeddings and indexing

- Add `wsh wiki embed --snapshot <path>`.
- Add `wsh wiki index --snapshot <path>`.
- Keep provider-agnostic embedding interface, Qdrant-first adapter.
- Make upserts idempotent via `chunk_id` as point id.

### Phase 6: Incremental sync and reconciliation

- Add `wsh wiki scrape update` using recentchanges cursor + log event handling.
- Persist tombstones for deletes/moves/restores.
- Add `wsh wiki scrape reconcile` for periodic audits and repair workflows.

## Recommended Implementation Order

1. Phase 1 local scrape MVP.
2. Phase 2 clean markdown refinement.
3. Phase 3 snapshot packaging + status visibility.
4. Phase 4 chunk generation.
5. Phase 5 embedding + vector indexing.
6. Phase 6 incremental sync + reconciliation.
