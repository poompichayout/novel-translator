# Week 1 — Paste-to-DB Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the operator-facing Paste-to-DB tool so chapters land in Postgres with a cleaned body, ready for the Week 3 translation worker.

**Architecture:** New `internal/server` Go package mounts a `chi` router behind HTTP basic auth. Server-rendered HTMX views post raw text to `POST /api/chapters`, which runs the existing `internal/cleaner` pipeline (extended with chapter-header / translator-note / ad-line strippers) and upserts via the existing `repository.PostgresRepo`. UI uses HTMX + Tailwind via CDN (no build step). The `serve` CLI command (currently a stub returning exit 2) is replaced with the real server bootstrap.

**Tech Stack:** Go 1.24, `github.com/go-chi/chi/v5`, `html/template`, `pgx/v5/pgxpool`, HTMX 1.x via unpkg CDN, Tailwind via CDN, `net/http/httptest` for handler tests, real Postgres for repo integration tests.

**Source spec:** `docs/phase-1-spec.md` Component 1 (Paste-to-DB Tool) and Week 1 build-order row. Locked decisions A-1, A-4, A-7.

---

## File Structure

**Create:**

- `services/ingestion/internal/server/server.go` — chi router setup, basic-auth middleware, `Run(ctx)` entry point.
- `services/ingestion/internal/server/server_test.go` — basic-auth middleware test.
- `services/ingestion/internal/server/handlers.go` — `Handlers` struct holding a `Repo` interface, all HTTP handlers.
- `services/ingestion/internal/server/handlers_test.go` — handler tests using `httptest` + a fake repo.
- `services/ingestion/internal/server/templates.go` — `embed`ed HTML templates and a thin `render` helper.
- `services/ingestion/internal/server/templates/layout.html`
- `services/ingestion/internal/server/templates/paste.html`
- `services/ingestion/internal/server/templates/novel_options.html`
- `services/ingestion/internal/server/templates/chapter_saved.html`
- `services/ingestion/internal/cleaner/cleaner_test.go` — unit tests for new strippers and the pipeline.
- `services/ingestion/internal/repository/postgres_test.go` — integration tests for read methods (requires `TEST_DB_URL`).

**Modify:**

- `services/ingestion/go.mod` / `go.sum` — add `github.com/go-chi/chi/v5`.
- `services/ingestion/internal/config/config.go` — add `Server.Addr` + `Server.Password`, env overrides.
- `services/ingestion/internal/cleaner/cleaner.go` — add `StripChapterHeader`, `StripTranslatorNotes`, `StripPromoLines`, `FullCleanChapter`.
- `services/ingestion/internal/repository/postgres.go` — add `ListNovels`, `GetNovel`, `ListChapters`, `GetNextChapterNumber`; introduce a `Repo` interface in the server package referencing only what the handlers use.
- `services/ingestion/cmd/main.go` — wire `serve` cmd to call `server.Run(ctx, repo, cfg)`.
- `config.yaml` — add `server.addr`, `server.password` (non-committed defaults already gitignored).

---

## Conventions

- Every task ends with a green test run and a commit.
- Repo integration tests skip when `TEST_DB_URL` is unset (`t.Skip("TEST_DB_URL not set")`).
- Handler tests use a hand-written fake repo (not gomock) to keep deps small.
- All handlers return HTMX partials for HTMX requests (`HX-Request: true`) and HTML pages otherwise.
- All commit messages follow the existing repo style: `feat: ...`, `fix: ...`, `test: ...`.

---

## Tasks

### Task 1: Add chi dependency

**Files:**
- Modify: `services/ingestion/go.mod`, `services/ingestion/go.sum`

- [ ] **Step 1: Add chi v5**

```bash
cd services/ingestion && go get github.com/go-chi/chi/v5@v5.1.0 && go mod tidy
```

- [ ] **Step 2: Verify build still passes**

Run: `cd services/ingestion && go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add services/ingestion/go.mod services/ingestion/go.sum
git commit -m "chore: add chi v5 router dependency"
```

---

### Task 2: Extend config with server section

**Files:**
- Modify: `services/ingestion/internal/config/config.go`
- Modify: `config.yaml`

- [ ] **Step 1: Update config struct**

In `services/ingestion/internal/config/config.go`, replace the `Config` struct with:

```go
type Config struct {
	Database struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	Embedding struct {
		GeminiAPIKey string `yaml:"gemini_api_key"`
	} `yaml:"embedding"`
	LLM struct {
		AnthropicAPIKey string `yaml:"anthropic_api_key"`
	} `yaml:"llm"`
	Server struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
	} `yaml:"server"`
}
```

Add env overrides at the bottom of `Load`, after the existing `ANTHROPIC_API_KEY` block:

```go
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("SERVER_PASSWORD"); v != "" {
		cfg.Server.Password = v
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
```

- [ ] **Step 2: Update config.yaml**

Append to `config.yaml`:

```yaml
server:
  addr: ":8080"
  password: "change-me"
```

- [ ] **Step 3: Build verification**

Run: `cd services/ingestion && go build ./...`
Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add services/ingestion/internal/config/config.go config.yaml
git commit -m "feat(config): add server addr + basic-auth password"
```

---

### Task 3: Cleaner — StripChapterHeader (TDD)

**Files:**
- Create: `services/ingestion/internal/cleaner/cleaner_test.go`
- Modify: `services/ingestion/internal/cleaner/cleaner.go`

- [ ] **Step 1: Write the failing test**

Create `services/ingestion/internal/cleaner/cleaner_test.go`:

```go
package cleaner

