package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) mainWidth() int {
	w := m.width - 24
	if w < 0 {
		return 0
	}
	return w
}

func (m *Model) setMainContent() {
	mw := m.mainWidth()
	old := m.mainViewport.YOffset
	m.mainViewport.SetContent(m.generateMainContent(mw))
	m.mainViewport.YOffset = old
}

func (m Model) beginOpen(status string) Model {
	m.pageLoading = true
	m.pageErr = ""
	m.statusMsg = status
	m.setMainContent()
	return m
}

func (m Model) openArtist(channelID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading artist…")
	return m, fetchArtist(m.ytmapiClient, channelID)
}

func (m Model) openAlbum(browseID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading album…")
	return m, fetchAlbum(m.ytmapiClient, browseID)
}

func (m Model) openOlak(audioPlaylistID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading album…")
	return m, fetchAlbumFromAudioPlaylist(m.ytmapiClient, audioPlaylistID)
}

func (m Model) openPlaylist(playlistID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading playlist…")
	return m, fetchPlaylist(m.ytmapiClient, playlistID)
}

func (m Model) goHome() Model {
	m.stack.Clear()
	m.searchResults = nil
	m.artistPage = nil
	m.albumPage = nil
	m.playlistPage = nil
	m.pageLoading = false
	m.pageErr = ""
	m.activeMenu = "Home"
	m.setMainContent()
	m.mainViewport.YOffset = 0
	return m
}

func (m Model) popNav() Model {
	if _, ok := m.stack.Pop(); ok {
		// Clear page data for the screen we left; keep parent data if stack still has pages.
		if sc, ok := m.stack.Current(); ok {
			switch sc.Kind {
			case ScreenArtist:
				m.albumPage = nil
				m.playlistPage = nil
			case ScreenAlbum, ScreenPlaylist:
				// keep artist if we came from there
			default:
				m.artistPage = nil
				m.albumPage = nil
				m.playlistPage = nil
			}
		} else {
			m.artistPage = nil
			m.albumPage = nil
			m.playlistPage = nil
			if len(m.searchResults) == 0 {
				m.activeMenu = "Home"
			}
		}
		m.pageLoading = false
		m.pageErr = ""
		m.setMainContent()
		m.mainViewport.YOffset = 0
		return m
	}
	if len(m.searchResults) > 0 {
		m.searchResults = nil
		m.setMainContent()
		m.mainViewport.YOffset = 0
	}
	return m
}

// playVideo starts playback and optionally seeds the queue from /watch.
func (m Model) playVideo(videoID, title, artist, thumb string, seedWatch bool, watchPlaylistID string) (Model, tea.Cmd) {
	if videoID == "" {
		return m, nil
	}
	if title == "" {
		title = videoID
	}
	t := Track{
		VideoID:      videoID,
		Title:        title,
		Artist:       artist,
		ThumbnailURL: thumb,
	}
	m.queue.AppendAndSelect(t)
	m.currentTrack = &t
	m.isPlaying = true
	m.statusMsg = "Loading: " + title
	if m.onTracklistScreen() {
		m = m.syncTrackCursorToPlaying()
		m.ensureTrackCursorInView(10, 1)
		m.setMainContent()
	}

	cmds := []tea.Cmd{playTrack(m.player, m.extractor, t)}
	if seedWatch {
		cmds = append(cmds, fetchWatch(m.ytmapiClient, videoID, watchPlaylistID, false))
	}
	return m, tea.Batch(cmds...)
}

