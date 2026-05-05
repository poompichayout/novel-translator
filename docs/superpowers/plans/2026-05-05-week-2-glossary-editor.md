# Week 2 — Glossary Editor + Migration 003 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Operator can manually create/edit entities (characters, places, terms, items) per novel, including pronouns, formality registers, and inter-entity relationships.

**Architecture:** Extend Postgres schema with `entity_relationships`, `entity_pronouns`, `entity_formality` tables (migration 003). Build HTMX UI in the same Go server from Week 1 to manage entities.

**Tech Stack:** Postgres + pgvector, Go (chi + html/template + HTMX + Tailwind CDN).

**Source spec:** `docs/phase-1-spec.md` (Component 3 schema additions for relationships; Week 2 row in build order).

---

## Scope

- Migration `003_add_entity_extras.up.sql` / `.down.sql`:
  - `entity_relationships(id, novel_id, src_entity_id, dst_entity_id, relation_type, notes)`
  - `entity_pronouns(id, entity_id, pronoun_th, register, context_notes)`
  - `entity_formality(id, src_entity_id, dst_entity_id, register, address_term_th)`
- Add `domain.EntityRelationship`, `domain.EntityPronoun`, `domain.EntityFormality` types.
- Extend `repository/postgres.go` with CRUD for the three new tables and `ListEntities(novelID)`.
- New endpoints:
  - `GET /entities?novel_id=X` (HTMX page)
  - `POST /api/entities`, `PUT /api/entities/{id}`, `DELETE /api/entities/{id}`
  - `POST /api/entity-relationships`, `DELETE /api/entity-relationships/{id}`
  - `POST /api/entity-pronouns`, `DELETE /api/entity-pronouns/{id}`
  - `POST /api/entity-formality`, `DELETE /api/entity-formality/{id}`
- HTMX partials: entity row, relationship row, pronoun row, formality row.

## Out of Scope

- Embedding generation for entities (worker handles embedding in Week 3).
- Auto-extraction of entities from chapters (deferred to feedback loop / Week 6).

## Acceptance

- Given a novel, operator can add a character, attach two pronouns and three relationships, and refresh page to see them.
- Migration up/down both clean.
- Repo CRUD covered by integration tests against local Postgres.

## Notes

Update `docker-compose.yml` to mount migration 003 alongside 001/002 (or commit to running migrations manually — pick one before Week 2 starts and document in `CLAUDE.md`).
