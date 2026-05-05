package cleaner

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// CleanHTMLPipeline removes HTML tags, normalizes whitespace, and handles Thai encoding
func CleanHTMLPipeline(rawHTML string) (string, error) {
	// 1. Strip HTML tags via goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return "", err
	}
	text := doc.Text()

	// 2. Normalize whitespace (collapse multiple spaces/newlines)
	reSpace := regexp.MustCompile(`\s+`)
	text = reSpace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// 3. Thai-specific: Look for common mojibake or check if it needs TIS-620 conversion
	// Note: Modern scraping usually gets UTF-8, but older Thai sites might use TIS-620/Windows-874.
	// For this MVP, we provide a basic heuristic or assume the user will configure if TIS-620.
	if NeedsTIS620Decoding(text) {
		text, _ = DecodeTIS620(text)
	}

	// 4. Clean ZWNJ/ZWJ (Zero Width Non-Joiner/Joiner) which often messes up Thai NLP
	text = strings.ReplaceAll(text, "\u200C", "")
	text = strings.ReplaceAll(text, "\u200D", "")
	
	// 5. Clean common zero-width spaces
	text = strings.ReplaceAll(text, "\u200B", "")

	return text, nil
}

// ExtractSentences provides a basic heuristic for splitting sentences
func ExtractSentences(text string) []string {
	// Simple split by terminal punctuation
	// Thai often uses spaces for sentence separation, but let's handle explicit punctuation first
	re := regexp.MustCompile(`([.!?。]+)`)
	parts := re.Split(text, -1)
	
	var sentences []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			sentences = append(sentences, strings.TrimSpace(p))
		}
	}
	return sentences
}

// NeedsTIS620Decoding returns true if the string seems to contain TIS-620 artifacts.
// This is a naive heuristic (e.g. checking for high-ascii non-utf8 signatures).
func NeedsTIS620Decoding(s string) bool {
	// Simplified check: If it contains valid Thai UTF-8 range (\u0E00-\u0E7F), it's probably fine.
	// If it contains a lot of characters in \xA1-\xFB without matching valid UTF-8 sequences, it might be TIS-620.
	// We'll leave this false by default unless explicitly configured in a real world scenario.
	return false 
}

// StripChapterHeader removes a leading "Chapter N[: title]" or "第N章 ..." line if present.
func StripChapterHeader(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*chapter\s+\d+[^\n]*\n`),
		regexp.MustCompile(`^\s*第[一二三四五六七八九十百千零\d]+章[^\n]*\n`),
	}
	for _, re := range patterns {
		text = re.ReplaceAllString(text, "")
	}
	return text
}

// StripTranslatorNotes removes inline translator notes such as [T/N: ...], (TL note: ...), (TN: ...).
// Trailing whitespace before/after the removal is left in place; callers should normalize whitespace.
func StripTranslatorNotes(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\[T/?N:[^\]]*\]`),
		regexp.MustCompile(`\((?:TL note|TN|T/N|Translator note)[^)]*\)`),
	}
	for _, re := range patterns {
		text = re.ReplaceAllString(text, "")
	}
	return text
}

// DecodeTIS620 converts Windows-874/TIS-620 to UTF-8
func DecodeTIS620(s string) (string, error) {
	decoder := charmap.Windows874.NewDecoder()
	reader := transform.NewReader(strings.NewReader(s), decoder)
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return s, err // Return original on error
	}
	return buf.String(), nil
}
