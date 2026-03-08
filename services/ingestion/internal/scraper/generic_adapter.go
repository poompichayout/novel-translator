package scraper

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// GenericAdapter is a configurable adapter for basic sites.
type GenericAdapter struct {
	IndexSelector     string
	ContentSelector   string
	NextPageSelector  string
	TitleSelector     string
	RequiresJS        bool
}

func (g *GenericAdapter) Name() string {
	return "GenericAdapter"
}

func (g *GenericAdapter) MatchesURL(url string) bool {
	return true // Fallback adapter
}

func (g *GenericAdapter) ExtractChapterList(doc *goquery.Document) ([]ChapterMeta, error) {
	var chapters []ChapterMeta
	
	// Try to find chapter links
	doc.Find(g.IndexSelector).Each(func(i int, s *goquery.Selection) {
		link, exists := s.Attr("href")
		if exists {
			title := strings.TrimSpace(s.Text())
			// Attempt to extract chapter number from title or URL
			chapNum := extractNumber(title)
			if chapNum == 0 {
				chapNum = i + 1 // Fallback
			}

			chapters = append(chapters, ChapterMeta{
				ChapterNumber: chapNum,
				Title:         title,
				URL:           link, // Note: might need absolute URL resolution in caller
			})
		}
	})

	return chapters, nil
}

func (g *GenericAdapter) ExtractChapterContent(doc *goquery.Document) (string, error) {
	selection := doc.Find(g.ContentSelector)
	
	if selection.Length() == 0 {
		return "", errors.New("content selector found no matches")
	}

	html, err := selection.Html()
	if err != nil {
		return "", fmt.Errorf("failed to extract html: %w", err)
	}

	return html, nil
}

func (g *GenericAdapter) NeedsJSRendering() bool {
	return g.RequiresJS
}

func extractNumber(s string) int {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(s)
	if match != "" {
		if num, err := strconv.Atoi(match); err == nil {
			return num
		}
	}
	return 0
}
