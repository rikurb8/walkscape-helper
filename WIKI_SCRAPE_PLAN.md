# WalkScape Wiki Scrape Plan

## Goals

- Scrape the full WalkScape wiki content from `https://wiki.walkscape.app` in a way that is reproducible and incremental.
- Store the scraped data in a portable snapshot format that can be zipped and shared.
- Transform snapshots into chunked embeddings for vector DB indexing (Qdrant first, provider-agnostic design).

## Recon Notes (MediaWiki specifics)

- Site appears to be MediaWiki `1.44.0` (`api.php?action=query&meta=siteinfo`).
- Public API endpoints are available at `https://wiki.walkscape.app/api.php` and `https://wiki.walkscape.app/rest.php`.
- Current stats snapshot shows roughly:
  - `pages`: 17024
  - `articles`: 1771
  - `images`: 2293
- Custom namespaces exist (`Guide`, `Gear`, `Staff`, `Data`, `Module`, `Translations`).
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
- `manifest.json` enables validation and exact rebuild.

### `manifest.json` minimum fields

- snapshot id (`YYYYMMDDTHHMMSSZ`)
- source base URL
- namespaces scraped
- request config (rate, retries, maxlag)
- schema versions (`raw_schema`, `normalized_schema`, `vector_schema`)
- item counts per file
- tool version / git commit

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

## Recommended Implementation Order

1. Add SQLite schema + `wiki scrape full` for latest revisions only.
2. Add export to portable snapshot zip with manifest/checksums.
3. Add chunking + embedding + vector upsert pipeline.
4. Add `wiki scrape update` incremental sync with cursor/tombstones.
5. Add reconciliation mode (`wiki scrape reconcile`) for periodic consistency audits.
