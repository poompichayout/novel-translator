# EN→TH Novel Translator — Validation + GraphRAG Research

Generated 2026-05-05.

## Validate-Idea Verdict: **NEEDS MORE VALIDATION**

### Problem Definition

Who exactly?
- Thai readers of EN web novels (xianxia, isekai, LitRPG)
- Fan TL groups burning out on manual work
- Patreon-backed MTL editors

Current workarounds (= real competition):
- Google Translate / DeepL → break on pronouns, honorifics, gender
- Fan TL groups → slow, abandon novels mid-series
- MTL+editor pipeline → still ships pronoun bugs (English "I" → Thai needs ผม/ฉัน/กู/ข้า/หม่อมฉัน per speaker rank)
- Fictionlog, ReadAWrite Thai TL communities

Pain level: real. Thai readers actively complain. Honorific/relationship bugs ruin xianxia translation specifically.

### Red Flags

- ❓ Belong to community? Unclear. If not Thai novel reader → big risk.
- ❓ 10 named potential customers? Not yet.
- ❓ 3 paid validations? None.
- ❓ Manual chapter delivered to anyone? No.

### Green Flags

- People already pay (Fictionlog subscriptions, Patreon for fan TL)
- Niche complains loudly about MTL quality
- One-sentence customer description possible: "Thai xianxia readers who pay Patreon for fan TL chapters delayed by burnout"

### Validation Steps Before More Code

1. **Pick 1 stalled EN novel popular in Thai community.** Look at NovelUpdates Thai TL trackers, Reddit r/noveltranslations, Fictionlog dropped novels.
2. **Translate 3 chapters by hand using your pipeline.** You = the manual process. No automation yet.
3. **Post free in Thai novel Discord/Facebook group.** Measure reaction.
4. **Charge 50-100 THB next 3 chapters.** If 10 readers pay → validated. If zero pay → pivot.
5. **Only THEN automate.** Sahil rule: ship in weekend, profitable day one.

Stop building until step 4 hits paying customer. Repo already over-engineered for unvalidated demand.

### Pivot Suggestions If Step 4 Fails

- B2B: sell pipeline to existing Thai TL platforms (Fictionlog, ReadAWrite) as backend
- Tooling: sell glossary+relationship-graph editor to fan TL groups (they keep doing TL, you sell their workflow tool)
- Specific genre: xianxia/cultivation only — relationship graph + cultivation rank tracking = real differentiator

---

## GraphRAG Research

### License + Cost

| Item | Status |
|---|---|
| GraphRAG library | **Free, MIT license, open source** (microsoft/graphrag on GitHub) |
| Microsoft support | None official. "Demonstration code." |
| LLM indexing cost | **5-10x source token cost** for full GraphRAG |
| Index cost vs vector RAG | **4-8x more expensive** |
| **LazyGraphRAG** | New variant. **0.1% of full GraphRAG cost.** Same quality on many tasks. |

100-chapter novel ~ 300k-500k tokens source. Full GraphRAG indexing = 2M-5M token LLM bill per novel. Gemini 2.5 Flash ~$0.30/1M input → ~$1-5 per novel indexing. RunPod self-host = near zero LLM cost but slow.

### Compatibility w/ Current Stack

| Component | Fit |
|---|---|
| Postgres + pgvector | ❌ GraphRAG default uses LanceDB/Parquet. Need adapter or dual-store. |
| Python alignment service | ✅ GraphRAG is Python. Drop-in. |
| Go ingestion service | ⚠️ Shell out same way as scrapegraph. |
| Existing `entities` table | ✅ Mirror GraphRAG entity output into existing schema. |

### Recommendation: Don't Use Full GraphRAG. Build Lite Custom Graph.

Reasons:
- Full GraphRAG = community summarization for global queries (e.g. "what themes run through this corpus?"). Translation needs **per-chapter local entity lookup**. Wrong tool.
- Translation needs cheap fast lookup of "who is this pronoun?", "rank of speaker A vs B?", "first appearance chapter?".
- Indexing cost 5-10x kills economics on 100+ chapter scale.
- Existing schema (`entities`, `entities.embedding vector(768)`) already handles 80% of what's needed.

