package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

type ArtistMsg struct {
	Page      *ytmapi.ArtistPage
	RequestID string
	Err       error
}

type AlbumMsg struct {
	Page *ytmapi.AlbumPage
	Err  error
}

type PlaylistMsg struct {
	Page *ytmapi.PlaylistPage
	Err  error
}

type WatchMsg struct {
	Watch *ytmapi.WatchPlaylist
	Err   error
}

func fetchArtist(api *ytmapi.Client, channelID string) tea.Cmd {
	return func() tea.Msg {
		page, err := api.GetArtist(channelID)
		return ArtistMsg{Page: page, RequestID: channelID, Err: err}
	}
}

func fetchAlbum(api *ytmapi.Client, browseID string) tea.Cmd {
	return func() tea.Msg {
		page, err := api.GetAlbum(browseID)
		return AlbumMsg{Page: page, Err: err}
	}
}

func fetchAlbumFromAudioPlaylist(api *ytmapi.Client, audioPlaylistID string) tea.Cmd {
	return func() tea.Msg {
		browseID, err := api.GetAlbumBrowseID(audioPlaylistID)
		if err != nil {
			return AlbumMsg{Err: err}
		}
		page, err := api.GetAlbum(browseID)
		return AlbumMsg{Page: page, Err: err}
	}
}

func fetchPlaylist(api *ytmapi.Client, playlistID string) tea.Cmd {
	return func() tea.Msg {
		page, err := api.GetPlaylist(playlistID, 100)
		return PlaylistMsg{Page: page, Err: err}
	}
}

func fetchWatch(api *ytmapi.Client, videoID, playlistID string, radio bool) tea.Cmd {
	return func() tea.Msg {
		w, err := api.GetWatchPlaylist(videoID, playlistID, radio, 25)
		return WatchMsg{Watch: w, Err: err}
	}
}

func doSearchFiltered(api *ytmapi.Client, query, filter string) tea.Cmd {
	return func() tea.Msg {
		results, err := api.SearchFiltered(query, filter, 30)
		return SearchResultsMsg{Results: results, Err: err}
	}
}

func mapStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	// JSON numbers occasionally appear
	if v, ok := m[key].(float64); ok {
		return fmt.Sprintf("%.0f", v)
	}
	return ""
}

func artistRefName(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return mapStr(t, "name")
	case []any:
		if len(t) > 0 {
			return artistRefName(t[0])
		}
	}
	return ""
}
