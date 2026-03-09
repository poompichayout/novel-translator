package scraper

import (
	"context"
	"log"
	"sync"
	"time"
)

type Engine struct {
	concurrency int
	adapter     SiteAdapter
}

func NewEngine(concurrency int, geminiAPIKey string) *Engine {
	return &Engine{
		concurrency: concurrency,
		adapter:     NewScrapeGraphAdapter(geminiAPIKey),
	}
}

// GetAdapter returns the single adapter
func (e *Engine) GetAdapter(url string) SiteAdapter {
	return e.adapter
}

// ScrapePage scrapes a page (index or chapter) using ScrapeGraphAI
func (e *Engine) ScrapePage(ctx context.Context, url string) (*ScrapeResult, error) {
	return e.adapter.Scrape(ctx, url)
}

// ProcessBatch takes a list of URLs and scrapes them concurrently
// Expects chapter URLs and returns chapter contents
func (e *Engine) ProcessBatch(ctx context.Context, urls []string, resultChan chan<- string, errorChan chan<- error) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, e.concurrency)

	for _, u := range urls {
		wg.Add(1)
		semaphore <- struct{}{} // block if at concurrency limit

		go func(url string) {
			defer wg.Done()
			defer func() { <-semaphore }() // release permit

			// Simple rate limiting sleep
			time.Sleep(1 * time.Second)

			res, err := e.ScrapePage(ctx, url)
			if err != nil {
				log.Printf("[Scraper] failed to scrape %s: %v", url, err)
				errorChan <- err
				return
			}
			resultChan <- res.Content
		}(u)
	}

	wg.Wait()
}
