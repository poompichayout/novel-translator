# Phase 1 Implementation Spec — Locked Scope

Based on BA answers in `docs/BA-answer.txt` (2026-05-05).

Supersedes crawler section of `docs/solution-architecture.md`. RAG + LLM sections still apply.

---

## Locked Decisions

| Item | Decision | Source |
|---|---|---|
| Source acquisition | **Manual copy-paste via Paste-to-DB tool** | A-1, A-4 |
| Crawler engine | **DROPPED** — no automated scraping | A-1 |
| Distribution | B2B tooling target; private/small-scale first | A-2 |
| Quality target | Replace human TL; Sonnet 4.6 + Typhoon post-edit | A-3 |
| QA | Human reviewer in pipeline; feedback to model | A-5 |
| Hosting | Singapore (AWS ap-southeast-1 or Linode SG) | A-6 |
| Self-host scope | **Infra only** (Postgres, queue, workers, paste tool, review UI on Singapore VPS). **Sonnet = Anthropic API.** Typhoon optional post-edit, **self-hosted on RunPod** when enabled. | A-7 + A-3 (locked 2026-05-05) |
| Languages | EN→TH only Phase 1 | A-8 |
| Output rights | Permissive, anyone can use | A-9 |
| Quality/Cost weight | 75/25 | A-10 |

---

## Architecture v2 — No Crawler

```
┌──────────────────────────────────────────────────────────────┐
│  Operator (you / contributor)                                │
│  Browser → manual copy from source site                      │
└────────────┬─────────────────────────────────────────────────┘
             │
   ┌─────────▼──────────────────┐
   │  Paste-to-DB Tool (Web UI) │  Phase 1 priority build
   │  - Paste raw text/HTML     │
   │  - Auto-detect chapter no. │
   │  - Chapter splitter helper │
   │  - Save → chapters table   │
   └─────────┬──────────────────┘
             │
             ▼
       ┌───────────┐
       │ Postgres  │  scrape_status='completed', skip crawler entirely
       └─────┬─────┘
             │
   ┌─────────▼────────────────────────────────┐
   │  Translation Worker (Go orchestrator)    │
   │  ┌──────────────────────────────────┐    │
   │  │ Context Builder (4-channel RAG)  │    │
   │  │ → Sonnet 4.6 (cache + batch)     │    │
   │  │ → Typhoon 2 70B post-edit pass   │    │
   │  └──────────────────────────────────┘    │
   └─────────┬────────────────────────────────┘
             │
   ┌─────────▼─────────────────┐
   │  Review UI (Web)          │  Phase 1 priority build
   │  - Side-by-side EN/TH     │
   │  - Edit, mark validated   │
   │  - Flag pronoun/relation  │
   │  - Feedback → glossary    │
   └─────────┬─────────────────┘
             │
             ▼
       ┌────────────┐
       │ Postgres   │
       │ - chapters │
       │ - translations           │
       │ - translation_pairs (validated/edited) │
       │ - entities + relationships             │
       │ - reviewer_feedback                    │
       └────────────────────────┘
             │
             ▼
   ┌─────────────────────┐
   │  train.jsonl export │  permissive output, anyone use
   └─────────────────────┘
```

---

## Component 1 — Paste-to-DB Tool

### Goal

Operator pastes raw chapter text. Tool stores cleanly with metadata, ready for translation.

### UI (Web, simple form)

```
┌─────────────────────────────────────────────┐
│  Novel: [dropdown: select existing or NEW]  │
│  If NEW:                                    │
│    Title: [____________________]            │
│    Source URL: [____________________]       │
│    Source Lang: [en]   Target Lang: [th]   │
│                                             │
│  Chapter Number: [____]  (auto-suggest n+1) │
│  Chapter Title:  [____________________]     │
│  Source URL:     [____________________]     │
│                                             │
│  Raw content (paste here):                  │
│  ┌─────────────────────────────────────┐   │
│  │ <textarea, 50 rows>                 │   │
│  │                                     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  [ ] Auto-clean (strip HTML, normalize ws)  │
│  [ ] Auto-split if multiple chapters        │
│                                             │
│       [Save & Next Chapter]  [Save]         │
└─────────────────────────────────────────────┘
```

### Tech Stack

| Layer | Pick | Why |
|---|---|---|
| Backend | Go (extend existing `services/ingestion`) | Reuse repo + domain types |
| API | net/http + chi router | Minimal deps |
| UI | HTMX + Tailwind via CDN | No build step. Operator-only tool, no need for SPA. |
| Auth | Single shared password (basic auth) | Internal tool, low stakes |
| Deploy | Docker container on Singapore VPS | Same compose as DB |

