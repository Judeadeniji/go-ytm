package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) mainWidth() int {
	_, main, _ := m.layoutWidths()
	return main
}

func (m *Model) setMainContent() {
	mw := m.mainWidth()
	old := m.mainViewport.YOffset
	m.mainViewport.SetContent(m.generateMainContent(mw))
	m.mainViewport.SetYOffset(old)
}

func (m *Model) setQueuePanelContent() {
	if !m.showQueuePanel() {
		return
	}
	_, _, right := m.layoutWidths()
	inner := right - 1 // leave room for the left border glyph
	if inner < 8 {
		inner = right
	}
	old := m.rightViewport.YOffset
	m.rightViewport.Width = inner
	m.rightViewport.Height = m.contentHeight()
	m.rightViewport.SetContent(m.generateQueuePanelContent(inner))
	m.rightViewport.SetYOffset(old)
}

func (m *Model) applyLayout() {
	left, main, right := m.layoutWidths()
	ch := m.contentHeight()
	mh := m.mainPaneHeight()

	m.leftViewport.Width = left
	m.leftViewport.Height = ch
	m.leftViewport.SetContent(m.generateSidebarContent(left))

	m.mainViewport.Width = main - 2
	if m.mainViewport.Width < 0 {
		m.mainViewport.Width = 0
	}
	m.mainViewport.Height = mh
	m.setMainContent()

	if right > 0 {
		m.setQueuePanelContent()
	}
	m.ensureNowPlayingLayout()
}

func (m Model) beginOpen(status string) Model {
	m.cancelNavFetch()
	m.navGen++
	ctx, cancel := context.WithCancel(context.Background())
	m.navCancel = cancel
	m.navCtx = ctx
	m.pageLoading = true
	m.pageErr = ""
	m.statusMsg = status
	m.setMainContent()
	return m
}

func (m Model) beginSearch(query string) (Model, tea.Cmd) {
	m.cancelNavFetch()
	m.navGen++
	ctx, cancel := context.WithCancel(context.Background())
	m.navCancel = cancel
	m.navCtx = ctx
	m.pageLoading = true
	m.pageErr = ""
	m.lastSearchQuery = query
	m.statusMsg = "Searching…"
	m.setMainContent()
	return m, doSearchFiltered(m.ytmapiClient, query, m.searchFilter, m.navGen, ctx)
}

func (m Model) openArtist(channelID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading artist…")
	return m, fetchArtist(m.ytmapiClient, channelID, m.navGen, m.navCtx)
}

func (m Model) openAlbum(browseID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading album…")
	return m, fetchAlbum(m.ytmapiClient, browseID, m.navGen, m.navCtx)
}

func (m Model) openOlak(audioPlaylistID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading album…")
	return m, fetchAlbumFromAudioPlaylist(m.ytmapiClient, audioPlaylistID, m.navGen, m.navCtx)
}

// playingAlbumRef returns the album name + browse/playlist id for the current track.
func (m Model) playingAlbumRef() (name, id string) {
	if m.currentTrack == nil {
		return "", ""
	}
	name = m.currentTrack.Album
	id = m.currentTrack.AlbumID
	if m.songDetails != nil && m.songDetails.Album != nil {
		if m.songDetails.Album.Name != "" {
			name = m.songDetails.Album.Name
		}
		if m.songDetails.Album.ID != "" {
			id = m.songDetails.Album.ID
		}
	}
	return name, id
}

