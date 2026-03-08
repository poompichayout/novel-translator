package scraper

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Engine struct {
	client      *http.Client
	adapters    []SiteAdapter
	concurrency int
}

func NewEngine(concurrency int) *Engine {
	return &Engine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		adapters: []SiteAdapter{
			NewNovelLiveAdapter(),
			NewNovelOnesAdapter(),
		},
		concurrency: concurrency,
	}
}

func (e *Engine) FetchDocument(ctx context.Context, url string, needsJS bool) (*goquery.Document, error) {
	if needsJS {
		return RenderWithPlaywright(ctx, url, 30000)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d error for %s", res.StatusCode, url)
	}

	return goquery.NewDocumentFromReader(res.Body)
}

// DefaultGenericAdapter provides a fallback
func (e *Engine) GetAdapter(url string) SiteAdapter {
	for _, a := range e.adapters {
		if a.MatchesURL(url) {
			return a
		}
	}
	return &GenericAdapter{
		IndexSelector:   "a.chapter-link, .chapter-list a",
		ContentSelector: ".chapter-content, #chapter-content, article",
		RequiresJS:      false,
	}
}

// ScrapeChapter fetches and extracts a single chapter using the appropriate adapter
func (e *Engine) ScrapeChapter(ctx context.Context, url string) (string, error) {
	adapter := e.GetAdapter(url)

	doc, err := e.FetchDocument(ctx, url, adapter.NeedsJSRendering())
	if err != nil {
		return "", err
	}

	content, err := adapter.ExtractChapterContent(doc)
	if err != nil {
		return "", err
	}

	return content, nil
}

// ProcessBatch takes a list of URLs and scrapes them concurrently
func (e *Engine) ProcessBatch(ctx context.Context, urls []string, resultChan chan<- string, errorChan chan<- error) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, e.concurrency)

	for _, u := range urls {
		wg.Add(1)
		semaphore <- struct{}{} // block if at concurrency limit

		go func(url string) {
			defer wg.Done()
			defer func() { <-semaphore }() // release permit

			// Simple rate limiting sleep (could be configurable)
			time.Sleep(1 * time.Second)

			content, err := e.ScrapeChapter(ctx, url)
			if err != nil {
				log.Printf("[Scraper] failed to scrape %s: %v", url, err)
				errorChan <- err
				return
			}
			resultChan <- content
		}(u)
	}

	wg.Wait()
}
