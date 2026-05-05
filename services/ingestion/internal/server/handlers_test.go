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
