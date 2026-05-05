package cleaner

import "testing"

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
