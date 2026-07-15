package session

// DefaultCrossfadeSec is the default fade/handoff window when crossfade is enabled.
const DefaultCrossfadeSec = 3

// MinCrossfadeSec / MaxCrossfadeSec clamp the configurable duration.
const (
	MinCrossfadeSec = 1
	MaxCrossfadeSec = 12
)

// PlaybackPrefs holds settings-page-ready playback options.
// Crossfade fields are used now; normalize/sleep can migrate here later.
type PlaybackPrefs struct {
	Crossfade    bool `json:"crossfade"`
	CrossfadeSec int  `json:"crossfadeSec"`
}

// ClampCrossfadeSec returns a valid duration in [Min, Max], or Default if unset/invalid.
func ClampCrossfadeSec(sec int) int {
	if sec < MinCrossfadeSec || sec > MaxCrossfadeSec {
		return DefaultCrossfadeSec
	}
	return sec
}

// DefaultPlaybackPrefs returns optional crossfade off with a sensible duration.
func DefaultPlaybackPrefs() PlaybackPrefs {
	return PlaybackPrefs{
		Crossfade:    false,
		CrossfadeSec: DefaultCrossfadeSec,
	}
}