### New Endpoints

```
POST /api/novels                # create novel
GET  /api/novels                # list
POST /api/chapters              # paste chapter
GET  /api/chapters?novel_id=X   # list chapters for novel
GET  /                          # paste UI
GET  /review                    # review UI
```

### Cleaning Pipeline

Reuse existing `internal/cleaner/cleaner.go`:
- Strip HTML tags
- Collapse whitespace
- Strip ZWNJ/ZWJ/ZWSP
- TIS-620 detection (already there)

Add:
- Detect & strip "Chapter X" duplicate header in body
- Detect & strip translator notes (`[T/N: ...]`, `(TL note: ...)`)
- Detect & strip ad/promo lines (`Read at <site>...`)

---

## Component 2 — Translation Worker

Same as architecture v1 doc, locked picks:

| Stage | Pick |
|---|---|
| Primary LLM | Claude Sonnet 4.6 via Anthropic API + prompt cache + batch API (50% off) |
| Post-edit (optional) | Typhoon 2 70B self-hosted on **RunPod** (H100/A100 spot); per-novel opt-in flag |
| Embedding | Gemini text-embedding-004 (existing pgvector(768)) |
| RAG | 4-channel: entity-first SQL + hybrid BM25/vector + recent plot + glossary dict |
| Vector store | pgvector + HNSW (self-hosted Postgres on SG VPS) |
| Compute | Singapore VPS (Linode/Vultr) — self-host all infra except LLM |

Cost per 100-ch novel:
- Sonnet 4.6 cached + batch: **~$4.65**
- Typhoon post-edit (API): **~$0.97**
- Embedding: **~$0.10**
- **Total: ~$5.72/novel**

At 10 novels/month = $57/mo LLM. At 100 novels/month = $572/mo.

### Typhoon (Locked: Optional Post-Edit on RunPod)

Typhoon = optional Phase 1, enabled per-novel via config flag. **Self-host on RunPod only** (no Anthropic-style API; sunsetting Dec 2025).

| Mode | When | Cost/100ch |
|---|---|---|
| Sonnet only (default) | All novels Phase 1 baseline | $4.65 (cached+batch) |
| Sonnet + Typhoon post-edit on RunPod | Novels needing premium Thai polish | $4.65 + $5 = $9.65 |

RunPod config:
- GPU: 1× H100 80GB (~$2.69/hr spot) or 1× A100 80GB (~$1.49/hr spot)
- Model: `scb10x/llama3.1-typhoon2-70b-instruct` via vLLM
- Batch mode: queue 10-50 chapters per spin-up to amortize startup (60-90s load)
- Idle: spin down after 5 min of empty queue

Adapter implements both. Operator picks per novel:
```yaml
novel_settings:
  novel_id: 42
  llm_primary: claude-sonnet-4-6
  llm_post_edit: typhoon-2-70b-runpod   # null to disable
```

Phase 1 default: post-edit disabled. Enable per-novel after manual quality eval.

---

## Component 3 — Review UI (Human Feedback Loop)

### Goal

Reviewer reads side-by-side EN/TH, edits Thai, flags issues. Edits feed back to glossary + future translations.

### UI

```
┌────────────────────────────────────────────────────────────┐
│ Novel: <title>   Chapter 42: <title>   [Prev] [Next]       │
├──────────────────────────┬─────────────────────────────────┤
│ EN (source)              │ TH (translation, editable)      │
│                          │                                 │
│ [s1] He looked at her... │ [s1] เขามองเธอ...   [✏][⚠][✓]   │
│ [s2] "Master, please."   │ [s2] "ท่านอาจารย์..."  [✏][⚠][✓] │
│ ...                      │ ...                             │
└──────────────────────────┴─────────────────────────────────┘
  Issues: [pronoun_wrong] [name_inconsistent] [register_off] [other]
  Notes: [____________________]
  [Save Edits] [Approve Chapter]
```

### Schema

