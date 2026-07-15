package library

import (
	"path/filepath"
	"testing"

	"github.com/judeadeniji/go-ytm/internal/session"
)

func TestSaveLoadSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenPath(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	in := session.Snapshot{
		ActiveMenu:       "Library",
		QueuePanelHidden: true,
		SearchFilter:     "songs",
		LastSearchQuery:  "oasis",
		PlayPos:          42.5,
		PlayDuration:     210,
		Volume:           65,
		Muted:            true,
		Normalize:        true,
		WasPlaying:       true,
		NowPlayingOpen:   true,
		QueueIndex:       1,
		ShowSearch:       true,
		Queue: []session.Track{
			{VideoID: "a", Title: "A", Artist: "X"},
			{VideoID: "b", Title: "B", Artist: "Y"},
		},
		Nav: []session.NavItem{{Kind: "playlist", ID: "PL1", Title: "Favs"}},
	}
	if err := db.SaveSession(in); err != nil {
		t.Fatal(err)
	}
	out, err := db.LoadSession()
	if err != nil || out == nil {
		t.Fatalf("load: %v %#v", err, out)
	}
	if out.ActiveMenu != "Library" || !out.QueuePanelHidden || out.QueueIndex != 1 {
		t.Fatalf("snapshot meta: %#v", out)
	}
	if len(out.Queue) != 2 || out.Queue[1].VideoID != "b" {
		t.Fatalf("queue: %#v", out.Queue)
	}
	if len(out.Nav) != 1 || out.Nav[0].ID != "PL1" {
		t.Fatalf("nav: %#v", out.Nav)
	}
	if out.PlayPos != 42.5 || !out.ShowSearch {
		t.Fatalf("playback fields: %#v", out)
	}
	if out.PlayDuration != 210 || !out.WasPlaying || !out.NowPlayingOpen {
		t.Fatalf("resume ui fields: %#v", out)
	}
	if out.Volume != 65 || !out.Muted || !out.Normalize {
		t.Fatalf("volume fields: %#v", out)
	}
}

func TestLoadSessionEmpty(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenPath(filepath.Join(dir, "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	snap, err := db.LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if snap != nil {
		t.Fatalf("want nil, got %#v", snap)
	}
}
