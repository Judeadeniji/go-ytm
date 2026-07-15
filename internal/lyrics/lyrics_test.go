package lyrics

import (
	"testing"
	"time"
)

func TestParseLRC(t *testing.T) {
	raw := `[00:17.12] I feel your breath upon my neck
[00:21.50] And it sends a spark into my soul
[03:20.31] The clock won't stop
[03:25.72]
[01:02.5] Short frac
[1:05.123] Single digit minute
[00:10.00][00:11.00] Double stamp`
	lines := ParseLRC(raw)
	if len(lines) < 6 {
		t.Fatalf("got %d lines, want >= 6", len(lines))
	}
	// Sorted by time — first stamp in file is 17s, but 10s comes first after sort.
	if lines[0].Text != "Double stamp" || lines[0].Time != 10*time.Second {
		t.Errorf("line0 after sort: %q @ %v", lines[0].Text, lines[0].Time)
	}
	want := 17*time.Second + 120*time.Millisecond
	var found bool
	for _, ln := range lines {
		if ln.Text == "I feel your breath upon my neck" {
			found = true
			if ln.Time != want {
				t.Errorf("17s line time: got %v want %v", ln.Time, want)
			}
		}
	}
	if !found {
		t.Error("missing 17s lyric line")
	}
	// Double timestamps produce two lines with same text.
	var doubles int
	for _, ln := range lines {
		if ln.Text == "Double stamp" {
			doubles++
		}
	}
	if doubles != 2 {
		t.Errorf("double stamp lines: got %d want 2", doubles)
	}
	// Ascending order.
	for i := 1; i < len(lines); i++ {
		if lines[i].Time < lines[i-1].Time {
			t.Fatalf("unsorted at %d: %v then %v", i, lines[i-1].Time, lines[i].Time)
		}
	}
}

func TestActiveLineIndexUnsortedInputIsSortedByParse(t *testing.T) {
	lines := ParseLRC(`[00:30.00] late
[00:05.00] early
[00:15.00] mid`)
	if got := ActiveLineIndex(lines, 6*time.Second); got != 0 {
		t.Fatalf("at 6s: got %d want 0 (early)", got)
	}
	if got := ActiveLineIndex(lines, 16*time.Second); got != 1 {
		t.Fatalf("at 16s: got %d want 1 (mid)", got)
	}
	if got := ActiveLineIndex(lines, 31*time.Second); got != 2 {
		t.Fatalf("at 31s: got %d want 2 (late)", got)
	}
}

func TestActiveLineIndex(t *testing.T) {
	lines := []Line{
		{Time: 1 * time.Second, Text: "a"},
		{Time: 5 * time.Second, Text: "b"},
		{Time: 10 * time.Second, Text: "c"},
	}
	cases := []struct {
		pos  time.Duration
		want int
	}{
		{0, -1},
		{1 * time.Second, 0},
		{3 * time.Second, 0},
		{5 * time.Second, 1},
		{9 * time.Second, 1},
		{10 * time.Second, 2},
		{60 * time.Second, 2},
	}
	for _, tc := range cases {
		if got := ActiveLineIndex(lines, tc.pos); got != tc.want {
			t.Errorf("pos %v: got %d want %d", tc.pos, got, tc.want)
		}
	}
}

func TestPickBestPrefersDurationThenSynced(t *testing.T) {
	results := []Result{
		{ID: 1, Duration: 200, PlainLyrics: "plain only"},
		{ID: 2, Duration: 233, SyncedLyrics: "[00:01.00] hi", PlainLyrics: "hi"},
		{ID: 3, Duration: 233, PlainLyrics: "plain match"},
	}
	best := pickBest(results, 233)
	if best.ID != 2 {
		t.Fatalf("want id 2 (synced+duration), got %d", best.ID)
	}
	best = pickBest(results, 0)
	if best.ID != 2 {
		t.Fatalf("want synced even without duration, got %d", best.ID)
	}
}
