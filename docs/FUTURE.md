# Future Roadmap

High-level areas to implement after phase 1.

## Wiki Ingestion and Sync

- implement local wiki `status/create/update` commands
- fetch MediaWiki changes incrementally using a persisted cursor
- handle page edits, moves, and deletions safely
- add periodic reconciliation/full consistency checks

## Retrieval and Vector Indexing

- add chunking + embedding pipeline for wiki pages
- support configurable embedding model providers
- support configurable vector database providers
- index only changed content to reduce update cost

## Guide Answering

- implement `wsh guide ask <question>`
- build prompt with persona + character context + retrieved wiki context
- return citations/sources with answers
- define fallback behavior when wiki index is empty

## Docker Compose Local Stack

- provide composable local runtime for optional services
- include vector database and optional local model runtime profile
- add health checks and startup docs

## Evaluation and Quality

- add automated evaluation suite after functional CLI is stable
- create regression datasets for representative Walkscape questions
- track answer quality, citation quality, and retrieval quality over time

## UX and Developer Experience

- improve interactive setup flows and command help text
- add import diagnostics and clearer validation errors
- improve JSON output schemas for AI orchestration stability
- provide richer examples and troubleshooting documentation
