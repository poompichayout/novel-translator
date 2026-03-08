package domain

import (
	"time"
)

type ProcessStatus string

const (
	StatusPending    ProcessStatus = "pending"
	StatusInProgress ProcessStatus = "in_progress"
	StatusCompleted  ProcessStatus = "completed"
	StatusFailed     ProcessStatus = "failed"
)

type EntityType string

const (
	EntityCharacter EntityType = "character"
	EntityPlace     EntityType = "place"
	EntityTerm      EntityType = "term"
	EntityItem      EntityType = "item"
	EntityOther     EntityType = "other"
)

type Novel struct {
	ID         int           `json:"id"`
	Title      string        `json:"title"`
	SourceURL  string        `json:"source_url"`
	SourceLang string        `json:"source_lang"`
	TargetLang string        `json:"target_lang"`
	Status     ProcessStatus `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type Chapter struct {
	ID             int           `json:"id"`
	NovelID        int           `json:"novel_id"`
	ChapterNumber  int           `json:"chapter_number"`
	Title          string        `json:"title"`
	RawContent     string        `json:"raw_content,omitempty"`
	CleanedContent string        `json:"cleaned_content,omitempty"`
	SourceURL      string        `json:"source_url"`
	ScrapeStatus   ProcessStatus `json:"scrape_status"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type Entity struct {
	ID          int        `json:"id"`
	NovelID     int        `json:"novel_id"`
	NameEn      string     `json:"name_en"`
	NameTh      string     `json:"name_th"`
	Type        EntityType `json:"type"`
	Aliases     []string   `json:"aliases"`
	Description string     `json:"description,omitempty"`
	Embedding   []float32  `json:"embedding,omitempty"` // 768-dim LaBSE vector
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TranslationPair represents an aligned sentence pair for RAG / fine-tuning.
type TranslationPair struct {
	ID              int       `json:"id"`
	ChapterID       int       `json:"chapter_id"`
	SentenceEn      string    `json:"sentence_en"`
	SentenceTh      string    `json:"sentence_th"`
	SimilarityScore float64   `json:"similarity_score"`
	IsValidated     bool      `json:"is_validated"`
	CreatedAt       time.Time `json:"created_at"`
}

type ScrapeJob struct {
	ID           int           `json:"id"`
	NovelID      int           `json:"novel_id"`
	Status       ProcessStatus `json:"status"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
}
