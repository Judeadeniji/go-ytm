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
type TrackStartedMsg struct {
	Track Track
	Gen   int
}

// streamReadyMsg is the async result of resolving a stream URL for a play request.
type streamReadyMsg struct {
	Track Track
	URL   string
	Gen   int
	Err   error
}

func fetchStreamURL(ext *search.Extractor, videoID string) tea.Cmd {
	return func() tea.Msg {
		url, err := ext.GetStreamURL(context.Background(), videoID)
		return StreamURLMsg{URL: url, Err: err}
	}
}

// playTrack stops silence gap immediately via a separate Stop cmd; this cmd only
// resolves the URL. Pair with stopPlayback in a Batch, then handle streamReadyMsg.
func playTrack(ext *search.Extractor, t Track, gen int) tea.Cmd {
	return func() tea.Msg {
		url, err := ext.GetStreamURL(context.Background(), t.VideoID)
		return streamReadyMsg{Track: t, URL: url, Gen: gen, Err: err}
	}
}

// loadTrack loads a resolved URL into mpv and signals TrackStartedMsg.
func loadTrack(p *player.Player, t Track, url string, gen int) tea.Cmd {
	return func() tea.Msg {
		_ = p.Load(url)
		_ = p.Play()
		return TrackStartedMsg{Track: t, Gen: gen}
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
	return doSearchFiltered(apiClient, query, "")
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
	URL    string
	Width  int
	Height int
	Kitty  *KittyImage
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
	return fetchImageSized(url, artWidth, artHeight)
}

func fetchImageSized(url string, width, height int) tea.Cmd {
	return func() tea.Msg {
		id := hashString(imageCacheKey(url, width, height))
		kitty := RenderRemoteImage(url, width, height, id)
		return ImageLoadedMsg{URL: url, Width: width, Height: height, Kitty: &kitty}
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