// goToPlayingAlbum opens the Album/Single/EP page for the playing song ("View Album").
// It does not open community playlists — only MPRE* / OLAK5uy_* release ids.
func (m Model) goToPlayingAlbum() (Model, tea.Cmd) {
	if m.currentTrack == nil {
		return m, nil
	}
	name, id := m.playingAlbumRef()

	if isAlbumBrowseID(id) {
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		m.searchInput.Blur()
		m.statusMsg = "View Album…"
		m.markSessionDirty()
		return m.openAlbum(id)
	}
	if isAlbumAudioPlaylistID(id) {
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		m.searchInput.Blur()
		m.statusMsg = "View Album…"
		m.markSessionDirty()
		return m.openOlak(id)
	}

	// Resolve from song metadata when we only know the video id.
	if m.currentTrack.VideoID != "" {
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		m.searchInput.Blur()
		m.statusMsg = "View Album…"
		m.navGen++
		m.pageLoading = true
		m.markSessionDirty()
		ctx := m.startNavCtx()
		return m, resolvePlayingAlbum(m.ytmapiClient, m.currentTrack.VideoID, name, m.navGen, ctx)
	}

	m.statusMsg = "View Album unavailable"
	return m, nil
}

func isAlbumBrowseID(id string) bool {
	return strings.HasPrefix(id, "MPRE")
}

func isAlbumAudioPlaylistID(id string) bool {
	return strings.HasPrefix(id, "OLAK5uy_") || strings.HasPrefix(id, "OLA")
}

func (m Model) openPlaylist(playlistID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading playlist…")
	return m, fetchPlaylist(m.ytmapiClient, playlistID, m.navGen, m.navCtx)
}

func (m Model) goHome() Model {
	m = m.leaveDetailPages()
	m.activeMenu = "Home"
	m.markSessionDirty()
	m.setMainContent()
	m.mainViewport.YOffset = 0
	return m
}

func (m Model) openExplore() (Model, tea.Cmd) {
	m = m.leaveDetailPages()
	m.activeMenu = "Explore"
	m.exploreSubTab = "overview"
	m.markSessionDirty()
	m.exploreLoading = true
	m.exploreErr = ""
	m.setMainContent()
	m.mainViewport.YOffset = 0
	ctx := m.startNavCtx()
	return m, fetchExplore(m.ytmapiClient, m.navGen, ctx)
}

// leaveDetailPages invalidates in-flight page/search fetches and clears detail state.
func (m Model) leaveDetailPages() Model {
	m.cancelNavFetch()
	m.navGen++
	m.stack.Clear()
	m.searchResults = nil
	m.artistPage = nil
	m.albumPage = nil
	m.playlistPage = nil
	m.pageLoading = false
	m.pageErr = ""
	return m
}

func (m *Model) cancelNavFetch() {
	if m.navCancel != nil {
		m.navCancel()
		m.navCancel = nil
	}
	m.navCtx = nil
}

func (m *Model) startNavCtx() context.Context {
	m.cancelNavFetch()
	ctx, cancel := context.WithCancel(context.Background())
	m.navCancel = cancel
	m.navCtx = ctx
	return ctx
}

