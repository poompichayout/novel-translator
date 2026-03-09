package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type ScrapeGraphAdapter struct {
	GeminiAPIKey string
}

func NewScrapeGraphAdapter(geminiAPIKey string) *ScrapeGraphAdapter {
	return &ScrapeGraphAdapter{
		GeminiAPIKey: geminiAPIKey,
	}
}

func (a *ScrapeGraphAdapter) Name() string {
	return "ScrapeGraphAI"
}

func (a *ScrapeGraphAdapter) MatchesURL(url string) bool {
	return true // Acts as a universal adapter now
}

type scrapeGraphJSON struct {
	NovelTitle    string `json:"novel_title"`
	NovelURL      string `json:"novel_url"`
	ChapterTitle  string `json:"chapter_title"`
	ChapterNumber int    `json:"chapter_number"`
	Content       string `json:"content"`
	Error         string `json:"error"`
	Chapters      []struct {
		ChapterNumber int    `json:"chapter_number"`
		Title         string `json:"title"`
		URL           string `json:"url"`
	} `json:"chapters"`
}

func (a *ScrapeGraphAdapter) Scrape(ctx context.Context, targetURL string) (*ScrapeResult, error) {
	scriptPath := os.Getenv("SCRAPEGRAPH_SCRIPT_PATH")
	if scriptPath == "" {
		scriptPath = "/app/scripts/scrapegraph/extract.py"
	}
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Fallback for local run
		scriptPath = "./scripts/scrapegraph/extract.py"
		if _, err2 := os.Stat(scriptPath); os.IsNotExist(err2) {
			scriptPath = "./scripts/scrapegraph/extract.py"
			if _, err3 := os.Stat(scriptPath); os.IsNotExist(err3) {
				return nil, fmt.Errorf("could not find python script at %s", scriptPath)
			}
		}
	}

	pythonBin := os.Getenv("PYTHON_BIN")
	if pythonBin == "" {
		// Default to local venv if available
		if _, err := os.Stat("./.venv/bin/python"); err == nil {
			pythonBin = "./.venv/bin/python"
		} else {
			pythonBin = "python3"
		}
	}

	cmd := exec.CommandContext(ctx, pythonBin, scriptPath, targetURL)
	cmd.Env = append(os.Environ(), "GEMINI_API_KEY="+a.GeminiAPIKey)

	// We only capture stdout so we avoid scrapegraph debug logs
	output, err := cmd.Output()
	if err != nil {
		// Output might contain stdout, but we should also check stderr if we want a better error, 
		// but getting an ExitError gives us the basics.
		return nil, fmt.Errorf("python script failed for %s: %w, output: %s", targetURL, err, string(output))
	}

	outStr := string(output)
	idx := strings.Index(outStr, "{")
	if idx == -1 {
		return nil, fmt.Errorf("no json output found from python script: %s", outStr)
	}

	jsonStr := outStr[idx:]
	var raw scrapeGraphJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse json output: %w, output: %s", err, outStr)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("scrapegraph error: %s", raw.Error)
	}

	result := &ScrapeResult{
		NovelTitle:    raw.NovelTitle,
		NovelURL:      raw.NovelURL,
		ChapterTitle:  raw.ChapterTitle,
		ChapterNumber: raw.ChapterNumber,
		Content:       raw.Content,
	}

	for _, ch := range raw.Chapters {
		result.Chapters = append(result.Chapters, ChapterMeta{
			ChapterNumber: ch.ChapterNumber,
			Title:         ch.Title,
			URL:           ch.URL,
		})
	}

	return result, nil
}
