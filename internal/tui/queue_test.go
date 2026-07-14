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
	next, ok := q.Next()
	if !ok || next.VideoID != "d" {
		t.Fatalf("next=%v ok=%v", next, ok)
	}
	if _, ok := q.Next(); ok {
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