```sql
CREATE TYPE review_issue AS ENUM (
    'pronoun_wrong','name_inconsistent','register_off',
    'mistranslation','missing_text','extra_text',
    'formality_wrong','relation_wrong','other'
);

ALTER TABLE translation_pairs
    ADD COLUMN edited_th TEXT,                  -- reviewer's correction
    ADD COLUMN reviewer_id INTEGER,
    ADD COLUMN reviewed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN review_status VARCHAR(20) DEFAULT 'pending';

CREATE TABLE reviewer_feedback (
    id SERIAL PRIMARY KEY,
    pair_id INTEGER REFERENCES translation_pairs(id) ON DELETE CASCADE,
    issue review_issue NOT NULL,
    note TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE reviewers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(255) UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### Feedback Loop

1. Reviewer edits TH → `edited_th` saved.
2. If `pronoun_wrong` issue flagged → enqueue glossary update job:
   - Detect entity in source sentence
   - Update `entity_pronouns` table for that entity
   - Mark prior translations of same entity for re-review
3. If `name_inconsistent` → enqueue entity dedup job.
4. Validated pairs (`review_status='approved'`) flow to `train.jsonl` export.

---

## Hosting Plan (Singapore)

Self-host = **infra only** on Singapore VPS. LLM stays on API (Sonnet) or RunPod (Typhoon optional).

### Phase 1 (10 novels/mo, Sonnet only, no Typhoon)

| Service | Choice | Cost/mo |
|---|---|---|
| Compute (app + Postgres combined) | Linode/Vultr SG: 4 vCPU, 8 GB, 160 GB SSD | $24 |
| Backups | Daily snapshot | $5 |
| Anthropic API (Sonnet 4.6 cached+batch) | 10 novels × $4.65 | $47 |
| Embedding (Gemini text-embedding-004) | per novel | $1 |
| Domain + DNS | Cloudflare free tier | $0 |
| RunPod (Typhoon optional) | disabled Phase 1 | $0 |
| **Phase 1 total** | | **~$77/mo** |

### Phase 1 + Typhoon Enabled (per-novel opt-in)

| Service | Cost/mo |
|---|---|
| Phase 1 base | $77 |
| RunPod H100 spot, ~5 hr/mo for 10 novels post-edit | $14 |
| **Total** | **~$91/mo** |

### Phase 2 (100 novels/mo)

| Service | Cost/mo |
|---|---|
| Compute (split: app VPS + managed Postgres SG) | $50 |
| Sonnet API (cached + batch) 100 × $4.65 | $465 |
| Typhoon RunPod (if all novels post-edited, ~50 hr/mo) | $135 |
| Embedding | $10 |
| **Total Sonnet only** | **~$525** |
| **Total Sonnet + Typhoon** | **~$660** |

---

## Phase 1 Build Order (Estimated 4-6 weeks Solo)

| Week | Deliverable |
|---|---|
| 1 | Paste-to-DB tool: Go HTTP server + HTMX UI + chapter upsert + cleaning pipeline |
| 2 | Glossary editor (manual entity entry); migration 003 (entity_relationships, pronouns, formality) |
| 3 | Translation worker: Sonnet adapter + prompt cache + batch API; pgvector HNSW + tsvector indexes |
| 4 | 4-channel RAG context builder; first end-to-end translation; align + train.jsonl export |
| 5 | Review UI: side-by-side viewer + edit/flag/approve flows |
| 6 | Feedback loop: glossary update on flag; reviewer_feedback table; export pipeline |

---

## Phase 1 Success Criteria

- [ ] 1 novel × 10 chapters fully pipelined: paste → translate → review → approved
- [ ] Pronoun/honorific consistency across chapters (manual validation)
- [ ] Reviewer flags drive glossary updates without code change
- [ ] `train.jsonl` exports cleanly with reviewer-approved pairs
- [ ] Total cost ≤ $80/mo at 10 novels/mo

---

## Risks (Updated)

| # | Risk | Mitigation |
|---|---|---|
| R1 | Manual paste = bottleneck | Paste tool with auto-split + auto-detect chapter no. minimizes friction |
| R2 | Sonnet API cost runaway | Daily spend cap; alert at 80%; batch API only |
| R3 | Singapore VPS to Anthropic latency ~200ms | Acceptable for batch translation; not real-time |
| R4 | Typhoon API uncertain (sunsetting) | Phase 1 = Sonnet only; Phase 2 add Typhoon |
| R5 | Reviewer burnout (you = solo reviewer initially) | Build review UI with hotkeys; batch approve trivial pairs |
| R6 | Output legal exposure (gray zone) | Per A-2: stay private/small. Per A-9: permissive license is reviewer's choice |
| R7 | Glossary drift across novels | Per-novel scoping; entity_id is novel-scoped |

---

## Out of Scope Phase 1

- Automated scraping (deferred / removed)
- Multi-language (TH only)
- Public B2C site
- Multi-tenant / team accounts
- A/B LLM routing
- Apache AGE / graph traversal
- Microsoft GraphRAG
- Real-time translation API
