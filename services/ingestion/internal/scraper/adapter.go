package scraper

import (
	"context"
)

// ChapterMeta represents lightweight chapter metadata from an index page
type ChapterMeta struct {
	ChapterNumber int
	Title         string
	URL           string
}

// ScrapeResult represents the data extracted from a page
type ScrapeResult struct {
	Title    string
	Content  string
	Chapters []ChapterMeta
}

// SiteAdapter defines how to extract data
type SiteAdapter interface {
	Name() string
	MatchesURL(url string) bool
	Scrape(ctx context.Context, url string) (*ScrapeResult, error)
}
