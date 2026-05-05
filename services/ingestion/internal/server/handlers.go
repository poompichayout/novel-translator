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

// stubs filled in by later tasks
func (h *Handlers) ListNovels(w http.ResponseWriter, r *http.Request) {
	novels, err := h.Repo.ListNovels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(novels)
}
func (h *Handlers) ListChapters(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}
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

func parseIntForm(r *http.Request, key string) (int, error) {
	v := r.FormValue(key)
	if v == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	return strconv.Atoi(v)
}

