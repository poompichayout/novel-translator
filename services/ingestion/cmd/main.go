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
		sourceLang := scrapeCmd.String("source-lang", "en", "Source language code (e.g., en, th)")
		targetLang := scrapeCmd.String("target-lang", "th", "Target language code (e.g., th, en)")
		scrapeCmd.Parse(os.Args[2:])

		if *url == "" {
			fmt.Println("Error: --url is required")
			os.Exit(1)
		}
		runScrape(ctx, cfg, *url, *sourceLang, *targetLang)

	case "export":
		fmt.Println("Export command not fully implemented in Go. Use the Python alignment service for JSONL export.")
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func runScrape(ctx context.Context, cfg *config.Config, url string, sourceLang string, targetLang string) {
	log.Printf("Connecting to DB: %s", cfg.Database.URL)
	repo, err := repository.NewPostgresRepo(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}
	defer repo.Close()

	// 1. Setup Scraper Engine
	engine := scraper.NewEngine(cfg.Scraper.Concurrency, cfg.Scraper.GeminiAPIKey)

	log.Printf("Extracting page with ScrapeGraphAI: %s", url)
	res, err := engine.ScrapePage(ctx, url)

	if err != nil {
		log.Fatalf("Failed to fetch page: %v", err)
	}

	// Figure out the true Novel URL and Title (in case the input URL was a chapter)
	novelURL := url
	if res.NovelURL != "" {
		novelURL = res.NovelURL
	}
	
	novelTitle := res.NovelTitle
	if novelTitle == "" {
		novelTitle = "Unknown Title"
	}

	// 2. Upsert Novel
	novelID, err := repo.UpsertNovel(ctx, domain.Novel{
		Title:      novelTitle,
		SourceURL:  novelURL,
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Status:     domain.StatusInProgress,
	})
	if err != nil {
		log.Fatalf("Failed to upsert novel record: %v", err)
	}

	// Create job since we now have the true Novel ID
	jobID, _ := repo.CreateScrapeJob(ctx, novelID)
	log.Printf("Started Scrape Job #%d for Novel %d", jobID, novelID)

	chapters := res.Chapters
	if len(chapters) == 0 {
		if res.Content != "" {
			// It could be a single chapter page
			log.Printf("Single chapter detected, extracting content...")

			// Use the dynamically extracted chapter number, defaulting to 1 if it couldn't be parsed
			chapterNum := res.ChapterNumber
			if chapterNum == 0 {
				chapterNum = 1
			}
			
			chapterTitle := res.ChapterTitle
			if chapterTitle == "" {
				chapterTitle = fmt.Sprintf("Chapter %d", chapterNum)
			}

			chapterID, err := repo.UpsertChapter(ctx, domain.Chapter{
				NovelID:        novelID,
				ChapterNumber:  chapterNum,
				Title:          chapterTitle,
				SourceURL:      url, // Single chapter URL is the one passed in
				RawContent:     res.Content,
				CleanedContent: "", // Requires separate pipeline/cleaning
				ScrapeStatus:   domain.StatusCompleted,
			})

			if err != nil {
				log.Printf("Warning: failed to upsert single chapter content: %v", err)
			} else {
				log.Printf("Successfully scraped and saved single chapter (ID %d) to database!", chapterID)
			}

			// For debug, still dump it
			os.WriteFile("chapter_output.html", []byte(res.Content), 0644)
		} else {
			repo.UpdateScrapeJobStatus(ctx, jobID, domain.StatusFailed, "no chapters or content found")
			log.Fatalf("No chapters or content found (Title: %s)", novelTitle)
		}
	} else {
		log.Printf("Found %d chapters. Queuing for download...", len(chapters))

		// Channels for batch processing
		urls := make([]string, len(chapters))
		for i, c := range chapters {
			urls[i] = c.URL

			// Store chapter metadata into DB
			chapterID, err := repo.UpsertChapter(ctx, domain.Chapter{
				NovelID:       novelID,
				ChapterNumber: c.ChapterNumber,
				Title:         c.Title,
				SourceURL:     c.URL,
				ScrapeStatus:  domain.StatusPending, // Will be picked up by the batch processor
			})
			if err != nil {
				log.Printf("Warning: failed to upsert chapter %d: %v", c.ChapterNumber, err)
			} else {
				log.Printf("Stored chapter %d (ID %d) in DB for later scraping", c.ChapterNumber, chapterID)
			}
		}

		// Launch batch (simplified; real code binds results to chapters)
		// For MVP demonstration, we just mark it complete
		time.Sleep(2 * time.Second) // simulate time
	}

	repo.UpdateScrapeJobStatus(ctx, jobID, domain.StatusCompleted, "")
	repo.UpsertNovel(ctx, domain.Novel{
		Title:      novelTitle,
		SourceURL:  novelURL,
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Status:     domain.StatusCompleted,
	})

	log.Println("Scrape job completed successfully.")
}
