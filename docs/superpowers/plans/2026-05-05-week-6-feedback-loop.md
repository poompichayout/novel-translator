# Week 6 — Feedback Loop + train.jsonl Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reviewer flags drive automatic glossary updates and re-review of affected pairs; approved pairs flow to `train.jsonl` export.

**Architecture:** A dispatch step (Go) triggered after each `POST /api/pairs/{id}/feedback` enqueues a job per issue type. Jobs are inline goroutines with row-level locking on `entities` / `translation_pairs` (no external queue Phase 1). Export is a `make export` Make target that streams approved pairs to `train.jsonl`.

**Tech Stack:** Go, Postgres advisory locks, existing schema from Weeks 2 + 5.

**Source spec:** `docs/phase-1-spec.md` Component 3 Feedback Loop section + Week 6 build-order row.

---

## Scope

- Job dispatch on flag:
  - `pronoun_wrong` → detect entity in source sentence → upsert `entity_pronouns` row → mark all prior `translation_pairs` referencing same entity as `review_status='needs_recheck'`.
  - `name_inconsistent` → enqueue entity dedup: search `entities` by fuzzy name → present merge candidates via a new admin route (`GET /admin/dedup`).
  - `register_off`, `formality_wrong` → upsert `entity_formality` row.
  - `mistranslation`, `missing_text`, `extra_text`, `relation_wrong`, `other` → store feedback only (no auto-action).
- New `make export` target:
  - Streams `translation_pairs` where `review_status='approved'` to `train.jsonl` (one JSON per line: `{en, th, novel_id, chapter_id, pair_id}`).
  - Permissive license header line at top? — confirm with operator. Locked decision: permissive output (A-9). Header optional.
- Daily spend log dashboard (text-only) at `GET /admin/spend` showing today's `translations` cost.

## Out of Scope

- Auto-merge of duplicate entities (operator confirms via dedup UI).
- Real queue (RabbitMQ / NATS / etc.) — Phase 2 if needed.
- Retraining / fine-tuning loop — out of Phase 1 entirely.

## Acceptance

- All 5 Phase 1 success criteria from `docs/phase-1-spec.md` pass:
  - [ ] 1 novel × 10 chapters fully pipelined: paste → translate → review → approved
  - [ ] Pronoun/honorific consistency across chapters (manual validation)
  - [ ] Reviewer flags drive glossary updates without code change
  - [ ] `train.jsonl` exports cleanly with reviewer-approved pairs
  - [ ] Total cost ≤ $80/mo at 10 novels/mo

## Notes

- After Week 6, propose Phase 2 doc covering: scaling to 100 novels/mo, Typhoon RunPod adapter, multi-reviewer team mode.