Better path:
1. **Steal GraphRAG entity extraction prompt** (open source, MIT — copy + adapt). Run per chapter.
2. **Add `entity_relationships` table**: `(novel_id, entity_a_id, entity_b_id, relation_type, evidence_chapter_id, formality_level)`.
3. **Add `entity_appearances` table**: track first/last chapter of each entity for timeline.
4. **At translation time** retrieve via 2-hop SQL JOIN, not LLM-driven graph traversal.
5. **If quality lacks**, evaluate **LazyGraphRAG** (0.1% cost) before full GraphRAG.

Engineering tradeoff: lose GraphRAG community summaries (don't need), gain native pgvector + Postgres simplicity + 100x cheaper indexing.

### Per-Constraint Approach

| Constraint | Approach |
|---|---|
| Pronoun resolution | Entity coreference resolver per chapter → pronoun → entity_id → speaker rank → Thai pronoun lookup table |
| Relationship | Custom edge table; LLM extracts on entity upsert; cache per novel |
| Timeline | `entity_appearances.chapter_number` MIN/MAX; flag entity introductions for translator hint |
| Formality (ระดับภาษา) | Speaker→listener rank delta → formality enum (สุภาพทางการ/สุภาพ/กันเอง/หยาบ); pass as glossary to LLM at translation step |

Don't make one big GraphRAG call solve all four. Each is a separate small system.

### Proposed Schema Additions

```sql
-- Track when entity first/last appears (timeline)
CREATE TABLE entity_appearances (
    id SERIAL PRIMARY KEY,
    entity_id INTEGER REFERENCES entities(id) ON DELETE CASCADE,
    chapter_id INTEGER REFERENCES chapters(id) ON DELETE CASCADE,
    mention_count INTEGER DEFAULT 1,
    first_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity_id, chapter_id)
);

-- Edges between entities (relationships)
CREATE TYPE relation_type AS ENUM (
    'parent_of', 'child_of', 'sibling_of', 'spouse_of',
    'master_of', 'disciple_of',
    'ally_of', 'enemy_of',
    'subordinate_of', 'superior_of',
    'friend_of', 'rival_of',
    'other'
);

CREATE TYPE formality_level AS ENUM (
    'formal_high',     -- ทางการสูง (royal, master→subject)
    'formal',          -- ทางการ (work, stranger)
    'neutral',         -- กลาง
    'casual',          -- กันเอง (friends)
    'rude'             -- หยาบ (enemy, contempt)
);

CREATE TABLE entity_relationships (
    id SERIAL PRIMARY KEY,
    novel_id INTEGER REFERENCES novels(id) ON DELETE CASCADE,
    entity_a_id INTEGER REFERENCES entities(id) ON DELETE CASCADE,
    entity_b_id INTEGER REFERENCES entities(id) ON DELETE CASCADE,
    relation relation_type NOT NULL,
    formality formality_level DEFAULT 'neutral',
    evidence_chapter_id INTEGER REFERENCES chapters(id),
    confidence FLOAT DEFAULT 1.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(novel_id, entity_a_id, entity_b_id, relation)
);

-- Pronoun map per character (for translation step)
CREATE TABLE entity_pronouns (
    id SERIAL PRIMARY KEY,
    entity_id INTEGER REFERENCES entities(id) ON DELETE CASCADE,
    self_reference_th VARCHAR(50),    -- ผม / ข้า / กู / หม่อมฉัน
    addressed_as_th   VARCHAR(50),    -- คุณ / ท่าน / เจ้า / มึง
    rank_tier INTEGER DEFAULT 0,      -- 0=peasant, 5=royal/master
    notes TEXT,
    UNIQUE(entity_id)
);
```

## Sources

- [Project GraphRAG - Microsoft Research](https://www.microsoft.com/en-us/research/project/graphrag/)
- [microsoft/graphrag GitHub](https://github.com/microsoft/graphrag)
- [LazyGraphRAG sets a new standard for GraphRAG quality and cost](https://www.microsoft.com/en-us/research/blog/lazygraphrag-setting-a-new-standard-for-quality-and-cost/)
- [GraphRAG Pricing 2026](https://aitoolsatlas.ai/tools/graphrag/pricing)
- [Welcome - GraphRAG docs](https://microsoft.github.io/graphrag/)
- [Methods - GraphRAG](https://microsoft.github.io/graphrag/index/methods/)
- [GraphRAG vs Traditional RAG - ragaboutit](https://ragaboutit.com/graphrag-vs-traditional-rag-when-multi-hop-reasoning-becomes-your-competitive-advantage/)
- [RAG vs Graph-RAG 2026](https://ninthpost.com/rag-vs-graph-rag/)
