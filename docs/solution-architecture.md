# Novel Translator — Solution Architecture + BA Scope

Generated 2026-05-05. Two-part redesign: **Crawler** (raw HTML/text → DB) + **Translator Engine** (LLM adapter + RAG → Thai/multi-lang output).

---

## Part 0 — BA Scope Clarification (BLOCK BEFORE BUILD)

### Critical Concerns

#### 1. Legal — webnovel.com = HIGH RISK. Drop.
- Webnovel.com run by Cloudary Holdings / Yuewen. Hosts **licensed translations**.
- Scraping licensed translations = direct copyright infringement under DMCA.
- Active enforcement: `dmca@webnovel.com` takedown channel. History of suing aggregators.
- ToS forbids automated access.
- **Fine-tuning on infringed corpus = derivative work risk** if model published.
- **Recommend whitelist instead**:
  - `royalroad.com` — author copyright stays with author (NOT CC-default), but no commercial licensee. Filter by CC-BY/CC-BY-SA where author marks. Lower risk.
  - `lightnovelpub.com` / `lightnovelworld.com` — fan-aggregator, gray zone, no commercial rights-holder enforcement.
  - Public domain bilingual (Project Gutenberg, Wikisource).

#### 1b. Wuxiaworld.com — ALSO HIGH RISK. Drop.
- ToS explicit ban: "Spidering, crawling, or accessing the site through any automated means is not allowed."
- ToS explicit AI clause: "Use the platform content to develop, train, or improve artificial intelligence or machine learning models without prior written consent."
- Worse than webnovel.com for our use case — AI/ML training is named-and-banned. Even private fine-tuning = direct contract breach.
- Only exception: search engine indexing per `robots.txt`.
- **Drop.**

#### 2. Output Distribution Model — UNDEFINED
| Mode | Legal Risk | Revenue Path |
|---|---|---|
| Public B2C subscription republishing TL | 🔴 max | direct, but DMCA exposure |
| B2B SaaS for fan TL groups (they own output) | 🟢 low | tooling fees |
| Personal/private use only | 🟡 ToS only | none |
| Fine-tuning dataset (private) | 🟡 grey | model sale |
| Fine-tuning dataset (public release) | 🔴 high | reputation damage |

**Pick ONE before infra finalized.**

#### 3. Quality Bar — UNDEFINED
- Replace human TL → must beat fan TL → premium LLM (Claude Sonnet 4.6/Opus)
- Assist human TL (80% draft) → mid LLM (Gemini 2.5 Flash + glossary)
- Beat Google Translate baseline → cheap LLM (DeepSeek/Flash)

#### 4. SLA + Latency
- Real-time per-user request → expensive infra, idle waste
- **Recommend batch overnight** → cheap, queue-based

#### 5. Multi-Lang Scaling
- Phase 1: EN→TH only. Multi-lang adapter pattern but defer second lang.
- Add CN→TH or EN→ID after TH validated.

### BA Question Set for BU

| # | Question | Drives |
|---|---|---|
| Q1 | Output monetization model? (B2C/B2B/private) | Legal risk + revenue model |
| Q2 | Quality bar (replace/assist/raw)? | LLM tier choice |
| Q3 | Volume month 1, 6, 12? | Infra sizing + cost |
| Q4 | Source whitelist (sites allowed)? | Legal risk + crawler complexity |
| Q5 | Translation QA process (human reviewer? self-eval LLM?)? | Feedback loop |
| Q6 | Hosting region preference? | Latency + data residency |
| Q7 | Monthly budget cap? | Self-host vs API decision |
| Q8 | Languages after Thai (CN/JP/KR/ID)? Timeline? | Adapter design |
| Q9 | Output rights: who owns translated text? | Contract + revenue |
| Q10 | Acceptable per-chapter cost? | Quality/cost knob |

---

## Part 1 — Crawler Engine (Raw Fetch, NO LLM)

