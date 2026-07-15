package session

import (
	"os"
	"path/filepath"
)

// Track is a playable item persisted with the session.
type Track struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	ArtistID     string `json:"artistId,omitempty"`
	Album        string `json:"album,omitempty"`
	AlbumID      string `json:"albumId,omitempty"`
	Duration     string `json:"duration,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl"`
	IsExplicit   bool   `json:"isExplicit,omitempty"`
}

// NavItem is one restored navigation stack entry.
type NavItem struct {
	Kind  string `json:"kind"` // artist | album | playlist | search
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Snapshot is the durable UI + playback state (stored in sqlite via library.DB).
type Snapshot struct {
	Version          int       `json:"version"`
	ActiveMenu       string    `json:"activeMenu"`
	QueuePanelHidden bool      `json:"queuePanelHidden"`
	SearchFilter     string    `json:"searchFilter"`
	LastSearchQuery  string    `json:"lastSearchQuery"`
	ActiveCarousel   int       `json:"activeCarousel"`
	HomeCardCursor   int       `json:"homeCardCursor"`
	TrackCursor      int       `json:"trackCursor"`
	ListCursor       int       `json:"listCursor"`
	QueueCursor      int       `json:"queueCursor"`
	PlayPos          float64   `json:"playPos"`
	PlayDuration     float64   `json:"playDuration"`
	Volume           float64   `json:"volume"`
	Muted            bool      `json:"muted"`
	WasPlaying       bool      `json:"wasPlaying"`
	NowPlayingOpen   bool      `json:"nowPlayingOpen"`
	Queue            []Track   `json:"queue"`
	QueueIndex       int       `json:"queueIndex"`
	ShowSearch       bool      `json:"showSearch"`
	Nav              []NavItem `json:"nav"`
}

// DefaultPath returns the legacy JSON session path (used once for migration).
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
