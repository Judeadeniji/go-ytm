package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &Store{path: filepath.Join(dir, "session.json")}
	in := Snapshot{
		ActiveMenu: "Library",
		PlayPos:    42.5,
		QueueIndex: 1,
		Queue: []Track{
			{VideoID: "a", Title: "A"},
			{VideoID: "b", Title: "B"},
		},
		Nav: []NavItem{{Kind: "playlist", ID: "PL1", Title: "Favs"}},
	}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.Load()
	if err != nil || out == nil {
		t.Fatalf("load: %v %#v", err, out)
	}
	if out.ActiveMenu != "Library" || out.QueueIndex != 1 || len(out.Queue) != 2 {
		t.Fatalf("unexpected snapshot: %#v", out)
	}
	if out.Nav[0].ID != "PL1" {
		t.Fatalf("nav: %#v", out.Nav)
	}
	if _, err := os.Stat(s.Path()); err != nil {
		t.Fatal(err)
	}
}