### Goals
- Fetch chapter HTML + extract narrative text. No summarization. No entity extraction.
- Bypass Cloudflare/Turnstile for protected sites.
- Store raw HTML + cleaned text to Postgres for downstream consumer.
- Idempotent. Retry-safe. Rate-limited per domain.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Source Registry (whitelist of approved sites + adapters)   │
└────────────────┬────────────────────────────────────────────┘
                 │
       ┌─────────▼──────────┐
       │  Index Crawler     │  Discover chapter URLs
       │  (per-novel job)   │  → INSERT chapters(scrape_status=pending)
       └─────────┬──────────┘
                 │
       ┌─────────▼──────────────────────────┐
       │  Chapter Worker Pool (Go, N=2-4)   │
       │  SELECT ... FOR UPDATE SKIP LOCKED │
       │                                    │
       │  ┌─Tier 1: plain HTTP + UA rotate  │
       │  │  fail→503/CF challenge          │
       │  ├─Tier 2: Patchright + residential│
       │  │  fail→still 403                 │
       │  └─Tier 3: Bright Data Unlocker API│
       └─────────┬──────────────────────────┘
                 │
        ┌────────▼─────────┐
        │  HTML Cleaner    │  goquery → strip tags
        │  + Text Extract  │  → cleaned_content
        └────────┬─────────┘
                 │
        ┌────────▼─────────┐
        │  Postgres        │
        │  raw_html (gz)   │  ← keep raw for replay
        │  cleaned_content │  ← consumer reads this
        │  scrape_status   │
        └──────────────────┘
```

### Tool Decision (from research)

| Need | Pick | Why |
|---|---|---|
| Cheap sites (royalroad, no CF) | plain HTTP + UA rotate | $0 |
| Mild CF | Patchright + residential proxy | ~70% success, $20-70/mo |
| Hard CF (Turnstile) | Bright Data Web Unlocker | 98% success, pay-per-success $1.50/1k |
| Self-host fallback | Camoufox | 90% success but 200MB+ RAM, 42s/challenge |

**Reject**: playwright-extra stealth (unmaintained since Mar 2023), FlareSolverr (no Turnstile interactive solve), undetected-chromedriver (declining).

### Storage

```sql
ALTER TABLE chapters
  ADD COLUMN raw_html BYTEA,                 -- gzipped raw HTML for replay
  ADD COLUMN content_hash CHAR(64),          -- sha256 for dedup
  ADD COLUMN scrape_attempts INTEGER DEFAULT 0,
  ADD COLUMN scrape_tier SMALLINT,           -- which tier succeeded
  ADD COLUMN last_scrape_error TEXT,
  ADD COLUMN scraped_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX idx_chapters_pending ON chapters(scrape_status, novel_id)
  WHERE scrape_status = 'pending';
