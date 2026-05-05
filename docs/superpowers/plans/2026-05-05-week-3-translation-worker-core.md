# Week 3 — Translation Worker Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the `translate` CLI command end-to-end with Claude Sonnet 4.6 (prompt cache + batch API), pgvector HNSW index, and tsvector full-text index — RAG context still stubbed.

**Architecture:** Go worker polls `chapters` rows where `scrape_status='completed'` and no row exists in `translations`. Worker calls Anthropic SDK with prompt cache enabled and submits batches via the batch API. Stores raw output in a new `translations` table.

**Tech Stack:** Go, Anthropic Go SDK (`anthropics/anthropic-sdk-go`), pgvector HNSW, Postgres `tsvector` + GIN index.

**Source spec:** `docs/phase-1-spec.md` Component 2 (Translation Worker) and Week 3 build-order row.

---

## Scope

- Migration `004_add_translations_and_indexes.up.sql`:
  - `translations(id, chapter_id, model, input_tokens, cached_input_tokens, output_tokens, raw_output, finish_reason, batch_id, status, created_at)`
  - `chapters_fulltext_idx` (GIN on `to_tsvector('english', cleaned_content)`)
  - `entities_embedding_hnsw_idx` (HNSW on `entities.embedding vector_cosine_ops`)
- New Go package `internal/llm`:
  - `Adapter` interface: `Translate(ctx, req) (Response, error)`, `BatchSubmit(ctx, reqs) (batchID, error)`, `BatchPoll(ctx, batchID) (status, []Response, error)`.
  - `SonnetAdapter` implementation with prompt cache (`cache_control: {type: "ephemeral"}` on system + glossary blocks).
  - Stub `TyphoonAdapter` returning `ErrNotImplemented` (RunPod adapter built in Phase 2 or as separate plan).
- New CLI subcommand `translate` (replace stub):
  - Loop: fetch next N pending chapters → build trivial prompt (no RAG yet) → batch submit → poll → write `translations` row.
- Config additions (`config.yaml` + `internal/config`):
  - `llm.primary: claude-sonnet-4-6`
  - `llm.use_batch: true`
  - `llm.daily_spend_cap_usd: 10`

## Out of Scope

- 4-channel RAG context builder (Week 4).
- Typhoon RunPod adapter beyond stub.
- Reviewer feedback (Week 6).

## Acceptance

- `make translate` consumes one pending chapter and writes a `translations` row with non-empty `raw_output`.
- Prompt cache hit visible via `cached_input_tokens > 0` on the second chapter of the same novel.
- HNSW + tsvector indexes present (`\d+ entities`, `\d+ chapters` in psql).

## Notes

- Use Anthropic batch API only — half cost, no realtime requirement Phase 1 (R2 in spec).
- Daily spend cap enforced in worker before submitting next batch (read sum from `translations.input_tokens * price + output_tokens * price` for today, abort if over cap).
- Use the `claude-api` superpower skill when wiring the SDK.