func (m Model) popNav() (Model, tea.Cmd) {
	if m.activeMenu == "Explore" && m.exploreSubTab != "overview" && m.stack.Len() == 0 {
		m.exploreSubTab = "overview"
		m.setMainContent()
		m.mainViewport.YOffset = 0
		return m, nil
	}

	m.cancelNavFetch()
	m.navGen++
	gen := m.navGen
	if _, ok := m.stack.Pop(); ok {
		// Clear page data for the screen we left; refetch parent if missing.
		if sc, ok := m.stack.Current(); ok {
			switch sc.Kind {
			case ScreenArtist:
				m.albumPage = nil
				m.playlistPage = nil
				if m.artistPage == nil && sc.ID != "" {
					m.pageLoading = true
					m.pageErr = ""
					m.statusMsg = "Loading artist…"
					m.setMainContent()
					m.mainViewport.YOffset = 0
					ctx := m.startNavCtx()
					return m, fetchArtist(m.ytmapiClient, sc.ID, gen, ctx)
				}
			case ScreenAlbum:
				m.playlistPage = nil
				if m.albumPage == nil && sc.ID != "" {
					m.pageLoading = true
					m.pageErr = ""
					m.statusMsg = "Loading album…"
					m.setMainContent()
					m.mainViewport.YOffset = 0
					ctx := m.startNavCtx()
					return m, fetchAlbum(m.ytmapiClient, sc.ID, gen, ctx)
				}
			case ScreenPlaylist:
				if m.playlistPage == nil && sc.ID != "" {
					m.pageLoading = true
					m.pageErr = ""
					m.statusMsg = "Loading playlist…"
					m.setMainContent()
					m.mainViewport.YOffset = 0
					ctx := m.startNavCtx()
					return m, fetchPlaylist(m.ytmapiClient, sc.ID, gen, ctx)
				}
			default:
				m.artistPage = nil
				m.albumPage = nil
				m.playlistPage = nil
			}
		} else {
			m.artistPage = nil
			m.albumPage = nil
			m.playlistPage = nil
			if len(m.searchResults) == 0 && m.activeMenu == "" {
				m.activeMenu = "Home"
			}
		}
		m.pageLoading = false
		m.pageErr = ""
		m.setMainContent()
		m.mainViewport.YOffset = 0
		return m, nil
	}
	if len(m.searchResults) > 0 {
		m.searchResults = nil
		m.setMainContent()
		m.mainViewport.YOffset = 0
	}
	return m, nil
}

// playVideo starts playback for an ad-hoc/search/home click.
// seedWatch continues with radio/related tracks after this one.
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
	// Fresh context: don't keep a stale queue under the new track.
	m.queue.SetPlaying(t)
	return m.beginPlay(t, seedWatch, watchPlaylistID)
}

// playTracklistFrom plays tracks[index] and queues only tracks after it
// from the open album/playlist (no wrap-around via /watch).
func (m Model) playTracklistFrom(index int) (Model, tea.Cmd) {
	apiTracks := m.tracklistTracks()
	if index < 0 || index >= len(apiTracks) {
		return m, nil
	}
	queued := make([]Track, 0, len(apiTracks)-index)
	for _, tr := range apiTracks[index:] {
		queued = append(queued, trackFromAPI(tr))
	}
	m.queue.SetFrom(queued, 0)
	t := queued[0]
	return m.beginPlay(t, false, "")
}

// beginPlay updates UI state and kicks off extraction for t (already selected in queue).
func (m Model) beginPlay(t Track, seedWatch bool, watchPlaylistID string) (Model, tea.Cmd) {
	m.clearCrossfadeArmState()
	m.currentTrack = &t
	m.isPlaying = true
	m.audioLoaded = false
	m.resumeSeek = 0
	m.resumeSeekTries = 0
	m.playPos = 0
	m.playDuration = 0
	m.playBuffered = 0
	m.playGen++
	gen := m.playGen
	m.cancelPlayExtract()
	ctx, cancel := context.WithCancel(context.Background())
	m.playCancel = cancel
	m.playCtx = ctx
	sideCmd := m.onTrackChanged()
	m.queueCursor = m.queue.CurrentIndex()
	m.statusMsg = "Loading: " + t.Title
	if m.onTracklistScreen() {
		m = m.syncTrackCursorToPlaying()
		m.ensureTrackCursorInView(10, 1)
		m.setMainContent()
	}
	m.applyLayout()
	m.setQueuePanelContent()
	m.markSessionDirty()

	cachedURL, _ := m.peekStreamCache(t.VideoID)
	cmds := []tea.Cmd{
		playTrackResolved(m.extractor, t, gen, ctx, cachedURL),
		m.enqueueVisibleImages(m.mainWidth()),
		sideCmd,
	}
	if seedWatch {
		cmds = append(cmds, fetchWatch(m.ytmapiClient, t.VideoID, watchPlaylistID, false, gen))
	}
	if warm := m.ensureUpcomingPreloaded(); warm != nil {
		cmds = append(cmds, warm)
	}
	// Stop must finish before extract/load so a concurrent Stop can't kill the new track.
	return m, tea.Sequence(stopPlayback(m.player), tea.Batch(cmds...))
}

