package cleaner

import (
	"testing"
)

func TestStripChapterHeader(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"explicit chapter prefix", "Chapter 12: The Hunt\nHe walked in.", "He walked in."},
		{"chapter without colon", "Chapter 7\nThe sky was red.", "The sky was red."},
		{"no header preserved", "He walked in.\nIt was cold.", "He walked in.\nIt was cold."},
		{"chinese-style header", "第十二章 狩猎\n他走了进来。", "他走了进来。"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StripChapterHeader(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripTranslatorNotes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"He looked. [T/N: subtle reference]", "He looked. "},
		{"\"Master.\" (TL note: master is shifu)", "\"Master.\" "},
		{"Plain prose with no notes.", "Plain prose with no notes."},
		{"Multi (TN: foo) chunks (T/N: bar) here.", "Multi  chunks  here."},
	}
	for _, tc := range cases {
		got := StripTranslatorNotes(tc.in)
		if got != tc.want {
			t.Errorf("StripTranslatorNotes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripPromoLines(t *testing.T) {
	in := "He walked in.\nRead at example-novel-site.com for free!\nIt was cold.\nVisit https://novelhost.io for more.\nEnd."
	want := "He walked in.\nIt was cold.\nEnd."
	got := StripPromoLines(in)
	if got != want {
		t.Errorf("StripPromoLines mismatch.\nin:\n%s\ngot:\n%s\nwant:\n%s", in, got, want)
	}
}
