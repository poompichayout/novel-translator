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
func (h *Handlers) PastePage(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}
func (h *Handlers) ListNovels(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}
func (h *Handlers) ListChapters(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}
func (h *Handlers) CreateChapter(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}

func parseIntForm(r *http.Request, key string) (int, error) {
	v := r.FormValue(key)
	if v == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	return strconv.Atoi(v)
}

var _ = cleaner.FullCleanChapter // referenced by CreateChapter in Task 13