func (m Model) togglePlayPause() (Model, tea.Cmd) {
	if m.currentTrack == nil {
		return m, nil
	}
	// Restored session: queue remembers the track but mpv hasn't loaded it yet.
	if !m.audioLoaded {
		cmd := m.cmdResumeUnloadedTrack()
		m.markSessionDirty()
		if m.onTracklistScreen() {
			m.setMainContent()
		}
		return m, cmd
	}
	m.isPlaying = !m.isPlaying
	m.markSessionDirty()
	if m.onTracklistScreen() {
		m.setMainContent()
	}
	return m, togglePause(m.player)
}

func trackFromAPI(tr ytmapi.TrackItem) Track {
	album, albumID := ytmapi.AlbumRef(tr.Album)
	return Track{
		VideoID:      tr.VideoID,
		Title:        tr.Title,
		Artist:       tr.ArtistName(),
		ArtistID:     tr.ArtistChannelID(),
		Album:        album,
		AlbumID:      albumID,
		Duration:     tr.DurationLabel(),
		ThumbnailURL: tr.ThumbURL(),
		IsExplicit:   tr.IsExplicit,
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
			mm, cmd := m.beginSearch(q)
			return mm, cmd, true
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
				tracks := playableTracks(m.albumPage.Tracks)
				for i, tr := range tracks {
					if tr.VideoID != "" && m.zone.Get("play_video_"+tr.VideoID).InBounds(mouse) {
						mm, cmd := m.playTracklistFrom(i)
						return mm, cmd, true
					}
				}
			}
		case ScreenPlaylist:
			if m.playlistPage != nil {
				tracks := playableTracks(m.playlistPage.Tracks)
				for i, tr := range tracks {
					if tr.VideoID != "" && m.zone.Get("play_video_"+tr.VideoID).InBounds(mouse) {
						mm, cmd := m.playTracklistFrom(i)
						return mm, cmd, true
					}
				}
			}
		}
	}

	// Explore moods — handled entirely within exploreSubTab == "moods".
	// activeMoodParams == "" → category tiles; activeMoodParams != "" → playlist grid.
	if m.activeMenu == "Explore" && m.exploreSubTab == "moods" {
		// Back button (shown when viewing playlists)
		if m.activeMoodParams != "" && m.zone.Get("mood_back").InBounds(mouse) {
			m.activeMoodParams = ""
			m.moodPlaylists = nil
			m.setMainContent()
			m.mainViewport.YOffset = 0
			return m, nil, true
		}

		// Playlist card clicks (when a mood is active)
		if m.activeMoodParams != "" {
			for _, p := range m.moodPlaylists {
				if pid := mapStr(p, "playlistId"); pid != "" {
					zid := "open_playlist_" + pid
					if m.zone.Get(zid).InBounds(mouse) {
						title := mapStr(p, "title")
						thumb := ""
						if thumbs, ok := p["thumbnails"].([]any); ok && len(thumbs) > 0 {
							if t, ok := thumbs[0].(map[string]any); ok {
								thumb, _ = t["url"].(string)
							}
						}
						return m.dispatchZone(zid, title, "", thumb)
					}
				}
			}
		}

		// Mood category tile clicks (when no mood is selected yet)
		if m.activeMoodParams == "" {
			for _, categories := range m.moodCategories {
				for _, cat := range categories {
					if m.zone.Get("mood_"+cat.Params).InBounds(mouse) {
						m.activeMoodParams = cat.Params
						m.moodPlaylists = nil
						m.exploreLoading = true
						m.markSessionDirty()
						m.setMainContent()
						m.mainViewport.YOffset = 0
						ctx := m.startNavCtx()
						return m, fetchMoodPlaylists(m.ytmapiClient, cat.Params, m.navGen, ctx), true
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
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
