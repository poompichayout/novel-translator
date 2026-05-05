# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose

EN→TH novel translation pipeline. Operator pastes source + existing translation chapters into Postgres; an alignment job emits sentence pairs to `train.jsonl` for fine-tuning / RAG; a translation worker (planned) calls Claude Sonnet 4.6 with 4-channel RAG context. EN↔TH is the locked Phase 1 scope — `source_lang` / `target_lang` columns exist but only `en`/`th` are exercised.

`docs/phase-1-spec.md` is the locked scope and **supersedes** the crawler section of `docs/solution-architecture.md`. The architecture doc still describes Part 2 (Translator Engine, RAG, schema additions) accurately.

## Stack

Polyglot, two runtimes glued by a shared Postgres database:

- **Go** (`services/ingestion`) — CLI binary. Owns the DB schema and is the planned home for the paste-to-DB HTTP server, translation worker, and review UI. Currently a thin shell.
- **Python** (`services/alignment/aligner.py`) — separate process that reads paired chapters out of Postgres and writes aligned sentence pairs back. Run manually.
- **Postgres + pgvector** (`docker/init.sql`, `migrations/`) — single source of truth.

Crawler/scraper code is intentionally absent — Phase 1 dropped automated scraping. Source acquisition is manual paste.

## Common commands

All from repo root unless noted.

```bash
make up            # docker compose up -d (db + ingestion containers)
make down
make db-logs
make build         # builds Go binary to bin/ingestion-service
make serve         # build + run `serve` (paste-to-DB HTTP server on :8080; needs SERVER_PASSWORD env or config.yaml server.password)
make translate     # build + run `translate` (translation worker — STUB, exits 2)
make clean         # rm -rf bin/

# Direct Go invocation (after make build)
CONFIG_PATH=./config.yaml ./bin/ingestion-service serve
CONFIG_PATH=./config.yaml ./bin/ingestion-service translate

# Alignment (Python). Requires GEMINI_API_KEY env or it greps config.yaml for the key.
cd services/alignment && python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
DB_URL=postgres://translator:password123@localhost:5432/novel_translator .venv/bin/python aligner.py

# Tests
cd services/ingestion && go test ./...                                # unit tests (cleaner, server)
cd services/ingestion && TEST_DB_URL=postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable go test ./internal/repository/...  # integration; skipped when TEST_DB_URL unset
```

`make build` no longer creates a `.venv` or installs Chromium — those steps were tied to the dropped scraper. The alignment venv is bootstrapped manually as shown above.

The DB schema is loaded by Postgres on first container start via `docker/init.sql` + `migrations/001_init_schema.up.sql` mounted into `/docker-entrypoint-initdb.d/`. Migration `002_add_novel_mappings` is **not** auto-applied — apply it manually (`psql ... -f migrations/002_add_novel_mappings.up.sql`) or wipe the `db-data` volume after wiring it into compose. `aligner.py` will fail without it because it joins on `novel_mappings`.

## Configuration

`config.yaml` is gitignored and contains real API keys. Only three sections are parsed by `internal/config/config.go`:

- `database.url`
- `embedding.gemini_api_key`
- `llm.anthropic_api_key`

Each is also overridable by env var (`DB_URL`, `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`). If `config.yaml` is missing, `main.go` falls back to defaults plus `DB_URL` from env so the binary can boot in containers without a mounted config.

When you add new config (Typhoon/RunPod, novel routing flags, etc.), extend `config.go` and the env override block together — the previous scraper-era pattern of YAML-only keys read by sidecar processes no longer exists.

## Data flow (current)

1. Operator manually inserts a `novels` row and `chapters` rows (the paste-to-DB UI is the planned `serve` command — today this means SQL or a future endpoint).
2. `novel_mappings` (migration 002) links a source-lang `novel_id` to a target-lang `novel_id`. Required for alignment.
3. `aligner.py` runs as a one-shot:
   - Picks the next unaligned chapter pair (`LIMIT 1` + `WHERE NOT EXISTS translation_pairs`).
   - Tokenizes with NLTK (EN) and PyThaiNLP `crfcut` (TH).
   - Embeds with Gemini `text-embedding-004` (768-dim).
   - **Forward-only sliding-window alignment**: window `[current_th_idx-1, current_th_idx+5]`, threshold `0.6`. Alignment cannot jump backwards.
   - Inserts pairs into `translation_pairs` and **appends** to `train.jsonl` in the alignment service's working dir.
   - Loop externally to drain the backlog (one chapter pair per run).
4. The translation worker described in `docs/phase-1-spec.md` (Sonnet 4.6 + prompt cache + batch API + 4-channel RAG) is **not implemented yet** — the `translate` command is a stub.

## Architecture notes

- **Both CLI commands are stubs.** `serve` and `translate` log "not implemented yet (Phase 1, Week 1/3)" and exit 2. The repository is in early Phase 1 — repo + domain types + DB are wired, the actual app surface isn't.
- **No scraper code.** `services/ingestion/internal/scraper/` and `scripts/` are intentionally gone. `domain.ScrapeJob` and the `scrape_jobs` table remain as historical artifacts; nothing writes to them.
- **`docker-compose.yml` builds `db` + `ingestion` only.** No alignment service container — run `aligner.py` from a host venv.
- **Status enum** lives in Postgres as `process_status` ENUM (`pending|in_progress|completed|failed`) and in Go as `domain.ProcessStatus`. Keep them in lockstep.
- **`entities.embedding` is `vector(768)`** — the `domain.Entity` comment says LaBSE, but `aligner.py` uses Gemini `text-embedding-004` which is also 768-dim, so the column is compatible. Switching embedding model means re-checking dimension.
- **Uniqueness constraints to respect on upsert:**
  - `novels`: `source_url`
  - `chapters`: `(novel_id, chapter_number)` — two different source URLs for the same chapter number on the same novel will collide and overwrite.
  - `entities`: `(novel_id, name_en)`
- **`internal/cleaner`** has the HTML-strip + ZWNJ/ZWJ/ZWSP + TIS-620 pipeline that the planned paste-to-DB tool is meant to reuse. `NeedsTIS620Decoding` is currently a stub returning `false` — wire it up before claiming TIS-620 support.
- **Repo writes only.** `PostgresRepo` exposes `UpsertNovel`, `UpsertChapter`, `UpsertEntity`. There are no read/list methods yet — adding them is part of building `serve`/`translate`.

## Phase 1 build order (from `docs/phase-1-spec.md`)

| Week | Deliverable |
|---|---|
| 1 | Paste-to-DB tool: Go HTTP server (chi + HTMX) + chapter upsert + cleaning pipeline |
| 2 | Glossary editor; migration 003 (entity_relationships, pronouns, formality) |
| 3 | Translation worker: Sonnet adapter + prompt cache + batch API; pgvector HNSW + tsvector indexes |
| 4 | 4-channel RAG context builder; first end-to-end translation; align + train.jsonl export |
| 5 | Review UI: side-by-side viewer + edit/flag/approve flows |
| 6 | Feedback loop: glossary update on flag; reviewer_feedback table |

When picking up new work, check the spec for which week's deliverables it belongs to before extending APIs or schema.
