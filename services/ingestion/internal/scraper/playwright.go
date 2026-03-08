package scraper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// RenderWithPlaywright shells out to the Node.js script to fully render a page.
func RenderWithPlaywright(ctx context.Context, url string, browserTimeoutMs int) (*goquery.Document, error) {
	scriptPath := os.Getenv("PLAYWRIGHT_SCRIPT_PATH")
	if scriptPath == "" {
		scriptPath = "/app/scripts/playwright/render.js"
	}

	// Make sure the script exists or adjust the path based on env
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Fallback for local dev
		scriptPath = "./scripts/playwright/render.js"
	}

	cmd := exec.CommandContext(ctx, "node", scriptPath, url, fmt.Sprintf("%d", browserTimeoutMs))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("playwright execution failed for %s: %w, output: %s", url, err, string(output))
	}

	// output is the rendered HTML string
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(output)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse playwright output HTML: %w", err)
	}

	return doc, nil
}
