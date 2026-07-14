package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const version = 1

// Track is a playable item persisted with the session.
type Track struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

// NavItem is one restored navigation stack entry.
type NavItem struct {
	Kind  string `json:"kind"` // artist | album | playlist | search
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Snapshot is the durable UI + playback state.
type Snapshot struct {
	Version          int      `json:"version"`
	ActiveMenu       string   `json:"activeMenu"`
	QueuePanelHidden bool     `json:"queuePanelHidden"`
	SearchFilter     string   `json:"searchFilter"`
	LastSearchQuery  string   `json:"lastSearchQuery"`
	ActiveCarousel   int      `json:"activeCarousel"`
	HomeCardCursor   int      `json:"homeCardCursor"`
	TrackCursor      int      `json:"trackCursor"`
	ListCursor       int      `json:"listCursor"`
	QueueCursor      int      `json:"queueCursor"`
	PlayPos          float64   `json:"playPos"`
	Queue            []Track   `json:"queue"`
	QueueIndex       int       `json:"queueIndex"`
	ShowSearch       bool      `json:"showSearch"`
	Nav              []NavItem `json:"nav"`
}

// Store reads/writes a Snapshot JSON file under the user state dir.
type Store struct {
	path string
	mu   sync.Mutex
}

// DefaultPath returns ~/.local/state/go-ytm/session.json (XDG_STATE_HOME aware).
func DefaultPath() (string, error) {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		state = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(state, "go-ytm", "session.json"), nil
}

// Open creates a Store at the default path.
func Open() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

// Path returns the session file path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load reads the snapshot. Missing file returns (nil, nil).
func (s *Store) Load() (*Snapshot, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	if snap.Version == 0 {
		snap.Version = version
	}
	return &snap, nil
}

// Save writes the snapshot atomically.
func (s *Store) Save(snap Snapshot) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	snap.Version = version
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
