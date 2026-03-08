package scraper

import (
	"strings"
)

// NovelLiveAdapter is a specific adapter for novellive.app
// which frequently uses Cloudflare / JS challenges.
type NovelLiveAdapter struct {
	GenericAdapter
}

func NewNovelLiveAdapter() *NovelLiveAdapter {
	return &NovelLiveAdapter{
		GenericAdapter: GenericAdapter{
			IndexSelector:   ".chapter-list a",
			ContentSelector: ".chapter-content, #chapter-content",
			RequiresJS:      true, // Force JS rendering to bypass 403 / Cloudflare
		},
	}
}

func (n *NovelLiveAdapter) Name() string {
	return "NovelLiveAdapter"
}

func (n *NovelLiveAdapter) MatchesURL(url string) bool {
	return strings.Contains(url, "novellive.app") || strings.Contains(url, "novellive.com")
}