import "testing"

func TestStripChapterHeader(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"explicit chapter prefix", "Chapter 12: The Hunt\nHe walked in.", "He walked in."},
		{"chapter without colon", "Chapter 7\nThe sky was red.", "The sky was red."},
		{"no header preserved", "He walked in.\nIt was cold.", "He walked in.\nIt was cold."},
		{"chinese-style header", "第十二章 狩猎\n他走了进来。", "他走了进来。"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StripChapterHeader(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: FAIL — `undefined: StripChapterHeader`.

- [ ] **Step 3: Implement StripChapterHeader**

Append to `services/ingestion/internal/cleaner/cleaner.go`:

```go
// StripChapterHeader removes a leading "Chapter N[: title]" or "第N章 ..." line if present.
func StripChapterHeader(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*chapter\s+\d+[^\n]*\n`),
		regexp.MustCompile(`^\s*第[一二三四五六七八九十百千零\d]+章[^\n]*\n`),
	}
	for _, re := range patterns {
		text = re.ReplaceAllString(text, "")
	}
	return text
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: PASS, all 4 subtests green.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/cleaner/cleaner.go services/ingestion/internal/cleaner/cleaner_test.go
git commit -m "feat(cleaner): strip leading chapter header line"
```

---

### Task 4: Cleaner — StripTranslatorNotes (TDD)

**Files:**
- Modify: `services/ingestion/internal/cleaner/cleaner_test.go`
- Modify: `services/ingestion/internal/cleaner/cleaner.go`

- [ ] **Step 1: Add failing test**

Append to `cleaner_test.go`:

```go
func TestStripTranslatorNotes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"He looked. [T/N: subtle reference]", "He looked."},
		{"\"Master.\" (TL note: master is shifu)", "\"Master.\""},
		{"Plain prose with no notes.", "Plain prose with no notes."},
		{"Multi (TN: foo) chunks (T/N: bar) here.", "Multi  chunks  here."},
	}
	for _, tc := range cases {
		got := StripTranslatorNotes(tc.in)
		if got != tc.want {
			t.Errorf("StripTranslatorNotes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: FAIL — `undefined: StripTranslatorNotes`.

- [ ] **Step 3: Implement**

Append to `cleaner.go`:

```go
// StripTranslatorNotes removes inline translator notes such as [T/N: ...], (TL note: ...), (TN: ...).
// Trailing whitespace before/after the removal is left in place; callers should normalize whitespace.
func StripTranslatorNotes(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\s*\[T/?N:[^\]]*\]`),
		regexp.MustCompile(`\s*\((?:TL note|TN|T/N|Translator note)[^)]*\)`),
	}
	for _, re := range patterns {
		text = re.ReplaceAllString(text, "")
	}
	return text
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/cleaner/cleaner.go services/ingestion/internal/cleaner/cleaner_test.go
git commit -m "feat(cleaner): strip inline translator notes"
```

---

### Task 5: Cleaner — StripPromoLines (TDD)

**Files:**
- Modify: `services/ingestion/internal/cleaner/cleaner_test.go`
- Modify: `services/ingestion/internal/cleaner/cleaner.go`

- [ ] **Step 1: Add failing test**

Append to `cleaner_test.go`:

```go
func TestStripPromoLines(t *testing.T) {
	in := "He walked in.\nRead at example-novel-site.com for free!\nIt was cold.\nVisit https://novelhost.io for more.\nEnd."
	want := "He walked in.\nIt was cold.\nEnd."
	got := StripPromoLines(in)
	if got != want {
		t.Errorf("StripPromoLines mismatch.\nin:\n%s\ngot:\n%s\nwant:\n%s", in, got, want)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: FAIL — `undefined: StripPromoLines`.

- [ ] **Step 3: Implement**

Append to `cleaner.go`:

```go
// StripPromoLines removes whole lines that look like ad/promo references to source sites.
func StripPromoLines(text string) string {
	promo := regexp.MustCompile(`(?i)^.*(read at|visit|please support|original at)\b.*$`)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if promo.MatchString(strings.TrimSpace(line)) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/cleaner/cleaner.go services/ingestion/internal/cleaner/cleaner_test.go
git commit -m "feat(cleaner): drop promo / cross-site reference lines"
```

---

### Task 6: Cleaner — FullCleanChapter pipeline (TDD)

**Files:**
- Modify: `services/ingestion/internal/cleaner/cleaner_test.go`
- Modify: `services/ingestion/internal/cleaner/cleaner.go`

- [ ] **Step 1: Add failing test**

Append to `cleaner_test.go`:

```go
func TestFullCleanChapter(t *testing.T) {
	raw := "<p>Chapter 5: Awakening</p>\n<p>He woke up. [T/N: literal]</p>\n<p>Read at example.com.</p>\n<p>It was bright.</p>"
	got, err := FullCleanChapter(raw)
	if err != nil {
		t.Fatalf("FullCleanChapter err: %v", err)
	}
	want := "He woke up. It was bright."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: FAIL — `undefined: FullCleanChapter`.

- [ ] **Step 3: Implement pipeline**

Append to `cleaner.go`:

```go
// FullCleanChapter runs the full chapter-cleaning pipeline:
// header strip → translator notes strip → promo strip → HTML strip → ZWNJ/ZWJ/ZWSP strip → whitespace normalize.
// Order matters: strippers that match patterns in raw HTML run before HTML tag removal.
func FullCleanChapter(raw string) (string, error) {
	stripped := StripChapterHeader(raw)
	stripped = StripTranslatorNotes(stripped)
	stripped = StripPromoLines(stripped)
	return CleanHTMLPipeline(stripped)
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/cleaner/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/cleaner/cleaner.go services/ingestion/internal/cleaner/cleaner_test.go
git commit -m "feat(cleaner): add FullCleanChapter pipeline"
```

---

### Task 7: Repo — ListNovels + GetNovel (TDD, integration)

**Files:**
- Create: `services/ingestion/internal/repository/postgres_test.go`
- Modify: `services/ingestion/internal/repository/postgres.go`

- [ ] **Step 1: Write failing integration test**

Create `services/ingestion/internal/repository/postgres_test.go`:

```go
package repository

import (
	"context"
	"os"
	"testing"

	"github.com/poompich/novel-translator/services/ingestion/internal/domain"
)

func newTestRepo(t *testing.T) *PostgresRepo {
	t.Helper()
	url := os.Getenv("TEST_DB_URL")
	if url == "" {
		t.Skip("TEST_DB_URL not set; skipping integration test")
	}
	repo, err := NewPostgresRepo(context.Background(), url)
	if err != nil {
		t.Fatalf("NewPostgresRepo: %v", err)
	}
	t.Cleanup(repo.Close)
	return repo
}

func TestListAndGetNovel(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	id, err := repo.UpsertNovel(ctx, domain.Novel{
		Title: "Test Novel A", SourceURL: "https://example.test/a", SourceLang: "en", TargetLang: "th",
		Status: domain.StatusPending,
	})
	if err != nil {
		t.Fatalf("UpsertNovel: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.db.Exec(ctx, "DELETE FROM novels WHERE id=$1", id) })

	got, err := repo.GetNovel(ctx, id)
	if err != nil {
		t.Fatalf("GetNovel: %v", err)
	}
	if got.Title != "Test Novel A" {
		t.Errorf("got title %q", got.Title)
	}

	list, err := repo.ListNovels(ctx)
	if err != nil {
		t.Fatalf("ListNovels: %v", err)
	}
	found := false
	for _, n := range list {
		if n.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListNovels did not return inserted novel %d", id)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && TEST_DB_URL=postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable go test ./internal/repository/...`
Expected: FAIL — `undefined: GetNovel` and `undefined: ListNovels`. (Make sure `make up` ran first.)

- [ ] **Step 3: Implement methods**

Append to `services/ingestion/internal/repository/postgres.go`:

```go
func (r *PostgresRepo) GetNovel(ctx context.Context, id int) (domain.Novel, error) {
	var n domain.Novel
	err := r.db.QueryRow(ctx, `
		SELECT id, title, source_url, source_lang, target_lang, status, created_at, updated_at
		FROM novels WHERE id = $1
	`, id).Scan(&n.ID, &n.Title, &n.SourceURL, &n.SourceLang, &n.TargetLang, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return domain.Novel{}, fmt.Errorf("get novel %d: %w", id, err)
	}
	return n, nil
}

func (r *PostgresRepo) ListNovels(ctx context.Context) ([]domain.Novel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, source_url, source_lang, target_lang, status, created_at, updated_at
		FROM novels ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list novels: %w", err)
	}
	defer rows.Close()
	var out []domain.Novel
	for rows.Next() {
		var n domain.Novel
		if err := rows.Scan(&n.ID, &n.Title, &n.SourceURL, &n.SourceLang, &n.TargetLang, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan novel: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && TEST_DB_URL=postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable go test ./internal/repository/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/repository/postgres.go services/ingestion/internal/repository/postgres_test.go
git commit -m "feat(repo): add GetNovel + ListNovels with integration tests"
```

---

### Task 8: Repo — ListChapters + GetNextChapterNumber (TDD, integration)

**Files:**
- Modify: `services/ingestion/internal/repository/postgres_test.go`
- Modify: `services/ingestion/internal/repository/postgres.go`

- [ ] **Step 1: Add failing test**

Append to `postgres_test.go`:

```go
func TestListChaptersAndNextNumber(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	novelID, err := repo.UpsertNovel(ctx, domain.Novel{
		Title: "Test Novel B", SourceURL: "https://example.test/b", SourceLang: "en", TargetLang: "th",
		Status: domain.StatusPending,
	})
	if err != nil {
		t.Fatalf("UpsertNovel: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.db.Exec(ctx, "DELETE FROM novels WHERE id=$1", novelID) })

	next, err := repo.GetNextChapterNumber(ctx, novelID)
	if err != nil {
		t.Fatalf("GetNextChapterNumber empty: %v", err)
	}
	if next != 1 {
		t.Errorf("expected next=1 for empty novel, got %d", next)
	}

	for _, n := range []int{1, 2, 3} {
		if _, err := repo.UpsertChapter(ctx, domain.Chapter{
			NovelID: novelID, ChapterNumber: n, Title: "ch", RawContent: "raw", CleanedContent: "clean",
			SourceURL: "https://example.test/b/" + string(rune('0'+n)), ScrapeStatus: domain.StatusCompleted,
		}); err != nil {
			t.Fatalf("UpsertChapter %d: %v", n, err)
		}
	}

	next, err = repo.GetNextChapterNumber(ctx, novelID)
	if err != nil {
		t.Fatalf("GetNextChapterNumber after seed: %v", err)
	}
	if next != 4 {
		t.Errorf("expected next=4, got %d", next)
	}

	list, err := repo.ListChapters(ctx, novelID)
	if err != nil {
		t.Fatalf("ListChapters: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 chapters, got %d", len(list))
	}
	if list[0].ChapterNumber != 1 || list[2].ChapterNumber != 3 {
		t.Errorf("chapters out of order: %+v", list)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && TEST_DB_URL=postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable go test ./internal/repository/...`
Expected: FAIL — `undefined: GetNextChapterNumber`, `undefined: ListChapters`.

- [ ] **Step 3: Implement methods**

Append to `postgres.go`:

```go
func (r *PostgresRepo) GetNextChapterNumber(ctx context.Context, novelID int) (int, error) {
	var maxN *int
	err := r.db.QueryRow(ctx, `SELECT MAX(chapter_number) FROM chapters WHERE novel_id = $1`, novelID).Scan(&maxN)
	if err != nil {
		return 0, fmt.Errorf("max chapter for novel %d: %w", novelID, err)
	}
	if maxN == nil {
		return 1, nil
	}
	return *maxN + 1, nil
}

func (r *PostgresRepo) ListChapters(ctx context.Context, novelID int) ([]domain.Chapter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, novel_id, chapter_number, title, source_url, scrape_status, created_at, updated_at
		FROM chapters WHERE novel_id = $1 ORDER BY chapter_number ASC
	`, novelID)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	defer rows.Close()
	var out []domain.Chapter
	for rows.Next() {
		var c domain.Chapter
		if err := rows.Scan(&c.ID, &c.NovelID, &c.ChapterNumber, &c.Title, &c.SourceURL, &c.ScrapeStatus, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chapter: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
```

Note: `ListChapters` deliberately omits `raw_content` / `cleaned_content` to keep the list payload small — handlers fetch full content via a future `GetChapter` if needed (out of scope this week).

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && TEST_DB_URL=postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable go test ./internal/repository/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/repository/postgres.go services/ingestion/internal/repository/postgres_test.go
git commit -m "feat(repo): add ListChapters + GetNextChapterNumber"
```

---

### Task 9: Server skeleton + basic-auth middleware (TDD)

**Files:**
- Create: `services/ingestion/internal/server/server.go`
- Create: `services/ingestion/internal/server/server_test.go`

- [ ] **Step 1: Write failing test**

Create `services/ingestion/internal/server/server_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuthMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := basicAuth("secret", next)

	t.Run("rejects missing auth", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
		if called {
			t.Error("next called despite missing auth")
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("operator", "wrong")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("accepts correct password", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("operator", "secret")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !called {
			t.Error("next not called")
		}
	})
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: FAIL — package or `basicAuth` not defined.

- [ ] **Step 3: Implement server skeleton**

Create `services/ingestion/internal/server/server.go`:

```go
package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Run boots the HTTP server and blocks until ctx is canceled.
func Run(ctx context.Context, h *Handlers, addr, password string) error {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler { return basicAuth(password, next) })

		r.Get("/", h.PastePage)
		r.Get("/api/novels", h.ListNovels)
		r.Post("/api/novels", h.CreateNovel)
		r.Get("/api/chapters", h.ListChapters)
		r.Post("/api/chapters", h.CreateChapter)
	})

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("paste-to-db server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

func basicAuth(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pwd, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pwd), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="paste-to-db"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

This file references `*Handlers` which doesn't exist yet — it'll be created in Task 10 before we wire the rest. The test only exercises `basicAuth`, so compilation only needs the middleware. Move the `Run` function to a separate file in Task 11 if needed; for now keep `Run` here but **do not yet add it to package compilation by importing it from main**.

To keep this task atomic, replace the body of `server.go` for now with just the basicAuth function and add `Run` later. Use this minimal file instead:

```go
package server

import (
	"crypto/subtle"
	"net/http"
)

func basicAuth(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pwd, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pwd), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="paste-to-db"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

The chi router + `Run` get added in Task 16.

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: PASS, all 3 subtests green.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/server/server.go services/ingestion/internal/server/server_test.go
git commit -m "feat(server): add basic-auth middleware"
```

---

### Task 10: Handlers struct + Repo interface + fake repo (TDD)

**Files:**
- Create: `services/ingestion/internal/server/handlers.go`
- Create: `services/ingestion/internal/server/handlers_test.go`

- [ ] **Step 1: Write failing test**

Create `services/ingestion/internal/server/handlers_test.go`:

```go
package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/poompich/novel-translator/services/ingestion/internal/domain"
)

type fakeRepo struct {
	novels   []domain.Novel
	chapters map[int][]domain.Chapter
	nextID   int
	upsertCh func(context.Context, domain.Chapter) (int, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{chapters: map[int][]domain.Chapter{}, nextID: 1}
}

func (f *fakeRepo) UpsertNovel(ctx context.Context, n domain.Novel) (int, error) {
	n.ID = f.nextID
	f.nextID++
	f.novels = append(f.novels, n)
	return n.ID, nil
}
func (f *fakeRepo) GetNovel(ctx context.Context, id int) (domain.Novel, error) {
	for _, n := range f.novels {
		if n.ID == id {
			return n, nil
		}
	}
	return domain.Novel{}, errors.New("not found")
}
func (f *fakeRepo) ListNovels(ctx context.Context) ([]domain.Novel, error) { return f.novels, nil }
func (f *fakeRepo) UpsertChapter(ctx context.Context, c domain.Chapter) (int, error) {
	if f.upsertCh != nil {
		return f.upsertCh(ctx, c)
	}
	c.ID = f.nextID
	f.nextID++
	f.chapters[c.NovelID] = append(f.chapters[c.NovelID], c)
	return c.ID, nil
}
func (f *fakeRepo) ListChapters(ctx context.Context, novelID int) ([]domain.Chapter, error) {
	return f.chapters[novelID], nil
}
func (f *fakeRepo) GetNextChapterNumber(ctx context.Context, novelID int) (int, error) {
	cs := f.chapters[novelID]
	if len(cs) == 0 {
		return 1, nil
	}
	max := 0
	for _, c := range cs {
		if c.ChapterNumber > max {
			max = c.ChapterNumber
		}
	}
	return max + 1, nil
}

func TestCreateNovelHandler(t *testing.T) {
	repo := newFakeRepo()
	h := &Handlers{Repo: repo}

	body := strings.NewReader(`title=My+Novel&source_url=https%3A%2F%2Fex.test%2Fn&source_lang=en&target_lang=th`)
	req := httptest.NewRequest(http.MethodPost, "/api/novels", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.CreateNovel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(repo.novels) != 1 {
		t.Fatalf("expected 1 novel saved, got %d", len(repo.novels))
	}
	if repo.novels[0].Title != "My Novel" {
		t.Errorf("got title %q", repo.novels[0].Title)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: FAIL — `undefined: Handlers`, `Repo`, `CreateNovel`.

- [ ] **Step 3: Implement handlers + Repo interface**

Create `services/ingestion/internal/server/handlers.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/poompich/novel-translator/services/ingestion/internal/cleaner"
	"github.com/poompich/novel-translator/services/ingestion/internal/domain"
)

// Repo is the subset of repository.PostgresRepo the server depends on.
type Repo interface {
	UpsertNovel(ctx context.Context, n domain.Novel) (int, error)
	GetNovel(ctx context.Context, id int) (domain.Novel, error)
	ListNovels(ctx context.Context) ([]domain.Novel, error)
	UpsertChapter(ctx context.Context, c domain.Chapter) (int, error)
	ListChapters(ctx context.Context, novelID int) ([]domain.Chapter, error)
	GetNextChapterNumber(ctx context.Context, novelID int) (int, error)
}

type Handlers struct {
	Repo Repo
}

func (h *Handlers) CreateNovel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	sourceURL := r.FormValue("source_url")
	if title == "" || sourceURL == "" {
		http.Error(w, "title and source_url required", http.StatusBadRequest)
		return
	}
	srcLang := r.FormValue("source_lang")
	if srcLang == "" {
		srcLang = "en"
	}
	dstLang := r.FormValue("target_lang")
	if dstLang == "" {
		dstLang = "th"
	}
	id, err := h.Repo.UpsertNovel(r.Context(), domain.Novel{
		Title: title, SourceURL: sourceURL, SourceLang: srcLang, TargetLang: dstLang,
		Status: domain.StatusPending,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "title": title})
}

// stubs filled in by later tasks
func (h *Handlers) PastePage(w http.ResponseWriter, r *http.Request)     { http.Error(w, "not yet", http.StatusNotImplemented) }
func (h *Handlers) ListNovels(w http.ResponseWriter, r *http.Request)    { http.Error(w, "not yet", http.StatusNotImplemented) }
func (h *Handlers) ListChapters(w http.ResponseWriter, r *http.Request)  { http.Error(w, "not yet", http.StatusNotImplemented) }
func (h *Handlers) CreateChapter(w http.ResponseWriter, r *http.Request) { http.Error(w, "not yet", http.StatusNotImplemented) }

// helpers used by later handlers
func parseIntForm(r *http.Request, key string) (int, error) {
	v := r.FormValue(key)
	if v == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	return strconv.Atoi(v)
}

var _ = cleaner.FullCleanChapter // referenced by CreateChapter in Task 13
```

The `var _ = cleaner.FullCleanChapter` line keeps the `cleaner` import alive so subsequent tasks compile incrementally; remove the line in Task 13 once `CreateChapter` actually uses it.

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/server/handlers.go services/ingestion/internal/server/handlers_test.go
git commit -m "feat(server): handlers skeleton with CreateNovel + Repo interface"
```

---

### Task 11: Templates + paste page (TDD)

**Files:**
- Create: `services/ingestion/internal/server/templates.go`
- Create: `services/ingestion/internal/server/templates/layout.html`
- Create: `services/ingestion/internal/server/templates/paste.html`
- Modify: `services/ingestion/internal/server/handlers.go`
- Modify: `services/ingestion/internal/server/handlers_test.go`

- [ ] **Step 1: Write failing test**

Append to `handlers_test.go`:

```go
func TestPastePage(t *testing.T) {
	repo := newFakeRepo()
	_, _ = repo.UpsertNovel(context.Background(), domain.Novel{
		Title: "Existing", SourceURL: "https://ex.test/e", SourceLang: "en", TargetLang: "th", Status: domain.StatusPending,
	})
	h := &Handlers{Repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.PastePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"Paste-to-DB", "Existing", "Raw content", `name="raw_content"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: FAIL — `PastePage` returns `not yet` 501.

- [ ] **Step 3: Create templates**

Create `services/ingestion/internal/server/templates/layout.html`:

```html
{{define "layout"}}<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.Title}} - Paste-to-DB</title>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-50 text-slate-800">
  <div class="max-w-4xl mx-auto p-6">
    <h1 class="text-2xl font-semibold mb-6">Paste-to-DB</h1>
    {{template "content" .}}
  </div>
</body>
</html>{{end}}
```

Create `services/ingestion/internal/server/templates/paste.html`:

```html
{{define "content"}}
<form hx-post="/api/chapters" hx-target="#result" hx-swap="innerHTML"
      class="space-y-4 bg-white p-6 rounded shadow">
  <div>
    <label class="block text-sm font-medium">Novel</label>
    <select name="novel_id" class="mt-1 block w-full border rounded p-2">
      <option value="">— New Novel —</option>
      {{range .Novels}}
        <option value="{{.ID}}">{{.Title}}</option>
      {{end}}
    </select>
  </div>

  <fieldset class="border p-3 rounded">
    <legend class="text-sm font-medium px-2">If new novel</legend>
    <input type="text" name="title" placeholder="Title"
           class="mt-1 block w-full border rounded p-2">
    <input type="text" name="source_url" placeholder="Source URL (e.g., site/novel/slug)"
           class="mt-2 block w-full border rounded p-2">
    <div class="flex gap-2 mt-2">
      <input type="text" name="source_lang" value="en" class="border rounded p-2 w-24">
      <input type="text" name="target_lang" value="th" class="border rounded p-2 w-24">
    </div>
  </fieldset>

  <div class="grid grid-cols-3 gap-2">
    <input type="number" name="chapter_number" placeholder="Chapter #"
           class="border rounded p-2">
    <input type="text" name="chapter_title" placeholder="Chapter title"
           class="border rounded p-2 col-span-2">
  </div>
  <input type="text" name="chapter_source_url" placeholder="Chapter source URL"
         class="border rounded p-2 w-full">

  <textarea name="raw_content" rows="20" placeholder="Raw content (paste here)"
            class="block w-full border rounded p-2 font-mono text-sm"></textarea>

  <div class="flex items-center gap-4">
    <label class="text-sm"><input type="checkbox" name="auto_clean" checked> Auto-clean</label>
    <button class="bg-slate-800 text-white px-4 py-2 rounded">Save</button>
  </div>
</form>
<div id="result" class="mt-6"></div>
{{end}}
```

Create `services/ingestion/internal/server/templates.go`:

```go
package server

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/*.html
var templatesFS embed.FS

func renderPage(w io.Writer, page string, data any) error {
	tpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/"+page)
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	return tpl.ExecuteTemplate(w, "layout", data)
}

func renderPartial(w io.Writer, partial string, data any) error {
	tpl, err := template.ParseFS(templatesFS, "templates/"+partial)
	if err != nil {
		return fmt.Errorf("parse partial: %w", err)
	}
	return tpl.Execute(w, data)
}
```

- [ ] **Step 4: Wire PastePage**

Replace the `PastePage` stub in `handlers.go`:

```go
func (h *Handlers) PastePage(w http.ResponseWriter, r *http.Request) {
	novels, err := h.Repo.ListNovels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderPage(w, "paste.html", map[string]any{
		"Title":  "Paste",
		"Novels": novels,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

- [ ] **Step 5: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/ingestion/internal/server/templates.go services/ingestion/internal/server/templates/ services/ingestion/internal/server/handlers.go services/ingestion/internal/server/handlers_test.go
git commit -m "feat(server): paste page rendered with embedded HTMX templates"
```

---

### Task 12: GET /api/novels — JSON list (TDD)

**Files:**
- Modify: `services/ingestion/internal/server/handlers.go`
- Modify: `services/ingestion/internal/server/handlers_test.go`

- [ ] **Step 1: Add failing test**

Append to `handlers_test.go`:

```go
func TestListNovelsHandler(t *testing.T) {
	repo := newFakeRepo()
	_, _ = repo.UpsertNovel(context.Background(), domain.Novel{Title: "A", SourceURL: "u1", SourceLang: "en", TargetLang: "th"})
	_, _ = repo.UpsertNovel(context.Background(), domain.Novel{Title: "B", SourceURL: "u2", SourceLang: "en", TargetLang: "th"})
	h := &Handlers{Repo: repo}

	req := httptest.NewRequest(http.MethodGet, "/api/novels", nil)
	rr := httptest.NewRecorder()
	h.ListNovels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"title":"A"`) {
		t.Errorf("expected JSON with novel A, got %s", rr.Body.String())
	}
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: FAIL — handler returns 501.

- [ ] **Step 3: Implement**

Replace the `ListNovels` stub:

```go
func (h *Handlers) ListNovels(w http.ResponseWriter, r *http.Request) {
	novels, err := h.Repo.ListNovels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(novels)
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/server/handlers.go services/ingestion/internal/server/handlers_test.go
git commit -m "feat(server): GET /api/novels JSON list"
```

---

### Task 13: POST /api/chapters — paste + clean + upsert (TDD)

**Files:**
- Modify: `services/ingestion/internal/server/handlers.go`
- Modify: `services/ingestion/internal/server/handlers_test.go`
- Create: `services/ingestion/internal/server/templates/chapter_saved.html`

- [ ] **Step 1: Add failing test**

Append to `handlers_test.go`:

```go
func TestCreateChapterCleansAndUpserts(t *testing.T) {
	repo := newFakeRepo()
	novelID, _ := repo.UpsertNovel(context.Background(), domain.Novel{
		Title: "N", SourceURL: "u", SourceLang: "en", TargetLang: "th",
	})
	h := &Handlers{Repo: repo}

	form := strings.NewReader(
		"novel_id=" + strconv.Itoa(novelID) +
			"&chapter_number=1&chapter_title=Awakening" +
			"&chapter_source_url=https%3A%2F%2Fex.test%2Fn%2F1" +
			"&raw_content=" + url.QueryEscape("<p>Chapter 1: Awakening</p><p>He woke up. [T/N: literal]</p><p>Read at example.com.</p><p>It was bright.</p>") +
			"&auto_clean=on",
	)
	req := httptest.NewRequest(http.MethodPost, "/api/chapters", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.CreateChapter(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	chs := repo.chapters[novelID]
	if len(chs) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chs))
	}
	if chs[0].CleanedContent != "He woke up. It was bright." {
		t.Errorf("cleaned_content = %q", chs[0].CleanedContent)
	}
	if !strings.Contains(rr.Body.String(), "Saved chapter 1") {
		t.Errorf("response missing confirmation: %s", rr.Body.String())
	}
}
```

Add imports `net/url` and `strconv` to `handlers_test.go`.

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: FAIL — handler returns 501.

- [ ] **Step 3: Create confirmation partial**

Create `services/ingestion/internal/server/templates/chapter_saved.html`:

```html
<div class="bg-green-50 border border-green-300 p-4 rounded">
  <p class="text-green-800 font-medium">Saved chapter {{.ChapterNumber}} (id={{.ChapterID}}) for novel id={{.NovelID}}.</p>
  <p class="text-sm text-slate-600 mt-1">Cleaned length: {{.CleanedLen}} chars.</p>
</div>
```

- [ ] **Step 4: Implement CreateChapter**

Replace the `CreateChapter` stub and remove the `var _ = cleaner.FullCleanChapter` shim:

```go
func (h *Handlers) CreateChapter(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	novelID, err := parseIntForm(r, "novel_id")
	if err != nil {
		http.Error(w, "novel_id required", http.StatusBadRequest)
		return
	}
	chapterNumber, err := parseIntForm(r, "chapter_number")
	if err != nil {
		http.Error(w, "chapter_number required", http.StatusBadRequest)
		return
	}
	raw := r.FormValue("raw_content")
	if raw == "" {
		http.Error(w, "raw_content required", http.StatusBadRequest)
		return
	}

	cleaned := raw
	if r.FormValue("auto_clean") == "on" {
		c, err := cleaner.FullCleanChapter(raw)
		if err != nil {
			http.Error(w, "clean failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		cleaned = c
	}

	id, err := h.Repo.UpsertChapter(r.Context(), domain.Chapter{
		NovelID:        novelID,
		ChapterNumber:  chapterNumber,
		Title:          r.FormValue("chapter_title"),
		RawContent:     raw,
		CleanedContent: cleaned,
		SourceURL:      r.FormValue("chapter_source_url"),
		ScrapeStatus:   domain.StatusCompleted,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderPartial(w, "chapter_saved.html", map[string]any{
		"NovelID":       novelID,
		"ChapterID":     id,
		"ChapterNumber": chapterNumber,
		"CleanedLen":    len(cleaned),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

Remove the old `var _ = cleaner.FullCleanChapter` line at the bottom of `handlers.go`.

- [ ] **Step 5: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/ingestion/internal/server/handlers.go services/ingestion/internal/server/handlers_test.go services/ingestion/internal/server/templates/chapter_saved.html
git commit -m "feat(server): POST /api/chapters cleans + upserts + returns HTMX confirmation"
```

---

### Task 14: GET /api/chapters — list per novel (TDD)

**Files:**
- Modify: `services/ingestion/internal/server/handlers.go`
- Modify: `services/ingestion/internal/server/handlers_test.go`

- [ ] **Step 1: Add failing test**

Append to `handlers_test.go`:

```go
func TestListChaptersHandler(t *testing.T) {
	repo := newFakeRepo()
	novelID, _ := repo.UpsertNovel(context.Background(), domain.Novel{Title: "X", SourceURL: "x"})
	for n := 1; n <= 2; n++ {
		_, _ = repo.UpsertChapter(context.Background(), domain.Chapter{
			NovelID: novelID, ChapterNumber: n, Title: "t", ScrapeStatus: domain.StatusCompleted,
		})
	}
	h := &Handlers{Repo: repo}

	t.Run("missing novel_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/chapters", nil)
		rr := httptest.NewRecorder()
		h.ListChapters(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns chapters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/chapters?novel_id="+strconv.Itoa(novelID), nil)
		rr := httptest.NewRecorder()
		h.ListChapters(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, `"chapter_number":1`) || !strings.Contains(body, `"chapter_number":2`) {
			t.Errorf("unexpected body: %s", body)
		}
	})
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: FAIL — handler returns 501.

- [ ] **Step 3: Implement**

Replace the `ListChapters` stub:

```go
func (h *Handlers) ListChapters(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("novel_id")
	if idStr == "" {
		http.Error(w, "novel_id required", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "novel_id must be int", http.StatusBadRequest)
		return
	}
	chs, err := h.Repo.ListChapters(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chs)
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `cd services/ingestion && go test ./internal/server/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/internal/server/handlers.go services/ingestion/internal/server/handlers_test.go
git commit -m "feat(server): GET /api/chapters JSON list per novel"
```

---

### Task 15: Wire chi router + Run + main.go

**Files:**
- Modify: `services/ingestion/internal/server/server.go`
- Modify: `services/ingestion/cmd/main.go`

- [ ] **Step 1: Replace server.go with full version**

Replace the contents of `services/ingestion/internal/server/server.go` with:

```go
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Run(ctx context.Context, h *Handlers, addr, password string) error {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler { return basicAuth(password, next) })

		r.Get("/", h.PastePage)
		r.Get("/api/novels", h.ListNovels)
		r.Post("/api/novels", h.CreateNovel)
		r.Get("/api/chapters", h.ListChapters)
		r.Post("/api/chapters", h.CreateChapter)
	})

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5e9)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("paste-to-db server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

func basicAuth(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pwd, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pwd), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="paste-to-db"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Wire serve in main.go**

Replace `services/ingestion/cmd/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/poompich/novel-translator/services/ingestion/internal/config"
	"github.com/poompich/novel-translator/services/ingestion/internal/repository"
	"github.com/poompich/novel-translator/services/ingestion/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ingestion-service <command>")
		fmt.Println("Commands: serve, translate")
		os.Exit(1)
	}

	cmd := os.Args[1]

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "../../config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("Warning: failed to load config (%v); using defaults.", err)
		cfg = &config.Config{}
		if envDB := os.Getenv("DB_URL"); envDB != "" {
			cfg.Database.URL = envDB
		} else {
			cfg.Database.URL = "postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable"
		}
		cfg.Server.Addr = ":8080"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	repo, err := repository.NewPostgresRepo(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}
	defer repo.Close()

	switch cmd {
	case "serve":
		if cfg.Server.Password == "" {
			log.Fatal("server.password not set in config or SERVER_PASSWORD env")
		}
		h := &server.Handlers{Repo: repo}
		if err := server.Run(ctx, h, cfg.Server.Addr, cfg.Server.Password); err != nil {
			log.Fatalf("serve: %v", err)
		}
	case "translate":
		log.Println("translate: translation worker not implemented yet (Phase 1, Week 3)")
		os.Exit(2)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build verification**

Run: `cd services/ingestion && go build ./...`
Expected: clean build, no errors.

Run: `cd services/ingestion && go test ./...` (with `TEST_DB_URL` set if you want repo tests too)
Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add services/ingestion/internal/server/server.go services/ingestion/cmd/main.go
git commit -m "feat(server): wire chi router + serve cmd"
```

---

### Task 16: Manual smoke test

- [ ] **Step 1: Bring DB up + build**

Run: `make up && make build`
Expected: `db` container healthy, `bin/ingestion-service` exists.

- [ ] **Step 2: Run server**

Run: `SERVER_PASSWORD=local-pass make serve`
Expected: log line `paste-to-db server listening on :8080`.

- [ ] **Step 3: Hit the paste page**

Open `http://localhost:8080/` in a browser. Browser prompts for basic auth:
- user: anything (e.g. `operator`)
- pass: `local-pass`

Expected: paste form renders.

- [ ] **Step 4: Create a novel + chapter via curl**

```bash
curl -u operator:local-pass -X POST http://localhost:8080/api/novels \
  -d 'title=Smoke%20Test%20Novel&source_url=https://ex.test/smoke&source_lang=en&target_lang=th'
# response: {"id":N,"title":"Smoke Test Novel"}

curl -u operator:local-pass -X POST http://localhost:8080/api/chapters \
  -d 'novel_id=N&chapter_number=1&chapter_title=Hello&chapter_source_url=https://ex.test/smoke/1&auto_clean=on&raw_content=<p>Chapter%201</p><p>He%20woke%20up.</p>'
# response: HTML "Saved chapter 1 ..."
```

Replace `N` with the id returned by the first call. Expected: 200 responses, success messages in body.

- [ ] **Step 5: Verify in DB**

```bash
docker exec -it novel-translator-db psql -U translator -d novel_translator \
  -c "SELECT id, novel_id, chapter_number, length(cleaned_content) FROM chapters ORDER BY id DESC LIMIT 5;"
```

Expected: row exists with non-zero `cleaned_content` length.

- [ ] **Step 6: Stop server**

`Ctrl-C` in the `make serve` terminal. Server logs shutdown then exits 0.

---

## Self-Review Checklist

Before declaring Week 1 done, walk this list:

- [ ] All 16 tasks committed individually.
- [ ] `go build ./...` clean.
- [ ] `go test ./...` (without `TEST_DB_URL`) green — cleaner + server tests run.
- [ ] `TEST_DB_URL=... go test ./internal/repository/...` green.
- [ ] Smoke test (Task 16) end-to-end pass.
- [ ] `CLAUDE.md` "Common commands" section still accurate (the `make serve` line no longer mentions STUB — update it as part of Task 16 if you forgot).
- [ ] No leftover scaffolding (`var _ = cleaner.FullCleanChapter`, `// TODO`, etc.).

## Out of Scope Week 1

- Edit / delete endpoints for chapters (operator can re-paste; upsert overwrites).
- Multi-chapter auto-split helper from the spec UI mock — defer to Week 1.5 if operator hits the manual-paste-of-batched-chapters case in practice.
- Review UI (`GET /review`) — Week 5.
- Reviewer-side authentication beyond shared password — Week 5.
- HTTPS / TLS — handled by reverse proxy / Cloudflare in deploy, not in app.
