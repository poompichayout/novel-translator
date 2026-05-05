package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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
