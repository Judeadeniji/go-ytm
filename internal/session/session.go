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
	Normalize        bool      `json:"normalize"`
	SilenceSkip      bool      `json:"silenceSkip"`
	Tempo            float64   `json:"tempo"`
	Pitch            float64   `json:"pitch"`
	EQPreset         int       `json:"eqPreset"`
	RepeatMode       int       `json:"repeatMode"` // 0=Off, 1=All, 2=One
	Shuffle          bool      `json:"shuffle"`
	Crossfade        bool      `json:"crossfade"`
	CrossfadeSec     int       `json:"crossfadeSec"`
	WasPlaying       bool      `json:"wasPlaying"`
	NowPlayingOpen   bool      `json:"nowPlayingOpen"`
	Queue            []Track   `json:"queue"`
	QueueIndex       int       `json:"queueIndex"`
	ShowSearch       bool      `json:"showSearch"`
	ExploreSubTab    string    `json:"exploreSubTab"`
	Nav              []NavItem `json:"nav"`
}

// PlaybackPrefs returns the settings-ready playback preference subset.
func (s Snapshot) PlaybackPrefs() PlaybackPrefs {
	return PlaybackPrefs{
		Crossfade:    s.Crossfade,
		CrossfadeSec: ClampCrossfadeSec(s.CrossfadeSec),
	}
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
