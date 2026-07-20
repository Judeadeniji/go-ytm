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
	Page       *ytmapi.PlaylistPage
	PlaylistID string
	Gen        int
	Err        error
}

type WatchMsg struct {
	Watch       *ytmapi.WatchPlaylist
	Gen         int
	SeedVideoID string
	Err         error
}

type RelatedTracksMsg struct {
	Sections []ytmapi.RelatedSection
	Gen      int
	VideoID  string
	Err      error
}

func fetchArtist(api *ytmapi.Client, channelID string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		page, err := api.GetArtist(ctx, channelID)
		return ArtistMsg{Page: page, RequestID: channelID, Gen: gen, Err: err}
	}
}

func fetchAlbum(api *ytmapi.Client, browseID string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		page, err := api.GetAlbum(ctx, browseID)
		return AlbumMsg{Page: page, BrowseID: browseID, Gen: gen, Err: err}
	}
}

func fetchAlbumFromAudioPlaylist(api *ytmapi.Client, audioPlaylistID string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		browseID, err := api.GetAlbumBrowseID(ctx, audioPlaylistID)
		if err != nil {
			return AlbumMsg{Gen: gen, Err: err}
		}
		page, err := api.GetAlbum(ctx, browseID)
		return AlbumMsg{Page: page, BrowseID: browseID, Gen: gen, Err: err}
	}
}

// resolvePlayingAlbum finds the Album/Single/EP browse id for a video via song metadata.
func resolvePlayingAlbum(api *ytmapi.Client, videoID, fallbackName string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		if api == nil || videoID == "" {
			return resolveAlbumMsg{Gen: gen, Name: fallbackName, Err: fmt.Errorf("unavailable")}
		}
		song, err := api.GetSong(ctx, videoID)
		if err != nil {
			return resolveAlbumMsg{Gen: gen, Name: fallbackName, Err: err}
		}
		name := fallbackName
		var id string
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

func fetchPlaylist(api *ytmapi.Client, playlistID, title, author string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		page, err := api.GetPlaylist(ctx, playlistID, title, author, 100)
		return PlaylistMsg{Page: page, PlaylistID: playlistID, Gen: gen, Err: err}
	}
}

func fetchWatch(api *ytmapi.Client, videoID, playlistID string, radio bool, gen int) tea.Cmd {
	return func() tea.Msg {
		w, err := api.GetWatchPlaylist(context.Background(), videoID, playlistID, radio, 25)
		return WatchMsg{Watch: w, Gen: gen, SeedVideoID: videoID, Err: err}
	}
}

func fetchRelatedTracks(api *ytmapi.Client, browseID, videoID string, gen int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		sections, err := api.GetSongRelated(ctx, browseID)
		return RelatedTracksMsg{Sections: sections, Gen: gen, VideoID: videoID, Err: err}
	}
}

func doSearchFiltered(api *ytmapi.Client, query, filter string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		results, err := api.SearchFiltered(ctx, query, filter, 30)
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

// ── Explore Routing ──────────────────────────────────────────────────────────

type ExploreMsg struct {
	Data *ytmapi.ExploreData
	Gen  int
	Err  error
}

func fetchExplore(api *ytmapi.Client, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		data, err := api.GetExplore(ctx)
		return ExploreMsg{Data: data, Gen: gen, Err: err}
	}
}

type MoodCategoriesMsg struct {
	Categories map[string][]ytmapi.MoodCategory
	Gen        int
	Err        error
}

func fetchMoodCategories(api *ytmapi.Client, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		data, err := api.GetMoodCategories(ctx)
		return MoodCategoriesMsg{Categories: data, Gen: gen, Err: err}
	}
}

type MoodPlaylistsMsg struct {
	Playlists []map[string]any
	Params    string
	Gen       int
	Err       error
}

func fetchMoodPlaylists(api *ytmapi.Client, params string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		data, err := api.GetMoodPlaylists(ctx, params)
		return MoodPlaylistsMsg{Playlists: data, Params: params, Gen: gen, Err: err}
	}
}

type ChartsMsg struct {
	Data    *ytmapi.ChartsData
	Country string
	Gen     int
	Err     error
}

func fetchCharts(api *ytmapi.Client, country string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		data, err := api.GetCharts(ctx, country)
		return ChartsMsg{Data: data, Country: country, Gen: gen, Err: err}
	}
}

type PodcastMsg struct {
	Page     *ytmapi.PodcastPage
	BrowseID string
	Gen      int
	Err      error
}

func fetchPodcast(api *ytmapi.Client, browseID string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		page, err := api.GetPodcast(ctx, browseID)
		return PodcastMsg{Page: page, BrowseID: browseID, Gen: gen, Err: err}
	}
}

type UserMsg struct {
	Page      *ytmapi.UserPage
	ChannelID string
	Gen       int
	Err       error
}

func fetchUser(api *ytmapi.Client, channelID string, gen int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		page, err := api.GetUser(ctx, channelID)
		return UserMsg{Page: page, ChannelID: channelID, Gen: gen, Err: err}
	}
}
