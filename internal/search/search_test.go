package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLookPathYtDlpFindsHomeLocal(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	candidate := filepath.Join(home, ".local", "bin", "yt-dlp")
	if _, err := os.Stat(candidate); err != nil {
		t.Skip("yt-dlp not installed in ~/.local/bin")
	}
	old := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", old) })
	_ = os.Setenv("PATH", "/usr/bin:/bin")

	got, err := lookPathYtDlp()
	if err != nil {
		t.Fatalf("lookPathYtDlp: %v", err)
	}
	if got != candidate {
		// Absolute path via project bin is also OK.
		if _, err := os.Stat(got); err != nil {
			t.Fatalf("resolved path unusable: %s (%v)", got, err)
		}
	}
}

func TestGetStreamURLReportsMissingYtDlp(t *testing.T) {
	oldPath := os.Getenv("PATH")
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", oldPath)
		_ = os.Setenv("HOME", oldHome)
	})
	_ = os.Setenv("HOME", t.TempDir())
	_ = os.Setenv("PATH", "/usr/bin:/bin")

	e := NewExtractor()
	_, err := e.GetStreamURL(context.Background(), "xxxxxxxxxxx")
	if err == nil {
		t.Fatal("expected error for bogus id")
	}
	t.Log(err.Error())
}
