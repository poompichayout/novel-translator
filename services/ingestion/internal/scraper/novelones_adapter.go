package scraper

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type NovelOnesAdapter struct {
	GenericAdapter
}

func NewNovelOnesAdapter() *NovelOnesAdapter {
	return &NovelOnesAdapter{
		GenericAdapter: GenericAdapter{
			IndexSelector:   ".chapter-list a, .chapter-item a",
			ContentSelector: ".chapter-content",
			RequiresJS:      true, // Cloudflare protection
		},
	}
}

func (n *NovelOnesAdapter) Name() string {
	return "NovelOnesAdapter"
}

func (n *NovelOnesAdapter) MatchesURL(url string) bool {
	return strings.Contains(url, "novelones.com")
}

func (n *NovelOnesAdapter) ExtractChapterList(doc *goquery.Document) ([]ChapterMeta, error) {
	// If it's a chapter page being passed directly (detected by the presence of chapter content)
	if doc.Find(n.ContentSelector).Length() > 0 {
		title := doc.Find("h1").First().Text()
		
		// Get canonical URL if available
		url, _ := doc.Find("link[rel='canonical']").Attr("href")
		
		return []ChapterMeta{
			{
				ChapterNumber: extractNumber(title),
				Title:         title,
				URL:           url,
			},
		}, nil
	}

	// Otherwise, let's try the generic extraction (Novel Index page)
	chapters, err := n.GenericAdapter.ExtractChapterList(doc)
	if err == nil && len(chapters) > 0 {
		return chapters, nil
	}

	return nil, errors.New("no chapters found on novelones page")
}

func (n *NovelOnesAdapter) ExtractChapterContent(doc *goquery.Document) (string, error) {
	selection := doc.Find(n.ContentSelector)
	if selection.Length() == 0 {
		return "", errors.New("content selector found no matches")
	}

	// Novelones obfuscates text using CSS pseudo-elements:
	// <style> .pgbmxn::before { content: attr(gmhmvb) } </style>
	// <p class="pgbmxn" gmhmvb="Translated Text!"></p>
	
	// 1. Find all style blocks
	var classToAttr map[string]string = make(map[string]string)
	re := regexp.MustCompile(`\.([a-zA-Z0-9_-]+)::before\s*\{\s*content:\s*attr\(([a-zA-Z0-9_-]+)\)\s*\}`)
	
	doc.Find("style").Each(func(i int, s *goquery.Selection) {
		styleContent := s.Text()
		matches := re.FindAllStringSubmatch(styleContent, -1)
		for _, match := range matches {
			if len(match) == 3 {
				classToAttr[match[1]] = match[2]
			}
		}
	})

	// 2. Replace empty elements with the attribute text
	selection.Find("*").Each(func(i int, s *goquery.Selection) {
		// Check the classes on this element
		classAttr, _ := s.Attr("class")
		classes := strings.Fields(classAttr)
		
		for _, class := range classes {
			if targetAttr, exists := classToAttr[class]; exists {
				// We found a mapping! Extract the text from targetAttr
				if val, ok := s.Attr(targetAttr); ok {
					s.SetText(val)
					// Optionally clean up the attribute and class so it looks normal
					s.RemoveAttr(targetAttr)
					s.RemoveClass(class)
				}
			}
		}
	})

	// Get resulting clean HTML
	html, err := selection.Html()
	if err != nil {
		return "", fmt.Errorf("failed to extract html: %w", err)
	}

	return strings.TrimSpace(html), nil
}
