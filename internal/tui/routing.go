package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

type ArtistMsg struct {
	Page      *ytmapi.ArtistPage
	RequestID string
	Gen       int
	Err       error
}

type AlbumMsg struct {
	Page     *ytmapi.AlbumPage
	BrowseID string
	Gen      int
	Err      error
}

// resolveAlbumMsg is the result of looking up an album release for a playing song.
type resolveAlbumMsg struct {
	Gen      int
	BrowseID string
	AudioID  string
	Name     string
	Err      error
}

type PlaylistMsg struct {
	Page *ytmapi.PlaylistPage
	Gen  int
	Err  error
}

type WatchMsg struct {
	Watch       *ytmapi.WatchPlaylist
	Gen         int
	SeedVideoID string
	Err         error
}

func fetchArtist(api *ytmapi.Client, channelID string, gen int) tea.Cmd {
	return func() tea.Msg {
		page, err := api.GetArtist(context.Background(), channelID)
		return ArtistMsg{Page: page, RequestID: channelID, Gen: gen, Err: err}
	}
}

func fetchAlbum(api *ytmapi.Client, browseID string, gen int) tea.Cmd {
	return func() tea.Msg {
		page, err := api.GetAlbum(context.Background(), browseID)
		return AlbumMsg{Page: page, BrowseID: browseID, Gen: gen, Err: err}
	}
}

func fetchAlbumFromAudioPlaylist(api *ytmapi.Client, audioPlaylistID string, gen int) tea.Cmd {
	return func() tea.Msg {
		browseID, err := api.GetAlbumBrowseID(context.Background(), audioPlaylistID)
		if err != nil {
			return AlbumMsg{Gen: gen, Err: err}
		}
		page, err := api.GetAlbum(context.Background(), browseID)
		return AlbumMsg{Page: page, BrowseID: browseID, Gen: gen, Err: err}
	}
}

// resolvePlayingAlbum finds the Album/Single/EP browse id for a video via song metadata.
func resolvePlayingAlbum(api *ytmapi.Client, videoID, fallbackName string, gen int) tea.Cmd {
	return func() tea.Msg {
		if api == nil || videoID == "" {
			return resolveAlbumMsg{Gen: gen, Name: fallbackName, Err: fmt.Errorf("unavailable")}
		}
		song, err := api.GetSong(context.Background(), videoID)
		if err != nil {
			return resolveAlbumMsg{Gen: gen, Name: fallbackName, Err: err}
		}
		name := fallbackName
		id := ""
		if song != nil && song.Album != nil {
			if song.Album.Name != "" {
				name = song.Album.Name
			}
			id = song.Album.ID
		}
		if strings.HasPrefix(id, "MPRE") {
			return resolveAlbumMsg{Gen: gen, BrowseID: id, Name: name}
		}
		if strings.HasPrefix(id, "OLAK5uy_") || strings.HasPrefix(id, "OLA") {
			return resolveAlbumMsg{Gen: gen, AudioID: id, Name: name}
		}
		return resolveAlbumMsg{
			Gen:  gen,
			Name: name,
			Err:  fmt.Errorf("not an album/EP release"),
		}
	}
}

func fetchPlaylist(api *ytmapi.Client, playlistID string, gen int) tea.Cmd {
	return func() tea.Msg {
		page, err := api.GetPlaylist(context.Background(), playlistID, 100)
		return PlaylistMsg{Page: page, Gen: gen, Err: err}
	}
}

func fetchWatch(api *ytmapi.Client, videoID, playlistID string, radio bool, gen int) tea.Cmd {
	return func() tea.Msg {
		w, err := api.GetWatchPlaylist(context.Background(), videoID, playlistID, radio, 25)
		return WatchMsg{Watch: w, Gen: gen, SeedVideoID: videoID, Err: err}
	}
}

func doSearchFiltered(api *ytmapi.Client, query, filter string, gen int) tea.Cmd {
	return func() tea.Msg {
		results, err := api.SearchFiltered(context.Background(), query, filter, 30)
		return SearchResultsMsg{Results: results, Gen: gen, Err: err}
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
