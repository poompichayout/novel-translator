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
