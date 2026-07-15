package tui

import "testing"

func TestSetFromStartsAtIndex(t *testing.T) {
	var q Queue
	tracks := []Track{
		{VideoID: "a", Title: "A"},
		{VideoID: "b", Title: "B"},
		{VideoID: "c", Title: "C"},
		{VideoID: "d", Title: "D"},
	}
	q.SetFrom(tracks, 2)
	if q.Len() != 2 {
		t.Fatalf("len=%d want 2", q.Len())
	}
	cur, ok := q.Current()
	if !ok || cur.VideoID != "c" {
		t.Fatalf("current=%v ok=%v", cur, ok)
	}
	next, ok := q.Next(false, false)
	if !ok || next.VideoID != "d" {
		t.Fatalf("next=%v ok=%v", next, ok)
	}
	if _, ok := q.Next(false, false); ok {
		t.Fatal("expected end of queue")
	}
}

func TestTruncateAfterCurrent(t *testing.T) {
	var q Queue
	q.SetFrom([]Track{{VideoID: "a"}, {VideoID: "b"}, {VideoID: "c"}}, 0)
	q.TruncateAfterCurrent()
	if q.Len() != 1 {
		t.Fatalf("len=%d want 1", q.Len())
	}
}

func TestCapHistory(t *testing.T) {
	var q Queue
	tracks := make([]Track, 10)
	for i := range tracks {
		tracks[i] = Track{VideoID: string(rune('a' + i))}
	}
	q.SetFrom(tracks, 0)
	for i := 0; i < 7; i++ {
		q.Next(false, false)
	}
	q.CapHistory(3)
	if q.CurrentIndex() != 3 {
		t.Fatalf("current=%d want 3", q.CurrentIndex())
	}
	if q.Len() != 6 { // 3 before + current + 2 after (10-7=3 remaining after start at 7... wait)
		// Started at 0, advanced 7 times → current=7, len=10
		// CapHistory(3): drop = 7-3 = 4, new current=3, new len=6
		t.Fatalf("len=%d want 6", q.Len())
	}
	cur, ok := q.Current()
	if !ok || cur.VideoID != string(rune('a'+7)) {
		t.Fatalf("current=%v ok=%v", cur, ok)
	}
}
