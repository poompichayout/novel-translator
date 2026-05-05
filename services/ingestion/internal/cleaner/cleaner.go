package cleaner

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// stripHTMLTags removes HTML tags and returns plain text with block-level newlines preserved.
// Block tags (p/div/li/h1-h6/br) become \n so downstream line-aware strippers see paragraph structure.
func stripHTMLTags(rawHTML string) (string, error) {
	blockBreaks := regexp.MustCompile(`(?i)</(p|div|li|h[1-6])>|<br\s*/?>`)
	pre := blockBreaks.ReplaceAllString(rawHTML, "\n")
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pre))
	if err != nil {
		return "", err
	}
	return doc.Text(), nil
}

// normalizeText collapses whitespace, strips zero-width chars, and decodes TIS-620 if needed.
func normalizeText(text string) string {
	reSpace := regexp.MustCompile(`\s+`)
	text = reSpace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	if NeedsTIS620Decoding(text) {
		text, _ = DecodeTIS620(text)
	}

	text = strings.ReplaceAll(text, "\u200C", "")
	text = strings.ReplaceAll(text, "\u200D", "")
	text = strings.ReplaceAll(text, "\u200B", "")
	return text
}

// CleanHTMLPipeline removes HTML tags, normalizes whitespace, and handles Thai encoding.
func CleanHTMLPipeline(rawHTML string) (string, error) {
	text, err := stripHTMLTags(rawHTML)
	if err != nil {
		return "", err
	}
	return normalizeText(text), nil
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

// StripPromoLines removes whole lines that look like ad/promo references to source sites.
func StripPromoLines(text string) string {
	promo := regexp.MustCompile(`(?i)^.*(read at|visit|please support|original at)\b.*$`)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if promo.MatchString(strings.TrimSpace(line)) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// FullCleanChapter runs the full chapter-cleaning pipeline:
// HTML strip (block tags → newlines) → header strip → translator notes strip → promo strip → whitespace + zero-width normalize.
// HTML stripping happens first so the line-aware strippers see paragraph structure.
func FullCleanChapter(raw string) (string, error) {
	text, err := stripHTMLTags(raw)
	if err != nil {
		return "", err
	}
	text = StripChapterHeader(text)
	text = StripTranslatorNotes(text)
	text = StripPromoLines(text)
	return normalizeText(text), nil
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