func trackFromAPI(tr ytmapi.TrackItem) Track {
	return Track{
		VideoID:      tr.VideoID,
		Title:        tr.Title,
		Artist:       tr.ArtistName(),
		ThumbnailURL: tr.ThumbURL(),
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// handleZoneClick dispatches clicks for play_* / open_* / search_filter_* zones.
func (m Model) handleZoneClick(mouse tea.MouseMsg) (Model, tea.Cmd, bool) {
	// Search filter chips
	for _, f := range []string{"", "songs", "albums", "artists", "playlists"} {
		if m.zone.Get("search_filter_"+f).InBounds(mouse) {
			m.searchFilter = f
			q := m.searchInput.Value()
			if q == "" {
				q = m.lastSearchQuery
			}
			if q == "" {
				m.setMainContent()
				return m, nil, true
			}
			m.statusMsg = "Searching…"
			return m, doSearchFiltered(m.ytmapiClient, q, f), true
		}
	}

	// Collect clickable IDs from current pages / search / home
	// Artist related / album opens
	if sc, ok := m.stack.Current(); ok {
		switch sc.Kind {
		case ScreenArtist:
			if m.artistPage != nil {
				if mm, cmd, handled := m.clickArtistZones(mouse); handled {
					return mm, cmd, true
				}
			}
		case ScreenAlbum:
			if m.albumPage != nil {
				for _, tr := range m.albumPage.Tracks {
					if tr.VideoID != "" && m.zone.Get("play_video_"+tr.VideoID).InBounds(mouse) {
						mm, cmd := m.playVideo(tr.VideoID, tr.Title, tr.ArtistName(), tr.ThumbURL(), true, m.albumPage.AudioPlaylistID)
						return mm, cmd, true
					}
				}
			}
		case ScreenPlaylist:
			if m.playlistPage != nil {
				for _, tr := range m.playlistPage.Tracks {
					if tr.VideoID != "" && m.zone.Get("play_video_"+tr.VideoID).InBounds(mouse) {
						mm, cmd := m.playVideo(tr.VideoID, tr.Title, tr.ArtistName(), tr.ThumbURL(), true, m.playlistPage.ID)
						return mm, cmd, true
					}
				}
			}
		}
	}

	// Search results
	for _, res := range m.searchResults {
		zid := searchResultZone(res)
		if zid == "" || !m.zone.Get(zid).InBounds(mouse) {
			continue
		}
		return m.dispatchZone(zid, res.Title, firstArtist(res), thumbURL(res.Thumbnails))
	}

	// Home carousels
	for _, carousel := range m.homeCarousels {
		for _, card := range carousel.Contents {
			zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID)
			if zid == "" || !m.zone.Get(zid).InBounds(mouse) {
				continue
			}
			artist := ""
			if len(card.Artists) > 0 {
				artist = card.Artists[0].Name
			}
			thumb := ""
			if len(card.Thumbnails) > 0 {
				thumb = card.Thumbnails[0].URL
			}
			return m.dispatchZone(zid, card.Title, artist, thumb)
		}
	}

	return m, nil, false
}

func (m Model) clickArtistZones(mouse tea.MouseMsg) (Model, tea.Cmd, bool) {
	a := m.artistPage
	sections := []*ytmapi.ArtistSection{a.Songs, a.Albums, a.Singles, a.Videos, a.Related}
	kinds := []string{"song", "album", "album", "video", "related"}
	for i, sec := range sections {
		if sec == nil {
			continue
		}
		for _, item := range sec.Results {
			zid := artistItemZone(kinds[i], item)
			if zid == "" || !m.zone.Get(zid).InBounds(mouse) {
				continue
			}
			title := mapStr(item, "title")
			if title == "" {
				title = mapStr(item, "artist")
			}
			return m.dispatchZone(zid, title, artistRefName(item["artist"]), "")
		}
	}
	return m, nil, false
}

func (m Model) dispatchZone(zid, title, artist, thumb string) (Model, tea.Cmd, bool) {
	switch {
	case strings.HasPrefix(zid, "play_video_"):
		id := strings.TrimPrefix(zid, "play_video_")
		mm, cmd := m.playVideo(id, title, artist, thumb, true, "")
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_artist_"):
		id := strings.TrimPrefix(zid, "open_artist_")
		mm, cmd := m.openArtist(id)
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_album_"):
		id := strings.TrimPrefix(zid, "open_album_")
		mm, cmd := m.openAlbum(id)
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_olak_"):
		id := strings.TrimPrefix(zid, "open_olak_")
		mm, cmd := m.openOlak(id)
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_playlist_"):
		id := strings.TrimPrefix(zid, "open_playlist_")
		mm, cmd := m.openPlaylist(id)
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_browse_"):
		id := strings.TrimPrefix(zid, "open_browse_")
		if strings.HasPrefix(id, "UC") {
			mm, cmd := m.openArtist(id)
			return mm, cmd, true
		}
		if strings.HasPrefix(id, "MPRE") {
			mm, cmd := m.openAlbum(id)
			return mm, cmd, true
		}
		mm, cmd := m.openPlaylist(id)
		return mm, cmd, true
	}
	return m, nil, false
}

func firstArtist(res ytmapi.SearchResult) string {
	if len(res.Artists) > 0 {
		return res.Artists[0].Name
	}
	if res.Artist != "" {
		return res.Artist
	}
	return res.Author
}

func thumbURL(thumbs []ytmapi.Thumbnail) string {
	if len(thumbs) > 0 {
		return thumbs[0].URL
	}
	return ""
}

func fmtErr(err error) string {
	return fmt.Sprintf("%v", err)
}