```

Worker query (lock-free):
```sql
UPDATE chapters
SET scrape_status = 'in_progress', scrape_attempts = scrape_attempts + 1
WHERE id = (
    SELECT id FROM chapters
    WHERE scrape_status = 'pending' AND scrape_attempts < 5
    ORDER BY id LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, source_url, novel_id;
```

### Cost Matrix (USD/month)

| Volume | Self-host (Patchright + proxy) | Managed (Bright Data Unlocker) |
|---|---|---|
| Light: 10 novels (~1.5k req) | $25 ($10 VPS + $15 proxy) | $12 |
| Medium: 100 novels (~15k req) | $70 | $25 |
| Heavy: 1000 novels (~150k req) | $350 | $250 |

**Recommendation Phase 1**: Bright Data managed at light tier (~$12/mo). Self-host only if monthly volume >500k req.

### Crawler Adapter Interface (Go)

```go
type SourceAdapter interface {
    Name() string
    MatchesURL(url string) bool
    DiscoverChapters(ctx context.Context, indexURL string) ([]ChapterRef, error)
    FetchChapter(ctx context.Context, chapterURL string) (*RawChapter, error)
}

type RawChapter struct {
    HTML      []byte    // gzipped raw
    Text      string    // cleaned narrative
    Title     string
    ChapterNo int
    FetchedAt time.Time
}

type Fetcher interface {  // tiered
    Fetch(ctx context.Context, url string) ([]byte, error)
}
```

Per-site adapter only knows DOM selectors + URL patterns. Fetcher abstraction handles anti-bot.

---

## Part 2 — Translator Engine

### Goals
- Pull `chapters WHERE scrape_status='completed' AND translation_status='pending'`.
- Build per-chapter context: glossary + recent plot + relationships.
- Call LLM via adapter. Multi-LLM, multi-target-lang.
- Store translation + aligned sentence pairs.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Translation Worker (Go orchestrator + Python LLM client)   │
└────────────────┬────────────────────────────────────────────┘
                 │
   ┌─────────────▼──────────────────────────┐
   │  Context Builder (per chapter)         │
   │  ┌──────────────────────────────────┐  │
   │  │ Entity-first SQL lookup          │  │  ← canonical names
   │  │ Hybrid BM25+vector retrieval     │  │  ← past aligned pairs
   │  │ Recent chapters (n-3..n-1)       │  │  ← plot context
   │  │ Glossary (deterministic dict)    │  │  ← terms/places
   │  │ Pronoun + formality matrix       │  │  ← honorific table
   │  └──────────────────────────────────┘  │
   └─────────────┬──────────────────────────┘
                 │
       ┌─────────▼──────────┐
       │  LLM Adapter       │  Claude / Gemini / Typhoon / DeepSeek
       │  + prompt cache    │  glossary cached 90% discount
       │  + batch API       │  50% off non-realtime
       └─────────┬──────────┘
                 │
       ┌─────────▼──────────┐
       │  Post-processor    │
       │  - sentence align  │
       │  - validation pass │  (rule-based: pronouns match, no EN leak)
       └─────────┬──────────┘
                 │
       ┌─────────▼──────────┐
       │  Postgres          │
       │  translations      │  full chapter
       │  translation_pairs │  sentence-level
       │  entity_mentions   │  detected per chapter
       └────────────────────┘
```

### LLM Adapter Pattern

```go
type LLMAdapter interface {
    Name() string
    Translate(ctx context.Context, req TranslationRequest) (*TranslationResponse, error)
    SupportsCache() bool
    SupportsBatch() bool
}

type TranslationRequest struct {
    SourceText   string
    SourceLang   string
    TargetLang   string
    Glossary     []GlossaryEntry      // cached portion
    StyleGuide   string                // cached portion
    RecentPlot   string                // dynamic per chapter
    SystemPrompt string                // cached
}
```

Per-provider impl: `claude_adapter.go`, `gemini_adapter.go`, `typhoon_adapter.go`, `deepseek_adapter.go`. Driver chosen by config or A/B routing.

### Vector Store Decision: KEEP pgvector

| Store | Verdict | Reason |
|---|---|---|
| **pgvector (current)** | ✅ KEEP | Scale fits (1-2M vectors, 100x below pgvector breakpoint at 50-100M). Single Postgres = simple. JOIN entities + vectors in one query. |
| Qdrant / Weaviate / Pinecone | ❌ skip | Adds infra. No quality gain at scale. |
| Microsoft GraphRAG | ❌ skip | Designed for unstructured corpora needing LLM-extracted graph; you have structured chapters. Wasted indexing cost. |
| Apache AGE on Postgres | ⏸ defer | Add only when multi-hop graph traversal becomes bottleneck (e.g. "master-of-master-of-X"). Single-Postgres openCypher. |

**Index choice**: HNSW, not IVFFlat. Production default above 500K vectors.

### RAG Retrieval Pattern (Translation-Specific)

Pure embedding RAG **collapses entity mentions across time** — fatal for narrative. Per Entity-Event RAG (E²RAG, arXiv 2506.05939). Use **4-channel retrieval**:

| Channel | Source | Query | Why |
|---|---|---|---|
| 1. Entity-first | SQL on `entities` table | NER source sentence → exact name lookup | Proper-noun consistency |
| 2. Hybrid BM25+vector | `tsvector` + pgvector + RRF | source sentence | Past aligned pair retrieval (recall 0.72→0.91 with hybrid) |
| 3. Recent plot | SQL `chapter_number BETWEEN n-3 AND n-1` | `novel_id` | Continuity |
| 4. Glossary | Deterministic dict lookup | matched terms | Forced consistency |

Inject as constraints in prompt, not as freeform retrieval blob.

### Schema Additions

```sql
-- Translation output
CREATE TABLE translations (
    id SERIAL PRIMARY KEY,
    chapter_id INTEGER REFERENCES chapters(id) ON DELETE CASCADE,
    target_lang VARCHAR(10) NOT NULL,
    llm_provider VARCHAR(50),
    llm_model VARCHAR(100),
    content TEXT NOT NULL,
    cost_usd NUMERIC(10,6),
    tokens_in INTEGER,
    tokens_out INTEGER,
    cached_tokens INTEGER,
    quality_score FLOAT,
    status process_status DEFAULT 'completed',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chapter_id, target_lang, llm_provider, llm_model)
);

-- Pronoun + rank table (per character)
CREATE TABLE entity_pronouns (
    id SERIAL PRIMARY KEY,
    entity_id INTEGER REFERENCES entities(id) ON DELETE CASCADE,
    target_lang VARCHAR(10) NOT NULL,
    self_ref VARCHAR(50),       -- ผม / ข้า / กู / หม่อมฉัน
    addressed_as VARCHAR(50),   -- คุณ / ท่าน / เจ้า / มึง
    rank_tier INTEGER DEFAULT 0,
    notes TEXT,
    UNIQUE(entity_id, target_lang)
);

-- Relationship edges
CREATE TYPE relation_type AS ENUM (
    'parent_of','child_of','sibling_of','spouse_of',
    'master_of','disciple_of','ally_of','enemy_of',
    'subordinate_of','superior_of','friend_of','rival_of','other'
);
CREATE TYPE formality_level AS ENUM (
    'formal_high','formal','neutral','casual','rude'
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
    UNIQUE(novel_id, entity_a_id, entity_b_id, relation)
);

-- Hybrid BM25 + vector
ALTER TABLE translation_pairs
  ADD COLUMN sentence_en_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('english', sentence_en)) STORED,
  ADD COLUMN embedding vector(768);
CREATE INDEX idx_pairs_tsv ON translation_pairs USING GIN(sentence_en_tsv);
CREATE INDEX idx_pairs_emb ON translation_pairs
  USING hnsw (embedding vector_cosine_ops);
```

### LLM Cost Matrix

Per chapter ~6K input + 5K output. Per 100-chapter novel = 600K in + 500K out.

| Model | Cost/Chapter | Cost/100ch Novel | Thai Quality | When to Pick |
|---|---|---|---|---|
| Claude Opus 4.7 | $0.155 | $15.50 | 🥇🥇 | Premium; replace human TL |
| Claude Sonnet 4.6 | $0.093 | $9.30 | 🥇 | **DEFAULT** for replace human TL |
| Gemini 2.5 Pro | $0.0575 | $5.75 | 🥈 | Long-context multi-chapter consistency |
| GPT-5 | $0.0575 | $5.75 | 🥈 | Accuracy > literary voice |
| Typhoon 2 70B | $0.0097 | $0.97 | 🥇 (Thai-native) | Post-editor on Sonnet draft |
| Gemini 2.5 Flash | $0.0143 | $1.43 | 🥈 | **DEFAULT** for assist mode |
| GPT-4.1 mini | $0.0104 | $1.04 | 🥉 | Cheap volume |
| DeepSeek V3.1 | $0.0047 | $0.47 | 🥉 (flat) | Cheap baseline; weak at register |
| RunPod Llama 3.3 70B | ~$0.05 | ~$5.00 | 🥉 (no Thai) | Self-host only if API blocked |

**Quality ranking for Thai literary novel**: Sonnet/Opus > Typhoon 2 > Gemini Pro > GPT-5 > Qwen3 > Gemini Flash > DeepSeek > GPT-4.1 mini.

### Prompt Caching Strategy

Glossary + style guide ~5K tokens cached per novel. Sonnet 4.6:
- Uncached: 100ch × 5K × $3/M = **$1.50/novel**
- Cached: $0.19 write + $0.15 hits = **$0.34/novel** (77% saving on cached portion, ~12% on chapter total)

Stack with batch API (50% off, non-realtime) → Sonnet effective input ~$0.15/M = matches Gemini Flash price with Claude quality. **Optimal default**.

### Recommended LLM Routing Strategy

| Tier | Use Case | Stack |
|---|---|---|
| **Premium** | Final published TL | Claude Sonnet 4.6 + cache + batch + Typhoon 2 post-edit pass |
| **Standard** | Draft for human reviewer | Gemini 2.5 Flash + cache (1M context = whole novel) |
| **Volume** | Bulk dataset for fine-tuning | DeepSeek V3.1 + auto-cache |
| **Eval/A-B** | Quality benchmark | Run all 3 on same chapter, score with Claude Opus judge |

Adapter implements all 4. Routing config per novel.

---

## Total Cost Estimates

### Phase 1 MVP (10 novels/month)

| Component | Choice | $/mo |
|---|---|---|
| Crawler | Bright Data Unlocker | $12 |
| LLM (Sonnet + cache + batch) | Premium tier | 10 × $9.30 × 0.5 = $46.50 |
| LLM (alt Flash) | Standard tier | 10 × $1.43 = $14.30 |
| Postgres + pgvector | $5 VPS / RDS small | $20 |
| Embedding (Gemini text-emb-004) | per chapter | $1 |
| **Total Premium** | | **~$80/mo** |
| **Total Standard** | | **~$50/mo** |

### Phase 2 (100 novels/month)
- Crawler: $25
- LLM Sonnet+cache+batch: 100 × $4.65 = $465
- LLM Flash: 100 × $1.43 = $143
- DB: $50
- **Total Premium: ~$540/mo. Standard: ~$220/mo.**

### Phase 3 (1000 novels/month)
- Crawler: $250
- LLM Sonnet+batch: $4,650
- LLM Flash: $1,430
- DB: $200 (managed Postgres w/ replicas)
- **Total Premium: ~$5,100/mo. Standard: ~$1,900/mo.**

---

## Decision Matrix Summary

| Decision | Pick | Defer/Skip |
|---|---|---|
| Source whitelist | royalroad.com (CC-marked), lightnovelpub, public domain | webnovel.com (drop), wuxiaworld.com (drop — explicit AI/ML ban in ToS) |
| Anti-bot | Bright Data Unlocker (managed) | Self-host stealth (only at >500k req/mo) |
| Vector store | pgvector + HNSW + tsvector | Pinecone, Qdrant, Weaviate, GraphRAG |
| Graph layer | Custom entity tables (SQL JOIN) | Apache AGE (defer until multi-hop bottleneck), Neo4j (skip) |
| LLM premium | Claude Sonnet 4.6 + cache + batch | Opus (only for final-pass) |
| LLM standard | Gemini 2.5 Flash | DeepSeek (register flat) |
| LLM Thai post-edit | Typhoon 2 70B (when AWS relaunched) | OpenThaiGPT (outdated) |
| Embedding | Gemini text-embedding-004 (768) | Switch only when changing column dim |
| RAG pattern | 4-channel: entity-first + hybrid BM25/vec + recent plot + glossary dict | Pure vector RAG |

---

## Risks + Open Items

| # | Risk | Mitigation |
|---|---|---|
| R1 | Webnovel.com DMCA if BU insists on it | Refuse; route to royalroad CC-marked only |
| R2 | Cloudflare Turnstile evolves, breaks scraper | Tier 3 managed Unlocker absorbs; SLA from vendor |
| R3 | Typhoon 2 API sunset Dec 2025, AWS relaunch slip | Run Sonnet alone until relaunch confirmed |
| R4 | Cost runaway on premium LLM at scale | Daily spend cap per LLM adapter; alert at 80% |
| R5 | pgvector latency at 10M+ vectors | HNSW index from day 1; partition by `novel_id` |
| R6 | Hallucinated entities or pronouns | Rule-based post-validator: NER on output, compare against entity table; flag mismatches |
| R7 | Multi-lang scaling pollutes glossary | Per-target-lang glossary tables (already in schema) |

### Open Questions for BU (Block Phase 1 Start)

- [ ] Q1-Q10 from BA section answered?
- [ ] Source whitelist signed off (no webnovel.com)?
- [ ] Quality bar locked (premium/standard/volume)?
- [ ] Monthly budget approved?
- [ ] Output rights agreement drafted?

---

## Sources

### Crawler
- [Webnovel ToS](https://www.webnovel.com/terms_of_service)
- [Royal Road ToS](https://www.royalroad.com/tos), [Royal Road DMCA](https://www.royalroad.com/dmca)
- [Playwright Stealth 2026 - scrapewise](https://scrapewise.ai/blogs/playwright-stealth-2026)
- [Cloudflare Turnstile Bypass 2026 - Tapscape](https://www.tapscape.com/cloudflare-turnstile-bypass-2026-the-core-level-stealth-guide/)
- [Scrapfly: Bypass Cloudflare 2026](https://scrapfly.io/blog/posts/how-to-bypass-cloudflare-anti-scraping)
- [ZenRows: Bypass Cloudflare](https://www.zenrows.com/blog/bypass-cloudflare)
- [Bright Data vs Oxylabs 2026](https://brightdata.com/blog/comparison/bright-data-vs-oxylabs)

### LLM
- [Claude API Pricing 2026 - BenchLM](https://benchlm.ai/blog/posts/claude-api-pricing)
- [Anthropic API Pricing 2026 - Finout](https://www.finout.io/blog/anthropic-api-pricing)
- [Gemini API Pricing 2026 - TLDL](https://www.tldl.io/resources/google-gemini-api-pricing)
- [OpenAI API Pricing 2026 - DevTk](https://devtk.ai/en/blog/openai-api-pricing-guide-2026/)
- [DeepSeek V3.1 Pricing](https://pricepertoken.com/pricing-page/model/deepseek-deepseek-chat-v3.1)
- [Typhoon 2 SCB10X](https://www.scb10x.com/en/blog/introducing-typhoon-2-thai-llm)
- [Best LLM for Translation - Lokalise](https://lokalise.com/blog/what-is-the-best-llm-for-translation/)
- [Prompt Caching Comparison - Artificial Analysis](https://artificialanalysis.ai/models/caching)
- [Thai LLM Leaderboard - OpenTyphoon](https://opentyphoon.ai/blog/en/introducing-the-thaillm-leaderboard-thaillm-evaluation-ecosystem-508e789d06bf)

### Vector / RAG
- [pgvector HNSW vs IVFFlat - AWS](https://aws.amazon.com/blogs/database/optimize-generative-ai-applications-with-pgvector-indexing-a-deep-dive-into-ivfflat-and-hnsw-techniques/)
- [pgvector benchmarks - Instaclustr](https://www.instaclustr.com/education/vector-database/pgvector-performance-benchmark-results-and-5-ways-to-boost-performance/)
- [Vector DB benchmarks 2026 - CallSphere](https://callsphere.ai/blog/vector-database-benchmarks-2026-pgvector-qdrant-weaviate-milvus-lancedb)
- [pgvector + Apache AGE - MS Tech Community](https://techcommunity.microsoft.com/blog/adforpostgresql/combining-pgvector-and-apache-age---knowledge-graph--semantic-intelligence-in-a-/4508781)
- [Entity-Event RAG (narrative consistency)](https://arxiv.org/html/2506.05939)
- [Hybrid BM25+vector RAG - Superlinked](https://superlinked.com/vectorhub/articles/optimizing-rag-with-hybrid-search-reranking)
- [Microsoft GraphRAG GitHub](https://github.com/microsoft/graphrag)
