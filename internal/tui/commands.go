package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// TrackStartedMsg is sent when a track has been successfully loaded and started.
type TrackStartedMsg struct {
	Track       Track
	Gen         int
	Err         error
	SeekOnlyErr bool // Err is seek-only; playback already started
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

// playTrack resolves a stream URL for t. Pair with stopPlayback via tea.Sequence
// (not Batch) so Stop cannot race a subsequent Load.
func playTrack(ext *search.Extractor, t Track, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		url, err := ext.GetStreamURL(ctx, t.VideoID)
		return streamReadyMsg{Track: t, URL: url, Gen: gen, Err: err}
	}
}

// loadTrack loads a resolved URL into mpv and signals TrackStartedMsg.
func loadTrack(p *player.Player, t Track, url string, gen int, seekTo float64) tea.Cmd {
	return func() tea.Msg {
		if err := p.Load(url); err != nil {
			return TrackStartedMsg{Track: t, Gen: gen, Err: err}
		}
		if err := p.Play(); err != nil {
			return TrackStartedMsg{Track: t, Gen: gen, Err: err}
		}
		var seekErr error
		if seekTo > 1 {
			seekErr = p.SeekAbsolute(seekTo)
		}
		return TrackStartedMsg{Track: t, Gen: gen, Err: seekErr, SeekOnlyErr: seekErr != nil}
	}
}

// trackEndedMsg is raised when mpv fires an end-file event.
type trackEndedMsg struct {
	Reason string
	Closed bool // Events channel closed (mpv IPC gone)
}

// listenTrackEnded waits for the next mpv end-file event.
func listenTrackEnded(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return trackEndedMsg{Closed: true}
		}
		ev, ok := <-p.Events()
		if !ok {
			return trackEndedMsg{Closed: true}
		}
		return trackEndedMsg{Reason: ev.Reason}
	}
}

type playerErrMsg struct {
	Op  string
	Err error
}

// togglePause sends a pause-cycle command to mpv.
func togglePause(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		if err := p.TogglePause(); err != nil {
			return playerErrMsg{Op: "pause", Err: err}
		}
		return nil
	}
}

// seekCmd sends a relative seek command to mpv.
func seekCmd(p *player.Player, seconds float64) tea.Cmd {
	return func() tea.Msg {
		if err := p.SeekRelative(seconds); err != nil {
			return playerErrMsg{Op: "seek", Err: err}
		}
		return nil
	}
}

// seekAbsoluteCmd seeks to an absolute position in seconds.
func seekAbsoluteCmd(p *player.Player, seconds float64) tea.Cmd {
	return func() tea.Msg {
		if err := p.SeekAbsolute(seconds); err != nil {
			return playerErrMsg{Op: "seek", Err: err}
		}
		return nil
	}
}

func stopPlayback(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		if err := p.Stop(); err != nil {
			return playerErrMsg{Op: "stop", Err: err}
		}
		return nil
	}
}

func loadAndPlay(p *player.Player, url string) tea.Cmd {
	return func() tea.Msg {
		if err := p.Load(url); err != nil {
			return playerErrMsg{Op: "load", Err: err}
		}
		if err := p.Play(); err != nil {
			return playerErrMsg{Op: "play", Err: err}
		}
		return nil
	}
}

type SearchResultsMsg struct {
	Results []ytmapi.SearchResult
	Gen     int
	Err     error
}

func doSearch(apiClient *ytmapi.Client, query string, gen int) tea.Cmd {
	return doSearchFiltered(apiClient, query, "", gen)
}

type SearchSuggestionsMsg struct {
	Suggestions []ytmapi.SearchSuggestionItem
	Results     []ytmapi.SearchResult
	Query       string
	Gen         int
	Err         error
}

type suggestionsDebounceMsg struct {
	Query string
	Gen   int
}

const suggestionsDebounce = 180 * time.Millisecond

func debounceSuggestions(query string, gen int) tea.Cmd {
	return tea.Tick(suggestionsDebounce, func(time.Time) tea.Msg {
		return suggestionsDebounceMsg{Query: query, Gen: gen}
	})
}

func fetchSuggestions(apiClient *ytmapi.Client, query string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		q := strings.TrimSpace(query)
		if q == "" {
			return SearchSuggestionsMsg{Query: query, Gen: gen}
		}
		if ctx == nil {
			ctx = context.Background()
		}

		type sugOut struct {
			items []ytmapi.SearchSuggestionItem
			err   error
		}
		type resOut struct {
			items []ytmapi.SearchResult
			err   error
		}
		sugCh := make(chan sugOut, 1)
		resCh := make(chan resOut, 1)
		go func() {
			items, err := apiClient.GetSearchSuggestions(ctx, q)
			sugCh <- sugOut{items, err}
		}()
		go func() {
			items, err := apiClient.SearchFiltered(ctx, q, "", 5)
			resCh <- resOut{items, err}
		}()

		var sug sugOut
		var res resOut
		for sugCh != nil || resCh != nil {
			select {
			case <-ctx.Done():
				return SearchSuggestionsMsg{Query: query, Gen: gen, Err: ctx.Err()}
			case s := <-sugCh:
				sug = s
				sugCh = nil
			case r := <-resCh:
				res = r
				resCh = nil
			}
		}

		if sug.err != nil && res.err != nil {
			return SearchSuggestionsMsg{Query: query, Gen: gen, Err: sug.err}
		}
		return SearchSuggestionsMsg{
			Suggestions: sug.items,
			Results:     res.items,
			Query:       query,
			Gen:         gen,
			Err:         nil,
		}
	}
}

type HomeMsg struct {
	Carousels []ytmapi.HomeCarousel
	Err       error
}

func fetchHome(apiClient *ytmapi.Client) tea.Cmd {
	return func() tea.Msg {
		carousels, err := apiClient.GetHome(context.Background())
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
