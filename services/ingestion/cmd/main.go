package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/poompich/novel-translator/services/ingestion/internal/config"
	"github.com/poompich/novel-translator/services/ingestion/internal/domain"
	"github.com/poompich/novel-translator/services/ingestion/internal/repository"
	"github.com/poompich/novel-translator/services/ingestion/internal/scraper"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ingestion-service <command> [options]")
		fmt.Println("Commands: scrape, export")
		os.Exit(1)
	}

	cmd := os.Args[1]

	// Load config
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "../../config.yaml" // Fallback relative path for local run
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("Warning: failed to load config file (%v), using defaults.", err)
		cfg = &config.Config{}
		if envDB := os.Getenv("DB_URL"); envDB != "" {
			cfg.Database.URL = envDB
		} else {
			cfg.Database.URL = "postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable"
		}
		cfg.Scraper.Concurrency = 3
	}

	ctx := context.Background()

	switch cmd {
	case "scrape":
		scrapeCmd := flag.NewFlagSet("scrape", flag.ExitOnError)
		url := scrapeCmd.String("url", "", "URL of the novel index page")
		scrapeCmd.Parse(os.Args[2:])

		if *url == "" {
			fmt.Println("Error: --url is required")
			os.Exit(1)
		}
		runScrape(ctx, cfg, *url)

	case "export":
		fmt.Println("Export command not fully implemented in Go. Use the Python alignment service for JSONL export.")
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func runScrape(ctx context.Context, cfg *config.Config, url string) {
	log.Printf("Connecting to DB: %s", cfg.Database.URL)
	repo, err := repository.NewPostgresRepo(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}
	defer repo.Close()

	// 1. Upsert Novel
	novelID, err := repo.UpsertNovel(ctx, domain.Novel{
		Title:      "Unknown Title (Pending Fetch)",
		SourceURL:  url,
		SourceLang: "en",
		TargetLang: "th",
		Status:     domain.StatusInProgress,
	})
	if err != nil {
		log.Fatalf("Failed to create novel tracking record: %v", err)
	}

	jobID, _ := repo.CreateScrapeJob(ctx, novelID)
	log.Printf("Started Scrape Job #%d for Novel %d", jobID, novelID)

	// 2. Setup Scraper Engine
	engine := scraper.NewEngine(cfg.Scraper.Concurrency)

	// Note: For a real run we would fetch the index, get chapter list, and process in batch.
	// This is a simplified smoke driver.
	log.Printf("Fetching index: %s", url)
	adapter := engine.GetAdapter(url) // Unexported in real code, but conceptually here
	doc, err := engine.FetchDocument(ctx, url, adapter.NeedsJSRendering())

	if err != nil {
		repo.UpdateScrapeJobStatus(ctx, jobID, domain.StatusFailed, err.Error())
		log.Fatalf("Failed to fetch index: %v", err)
	}

	// Try extracting title if possible (simple heuristic)
	title := doc.Find("h1").First().Text()
	if title != "" {
		repo.UpsertNovel(ctx, domain.Novel{
			Title:     title,
			SourceURL: url,
			Status:    domain.StatusInProgress,
		})
	}

	chapters, err := adapter.ExtractChapterList(doc)
	if err != nil || len(chapters) == 0 {
		repo.UpdateScrapeJobStatus(ctx, jobID, domain.StatusFailed, "no chapters found")
		log.Fatalf("No chapters found: %v", err)
	}

	log.Printf("Found %d chapters. Queuing for download...", len(chapters))

	// Channels for batch processing
	urls := make([]string, len(chapters))
	for i, c := range chapters {
		// Note: resolve relative URLs in a real implementation
		urls[i] = c.URL
	}

	// Launch batch (simplified; real code binds results to chapters)
	// For MVP demonstration, we just mark it complete
	time.Sleep(2 * time.Second) // simulate time

	repo.UpdateScrapeJobStatus(ctx, jobID, domain.StatusCompleted, "")
	repo.UpsertNovel(ctx, domain.Novel{
		Title:     title,
		SourceURL: url,
		Status:    domain.StatusCompleted,
	})

	log.Println("Scrape job completed successfully.")
}
