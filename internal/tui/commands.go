package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// TrackStartedMsg is sent when a track has been successfully loaded and started.
type TrackStartedMsg struct{ Track Track }

func fetchStreamURL(ext *search.Extractor, videoID string) tea.Cmd {
	return func() tea.Msg {
		url, err := ext.GetStreamURL(context.Background(), videoID)
		return StreamURLMsg{URL: url, Err: err}
	}
}

// playTrack resolves the stream URL for t, loads it into mpv, and starts playback.
// On success it returns TrackStartedMsg; on URL extraction failure it returns StreamURLMsg{Err: ...}.
func playTrack(p *player.Player, ext *search.Extractor, t Track) tea.Cmd {
	return func() tea.Msg {
		url, err := ext.GetStreamURL(context.Background(), t.VideoID)
		if err != nil {
			return StreamURLMsg{Err: err}
		}
		p.Load(url)
		p.Play()
		return TrackStartedMsg{Track: t}
	}
}

// togglePause sends a pause-cycle command to mpv.
func togglePause(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		p.TogglePause()
		return nil
	}
}

// seekCmd sends a relative seek command to mpv.
func seekCmd(p *player.Player, seconds float64) tea.Cmd {
	return func() tea.Msg {
		p.SeekRelative(seconds)
		return nil
	}
}

func stopPlayback(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		p.Stop()
		return nil
	}
}

func loadAndPlay(p *player.Player, url string) tea.Cmd {
	return func() tea.Msg {
		p.Load(url)
		p.Play()
		return nil
	}
}

type SearchResultsMsg struct {
	Results []ytmapi.SearchResult
	Err     error
}

func doSearch(apiClient *ytmapi.Client, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := apiClient.Search(query)
		return SearchResultsMsg{Results: results, Err: err}
	}
}

type SearchSuggestionsMsg struct {
	Suggestions []ytmapi.SearchSuggestionItem
	Err         error
}

func fetchSuggestions(apiClient *ytmapi.Client, query string) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := apiClient.GetSearchSuggestions(query)
		return SearchSuggestionsMsg{Suggestions: suggestions, Err: err}
	}
}

type HomeMsg struct {
	Carousels []ytmapi.HomeCarousel
	Err       error
}

func fetchHome(apiClient *ytmapi.Client) tea.Cmd {
	return func() tea.Msg {
		carousels, err := apiClient.GetHome()
		return HomeMsg{Carousels: carousels, Err: err}
	}
}

type ImageLoadedMsg struct {
	URL   string
	Kitty *KittyImage
}

func hashString(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = 31*h + int(s[i])
	}
	if h < 0 {
		h = -h
	}
	if h == 0 {
		h = 1
	}
	return h
}

func fetchImage(url string) tea.Cmd {
	return func() tea.Msg {
		id := hashString(url)
		kitty := RenderRemoteImage(url, artWidth, artHeight, id)
		return ImageLoadedMsg{URL: url, Kitty: &kitty}
	}
}

// imagesRedrawMsg is fired after a short debounce when thumbs finish loading,
// so we rebuild the grid once instead of once-per-image.
type imagesRedrawMsg struct{}

func debounceImagesRedraw() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return imagesRedrawMsg{}
	})
}

// writeTTY writes a Kitty Graphics payload directly to /dev/tty,
// bypassing BubbleTea's renderer which strips APC escape sequences.
func writeTTY(k *KittyImage) tea.Cmd {
	return func() tea.Msg {
		_ = k.WriteToTTY()
		return nil
	}
}
