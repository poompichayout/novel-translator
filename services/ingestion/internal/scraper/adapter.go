package scraper

import (
	"github.com/PuerkitoBio/goquery"
)

// ChapterMeta represents lightweight chapter metadata from an index page
type ChapterMeta struct {
	ChapterNumber int
	Title         string
	URL           string
}

// SiteAdapter defines how to extract data from a specific novel site
type SiteAdapter interface {
	Name() string
	MatchesURL(url string) bool
	ExtractChapterList(doc *goquery.Document) ([]ChapterMeta, error)
	ExtractChapterContent(doc *goquery.Document) (string, error)
	NeedsJSRendering() bool
}
