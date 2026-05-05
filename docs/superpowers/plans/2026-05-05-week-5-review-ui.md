# Week 5 — Review UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reviewer can open a chapter side-by-side EN/TH, edit Thai sentences, flag issues per sentence, mark approved.

**Architecture:** Server-rendered HTMX page from the existing Go server. Each sentence row is its own HTMX target so edits patch a single `translation_pairs` row without full-page reloads. Hotkeys for approve/next-sentence to cut reviewer click cost.

**Tech Stack:** Go (chi + html/template), HTMX, Tailwind CDN, optional Alpine.js for hotkey wiring.

**Source spec:** `docs/phase-1-spec.md` Component 3 (Review UI) and Week 5 build-order row.

---

## Scope

- Migration `006_add_review_columns.up.sql`:
  - `ALTER TABLE translation_pairs ADD COLUMN edited_th TEXT, reviewer_id INT, reviewed_at TIMESTAMPTZ, review_status VARCHAR(20) DEFAULT 'pending';`
  - `CREATE TABLE reviewer_feedback (id, pair_id, issue, note, created_at)` with `review_issue` ENUM.
  - `CREATE TABLE reviewers (id, name, email UNIQUE, created_at)`.
- Routes:
  - `GET /review` — chapter list (filter by novel + status).
  - `GET /review/chapter/{id}` — side-by-side editor.
  - `PATCH /api/pairs/{id}` — save edited Thai.
  - `POST /api/pairs/{id}/feedback` — flag issue.
  - `POST /api/pairs/{id}/approve` — mark validated.
- Templates: `review_list.html`, `review_chapter.html`, `review_pair_row.html` (HTMX swap target).
- Hotkeys: `j` next, `k` prev, `a` approve, `e` edit, `f` flag.
- Reviewer auth: extend existing basic-auth — first request prompts for reviewer name → seeded into `reviewers` table.

## Out of Scope

- Automatic re-review of prior pairs after a flag (Week 6 — feedback loop).
- Multi-reviewer concurrency / conflict UI.
- Bulk-approve API (operator can call SQL if needed).

## Acceptance

- Reviewer reviews 10 chapters, edits at least 5 sentences, flags at least 3 with different issue types, approves all 10.
- All edits/flags visible in `translation_pairs.edited_th` and `reviewer_feedback`.
- No full-page reloads during sentence-level review.
