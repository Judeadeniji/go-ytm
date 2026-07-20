package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/library"
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
	slog.Info("navigating", "status", status)
	m.cancelNavFetch()
	m.navGen++
	ctx, cancel := context.WithCancel(context.Background())
	m.navCancel = cancel
	m.navCtx = ctx
	m.pageLoading = true
	m.pageErr = ""
	m.setStatus(status)
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
	m.setStatus("Searching…")
	m.setMainContent()
	return m, doSearchFiltered(m.ytmapiClient, query, m.searchFilter, m.navGen, ctx)
}

func (m Model) openArtist(channelID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading artist…")
	if cached, ok := m.pageCache["artist_"+channelID].(*ytmapi.ArtistPage); ok {
		m.artistPage = cached
		m.pageLoading = false
		m.stack.ReplaceOrPush(Screen{Kind: ScreenArtist, ID: channelID, Title: cached.Name})
		m.setStatus("Updating " + cached.Name + "...")
		m.setMainContent()
	}
	return m, fetchArtist(m.ytmapiClient, channelID, m.navGen, m.navCtx)
}

func (m Model) openAlbum(browseID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading album…")
	if cached, ok := m.pageCache["album_"+browseID].(*ytmapi.AlbumPage); ok {
		m.albumPage = cached
		m.pageLoading = false
		m.stack.ReplaceOrPush(Screen{Kind: ScreenAlbum, ID: browseID, Title: cached.Title})
		m.setStatus("Updating " + cached.Title + "...")
		m.setMainContent()
	}
	return m, fetchAlbum(m.ytmapiClient, browseID, m.navGen, m.navCtx)
}

func (m Model) openPodcast(browseID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading podcast…")
	if cached, ok := m.pageCache["podcast_"+browseID].(*ytmapi.PodcastPage); ok {
		m.podcastPage = cached
		m.pageLoading = false
		m.stack.ReplaceOrPush(Screen{Kind: ScreenPodcast, ID: browseID, Title: cached.Title})
		m.setStatus("Updating " + cached.Title + "...")
		m.setMainContent()
	}
	return m, fetchPodcast(m.ytmapiClient, browseID, m.navGen, m.navCtx)
}

func (m Model) openProfile(channelID string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading profile…")
	if cached, ok := m.pageCache["profile_"+channelID].(*ytmapi.UserPage); ok {
		m.userPage = cached
		m.pageLoading = false
		m.stack.ReplaceOrPush(Screen{Kind: ScreenProfile, ID: channelID, Title: cached.Name})
		m.setStatus("Updating " + cached.Name + "...")
		m.setMainContent()
	}
	return m, fetchUser(m.ytmapiClient, channelID, m.navGen, m.navCtx)
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
		m.setStatus("View Album…")
		m.markSessionDirty()
		return m.openAlbum(id)
	}
	if isAlbumAudioPlaylistID(id) {
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		m.searchInput.Blur()
		m.setStatus("View Album…")
		m.markSessionDirty()
		return m.openOlak(id)
	}

	// Resolve from song metadata when we only know the video id.
	if m.currentTrack.VideoID != "" {
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		m.searchInput.Blur()
		m.setStatus("View Album…")
		m.navGen++
		m.pageLoading = true
		m.markSessionDirty()
		ctx := m.startNavCtx()
		return m, resolvePlayingAlbum(m.ytmapiClient, m.currentTrack.VideoID, name, m.navGen, ctx)
	}

	m.setStatus("View Album unavailable")
	return m, nil
}

func isAlbumBrowseID(id string) bool {
	return strings.HasPrefix(id, "MPRE")
}

func isAlbumAudioPlaylistID(id string) bool {
	return strings.HasPrefix(id, "OLAK5uy_") || strings.HasPrefix(id, "OLA")
}

func (m Model) openPlaylist(playlistID, title, author string) (Model, tea.Cmd) {
	m = m.beginOpen("Loading playlist…")
	if cached, ok := m.pageCache["playlist_"+playlistID].(*ytmapi.PlaylistPage); ok {
		m.playlistPage = cached
		m.pageLoading = false
		m.stack.ReplaceOrPush(Screen{Kind: ScreenPlaylist, ID: playlistID, Title: cached.Title})
		m.setStatus("Updating " + cached.Title + "...")
		m.setMainContent()
	}
	return m, fetchPlaylist(m.ytmapiClient, playlistID, title, author, m.navGen, m.navCtx)
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
					m.setStatus("Loading artist…")
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
					m.setStatus("Loading album…")
					m.setMainContent()
					m.mainViewport.YOffset = 0
					ctx := m.startNavCtx()
					return m, fetchAlbum(m.ytmapiClient, sc.ID, gen, ctx)
				}
			case ScreenPlaylist:
				if m.playlistPage == nil && sc.ID != "" {
					m.pageLoading = true
					m.pageErr = ""
					m.setStatus("Loading playlist…")
					m.setMainContent()
					m.mainViewport.YOffset = 0
					ctx := m.startNavCtx()
					return m, fetchPlaylist(m.ytmapiClient, sc.ID, "", "", gen, ctx)
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
	m.setStatus("Loading: " + t.Title)
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

	// Library tab chips
	for _, f := range []string{"playlists", "songs", "albums", "artists", "downloads"} {
		if m.zone.Get("lib_tab_"+f).InBounds(mouse) {
			m.libraryTab = f
			m.listCursor = 0
			m.homeCardCursor = 0
			m.setMainContent()
			if m.hasLibraryData(f) {
				return m, nil, true
			}
			m.pageLoading = true
			return m, fetchLibraryTab(m.ytmapiClient, m.sessionStore, f), true
		}
	}

	// Downloads sub-chips (Playlists | Albums | Songs | Active)
	if m.activeMenu == "Library" && m.libraryTab == "downloads" {
		for _, f := range []string{"playlists", "albums", "songs", "active"} {
			if m.zone.Get("dl_sub_"+f).InBounds(mouse) {
				m.downloadsSubTab = f
				m.listCursor = 0
				m.setMainContent()
				return m, m.enqueueVisibleImages(m.mainWidth()), true
			}
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

	// Sidebar playlist rows (distinct zone prefix so they don't collide with library grid)
	for _, p := range m.libPlaylists {
		pid := mapStr(p, "playlistId")
		if pid == "" {
			continue
		}
		if m.zone.Get("sidebar_playlist_"+pid).InBounds(mouse) {
			return m.dispatchZone("open_playlist_"+pid, mapStr(p, "title"), mapStr(p, "description"), "")
		}
	}

	// Library playlists (grid cards)
	for _, p := range m.libPlaylists {
		pid := mapStr(p, "playlistId")
		if pid == "" {
			continue
		}
		zid := "open_playlist_" + pid
		if m.zone.Get(zid).InBounds(mouse) {
			return m.dispatchZone(zid, mapStr(p, "title"), mapStr(p, "description"), "")
		}
	}

	// Library albums / artists (grid cards)
	for _, a := range m.libAlbums {
		bid := mapStr(a, "browseId")
		if bid == "" {
			continue
		}
		zid := "open_album_" + bid
		if m.zone.Get(zid).InBounds(mouse) {
			return m.dispatchZone(zid, mapStr(a, "title"), "", "")
		}
	}
	for _, a := range m.libArtists {
		bid := mapStr(a, "browseId")
		if bid == "" {
			continue
		}
		zid := "open_artist_" + bid
		if m.zone.Get(zid).InBounds(mouse) {
			return m.dispatchZone(zid, mapStr(a, "artist"), "", "")
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

	if m.nowPlayingOpen && m.nowPlayingTab == "related" {
		for _, sec := range m.relatedTracks {
			for _, card := range sec.Contents {
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
	case strings.HasPrefix(zid, "np_tab_"):
		tab := strings.TrimPrefix(zid, "np_tab_")
		m.nowPlayingTab = tab
		m.setMainContent()
		return m, nil, true
	case zid == "retry_page":
		return m.retryCurrentPage()
	case strings.HasPrefix(zid, "play_video_"):
		id := strings.TrimPrefix(zid, "play_video_")
		mm, cmd := m.playVideo(id, title, artist, thumb, true, "")
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_artist_"):
		id := strings.TrimPrefix(zid, "open_artist_")
		mm, cmd := m.openArtist(id)
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_profile_"):
		id := strings.TrimPrefix(zid, "open_profile_")
		mm, cmd := m.openProfile(id)
		return mm, cmd, true
	case strings.HasPrefix(zid, "open_podcast_"):
		id := strings.TrimPrefix(zid, "open_podcast_")
		mm, cmd := m.openPodcast(id)
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
		mm, cmd := m.openPlaylist(id, title, artist)
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
		if strings.HasPrefix(id, "MPSPPL") {
			mm, cmd := m.openPodcast(id)
			return mm, cmd, true
		}
		mm, cmd := m.openPlaylist(id, title, artist)
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

func (m Model) toggleDownloadFocused() (tea.Model, tea.Cmd) {
	if m.downloadMgr == nil {
		m.setStatus("Downloads unavailable")
		return m, nil
	}

	var track *library.CachedTrack

	if m.nowPlayingOpen {
		if m.activePane == PaneQueue {
			if m.queueCursor >= 0 && m.queueCursor < m.queue.Len() {
				t, ok := m.queue.At(m.queueCursor)
				if ok {
					track = &library.CachedTrack{
						VideoID:  t.VideoID,
						Title:    t.Title,
						Artist:   t.Artist,
						Album:    t.Album,
						Duration: t.Duration,
					}
				}
			}
		} else {
			if m.currentTrack != nil {
				t := m.currentTrack
				track = &library.CachedTrack{
					VideoID:  t.VideoID,
					Title:    t.Title,
					Artist:   t.Artist,
					Album:    t.Album,
					Duration: t.Duration,
				}
			}
		}
	} else if m.activePane == PaneMain {
		if m.onTracklistScreen() {
			tracks := m.tracklistTracks()
			if m.trackCursor >= 0 && m.trackCursor < len(tracks) {
				t := tracks[m.trackCursor]
				albumStr := ""
				if aStr, ok := t.Album.(string); ok {
					albumStr = aStr
				} else if aRef, ok := t.Album.(ytmapi.NamedRef); ok {
					albumStr = aRef.Name
				} else if aMap, ok := t.Album.(map[string]any); ok {
					if n, ok := aMap["name"].(string); ok {
						albumStr = n
					}
				}
				track = &library.CachedTrack{
					VideoID:  t.VideoID,
					Title:    t.Title,
					Artist:   t.ArtistName(),
					Album:    albumStr,
					Duration: t.Duration,
				}
			}
		} else if m.currentScreen() == screenLibrary && m.libraryTab == "downloads" {
			if item, ok := m.downloadsFocusAt(m.listCursor); ok {
				switch item.Kind {
				case dlFocusActive:
					active := m.activeDownloadEntries()
					if item.Index >= 0 && item.Index < len(active) {
						t := active[item.Index].Track
						track = &t
					}
				case dlFocusSong:
					if item.Index >= 0 && item.Index < len(m.libDownloads) {
						t := m.libDownloads[item.Index]
						track = &t
					}
				}
			}
		} else if sc, ok := m.stack.Current(); ok && sc.Kind == ScreenArtist && m.artistPage != nil && m.listCursor >= 0 {
			// Actually tracklistTracks doesn't handle ScreenArtist, handle it here
			if m.artistPage.Songs != nil && m.listCursor < len(m.artistPage.Songs.Results) {
				t := m.artistPage.Songs.Results[m.listCursor]
				videoId, _ := t["videoId"].(string)
				title, _ := t["title"].(string)
				albumStr := ""
				if album, ok := t["album"].(map[string]any); ok {
					albumStr, _ = album["name"].(string)
				}
				artistsStr := ""
				if artists, ok := t["artists"].([]any); ok && len(artists) > 0 {
					if artMap, ok := artists[0].(map[string]any); ok {
						artistsStr, _ = artMap["name"].(string)
					}
				}
				track = &library.CachedTrack{
					VideoID:  videoId,
					Title:    title,
					Artist:   artistsStr,
					Album:    albumStr,
					Duration: "",
				}
			}
		} else if m.searchResults != nil && m.listCursor >= 0 && m.listCursor < len(m.searchResults) {
			t := m.searchResults[m.listCursor]
			if t.VideoID != "" {
				track = &library.CachedTrack{
					VideoID:  t.VideoID,
					Title:    t.Title,
					Artist:   t.Author,
					Album:    "", // search results typically lack album
					Duration: t.Duration,
				}
			}
		}
	} else if m.activePane == PaneQueue {
		if m.queueCursor >= 0 && m.queueCursor < m.queue.Len() {
			t, ok := m.queue.At(m.queueCursor)
			if ok {
				track = &library.CachedTrack{
					VideoID:  t.VideoID,
					Title:    t.Title,
					Artist:   t.Artist,
					Album:    t.Album,
					Duration: t.Duration,
				}
			}
		}
	}

	if track == nil || track.VideoID == "" {
		return m.downloadFocusedCard()
	}

	// Toggle
	isDownloaded := false
	for _, t := range m.libDownloads {
		if t.VideoID == track.VideoID {
			isDownloaded = true
			break
		}
	}

	if isDownloaded {
		_ = m.sessionStore.RemoveDownload(track.VideoID)
		
		// Remove from m.libDownloads and get path
		var path string
		for i, t := range m.libDownloads {
			if t.VideoID == track.VideoID {
				path = t.Path
				m.libDownloads = append(m.libDownloads[:i], m.libDownloads[i+1:]...)
				break
			}
		}
		
		// Delete actual file
		if path != "" {
			_ = os.Remove(path)
		}
		
		m.setStatus("Removed from downloads: " + track.Title)
	} else if m.downloadMgr.IsDownloading(track.VideoID) {
		m.downloadMgr.Cancel(track.VideoID)
		if m.dlProgress != nil {
			delete(m.dlProgress, track.VideoID)
		}
		m.setStatus("Canceled download: " + track.Title)
	} else {
		m.downloadMgr.Enqueue(*track)
		m.noteDownloadStarted(*track)
		m.setStatus("Downloading: " + track.Title)
	}

	m.setMainContent()
	return m, nil
}

func (m Model) downloadAllTracks() (tea.Model, tea.Cmd) {
	if m.downloadMgr == nil {
		m.setStatus("Downloads unavailable")
		return m, nil
	}

	var tracksToDownload []library.CachedTrack

	if m.nowPlayingOpen {
		if m.activePane == PaneQueue {
			for i := 0; i < m.queue.Len(); i++ {
				t, ok := m.queue.At(i)
				if ok && t.VideoID != "" {
					tracksToDownload = append(tracksToDownload, library.CachedTrack{
						VideoID:  t.VideoID,
						Title:    t.Title,
						Artist:   t.Artist,
						Album:    t.Album,
						Duration: t.Duration,
					})
				}
			}
		}
	} else if m.activePane == PaneMain {
		if m.onTracklistScreen() {
			tracks := m.tracklistTracks()
			for _, t := range tracks {
				if t.VideoID != "" {
					albumStr := ""
					if aStr, ok := t.Album.(string); ok {
						albumStr = aStr
					} else if aRef, ok := t.Album.(ytmapi.NamedRef); ok {
						albumStr = aRef.Name
					} else if aMap, ok := t.Album.(map[string]any); ok {
						if n, ok := aMap["name"].(string); ok {
							albumStr = n
						}
					}
					tracksToDownload = append(tracksToDownload, library.CachedTrack{
						VideoID:  t.VideoID,
						Title:    t.Title,
						Artist:   t.ArtistName(),
						Album:    albumStr,
						Duration: t.Duration,
					})
				}
			}
		} else if sc, ok := m.stack.Current(); ok && sc.Kind == ScreenArtist && m.artistPage != nil {
			if m.artistPage.Songs != nil {
				for _, t := range m.artistPage.Songs.Results {
					videoId, _ := t["videoId"].(string)
					if videoId == "" {
						continue
					}
					title, _ := t["title"].(string)
					albumStr := ""
					if album, ok := t["album"].(map[string]any); ok {
						albumStr, _ = album["name"].(string)
					}
					artistsStr := ""
					if artists, ok := t["artists"].([]any); ok && len(artists) > 0 {
						if artMap, ok := artists[0].(map[string]any); ok {
							artistsStr, _ = artMap["name"].(string)
						}
					}
					tracksToDownload = append(tracksToDownload, library.CachedTrack{
						VideoID:  videoId,
						Title:    title,
						Artist:   artistsStr,
						Album:    albumStr,
					})
				}
			}
		} else if m.searchResults != nil {
			for _, t := range m.searchResults {
				if t.VideoID != "" {
					tracksToDownload = append(tracksToDownload, library.CachedTrack{
						VideoID:  t.VideoID,
						Title:    t.Title,
						Artist:   t.Author,
						Duration: t.Duration,
					})
				}
			}
		}
	} else if m.activePane == PaneQueue {
		for i := 0; i < m.queue.Len(); i++ {
			t, ok := m.queue.At(i)
			if ok && t.VideoID != "" {
				tracksToDownload = append(tracksToDownload, library.CachedTrack{
					VideoID:  t.VideoID,
					Title:    t.Title,
					Artist:   t.Artist,
					Album:    t.Album,
					Duration: t.Duration,
				})
			}
		}
	}

	count := 0
	var trackIDs []string
	for _, track := range tracksToDownload {
		trackIDs = append(trackIDs, track.VideoID)
		isDownloaded := false
		for _, t := range m.libDownloads {
			if t.VideoID == track.VideoID {
				isDownloaded = true
				break
			}
		}
		if !isDownloaded && !m.downloadMgr.IsDownloading(track.VideoID) {
			m.downloadMgr.Enqueue(track)
			m.noteDownloadStarted(track)
			count++
		}
	}

	// Persist collection metadata when downloading an open album/playlist.
	if len(trackIDs) > 0 && m.onTracklistScreen() && m.sessionStore != nil {
		if sc, ok := m.stack.Current(); ok {
			col := library.OfflineCollection{TrackIDs: trackIDs, ID: sc.ID}
			switch sc.Kind {
			case ScreenPlaylist:
				col.Kind = "playlist"
				if m.playlistPage != nil {
					col.Title = m.playlistPage.Title
					col.Author = playlistAuthorName(m.playlistPage.Author)
					col.ThumbnailURL = firstThumbURL(m.playlistPage.Thumbnails)
				}
				if col.Title == "" {
					col.Title = sc.Title
				}
			case ScreenAlbum:
				if m.albumPage != nil {
					col.Title = m.albumPage.Title
					col.Kind = normalizeOfflineAlbumKind(m.albumPage.Type)
					col.ThumbnailURL = firstThumbURL(m.albumPage.Thumbnails)
					if len(m.albumPage.Artists) > 0 {
						col.Author = m.albumPage.Artists[0].Name
					}
				} else {
					col.Kind = "album"
					col.Title = sc.Title
				}
			}
			if col.ID != "" && col.Kind != "" {
				_ = m.sessionStore.SaveOfflineCollection(col)
				m.upsertOfflineCollection(col)
			}
		}
	}

	if count > 0 {
		m.setStatus(fmt.Sprintf("Enqueued %d tracks for download", count))
	} else {
		m.setStatus("No new tracks to download")
	}
	m.setMainContent()
	return m, nil
}

func fmtErr(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}

func (m Model) retryCurrentPage() (Model, tea.Cmd, bool) {
	m.pageErr = ""
	m.exploreErr = ""
	m.pageLoading = true
	m.exploreLoading = true
	m.setStatus("Retrying...")
	m.setMainContent()
	m.navGen++
	ctx := m.startNavCtx()

	if m.activeMenu == "Explore" {
		if m.exploreSubTab == "overview" {
			return m, fetchExplore(m.ytmapiClient, m.navGen, ctx), true
		}
		if m.exploreSubTab == "moods" {
			if m.activeMoodParams != "" {
				return m, fetchMoodPlaylists(m.ytmapiClient, m.activeMoodParams, m.navGen, ctx), true
			}
			return m, fetchMoodCategories(m.ytmapiClient, m.navGen, ctx), true
		}
		if m.exploreSubTab == "charts" {
			return m, fetchCharts(m.ytmapiClient, m.chartsCountry, m.navGen, ctx), true
		}
	}

	if sc, ok := m.stack.Current(); ok {
		switch sc.Kind {
		case ScreenArtist:
			return m, fetchArtist(m.ytmapiClient, sc.ID, m.navGen, ctx), true
		case ScreenAlbum:
			return m, fetchAlbum(m.ytmapiClient, sc.ID, m.navGen, ctx), true
		case ScreenPlaylist:
			return m, fetchPlaylist(m.ytmapiClient, sc.ID, "", "", m.navGen, ctx), true
		case ScreenSearch:
			return m, doSearchFiltered(m.ytmapiClient, m.lastSearchQuery, m.searchFilter, m.navGen, ctx), true
		}
	}

	if m.activeMenu == "Home" {
		return m, fetchHome(m.ytmapiClient), true
	}

	return m, nil, false
}

func (m Model) downloadFocusedCard() (tea.Model, tea.Cmd) {
	var playlistID, browseID, title string
	switch m.currentScreen() {
	case screenHome:
		if m.activeCarousel >= 0 && m.activeCarousel < len(m.homeCarousels) {
			c := m.homeCarousels[m.activeCarousel]
			if m.homeCardCursor >= 0 && m.homeCardCursor < len(c.Contents) {
				card := c.Contents[m.homeCardCursor]
				playlistID = card.PlaylistID
				browseID = card.BrowseID
				title = card.Title
			}
		}
	case screenExplore:
		if m.exploreSubTab == "overview" {
			cars := m.exploreOverviewCarousels()
			if m.activeCarousel >= 0 && m.activeCarousel < len(cars) {
				c := cars[m.activeCarousel]
				if m.homeCardCursor >= 0 && m.homeCardCursor < len(c.Contents) {
					card := c.Contents[m.homeCardCursor]
					playlistID = card.PlaylistID
					browseID = card.BrowseID
					title = card.Title
				}
			}
		} else if m.exploreSubTab == "moods" && m.activeMoodParams != "" && len(m.moodPlaylists) > 0 {
			row := m.listCursor
			idx := row * 2
			if idx >= 0 && idx < len(m.moodPlaylists) {
				p := m.moodPlaylists[idx]
				playlistID = mapStr(p, "playlistId")
				title = mapStr(p, "title")
			}
		}
	case screenLibrary:
		switch m.libraryTab {
		case "playlists":
			if m.homeCardCursor >= 0 && m.homeCardCursor < len(m.libPlaylists) {
				p := m.libPlaylists[m.homeCardCursor]
				playlistID = mapStr(p, "playlistId")
				title = mapStr(p, "title")
			}
		case "albums":
			if m.homeCardCursor >= 0 && m.homeCardCursor < len(m.libAlbums) {
				a := m.libAlbums[m.homeCardCursor]
				browseID = mapStr(a, "browseId")
				title = mapStr(a, "title")
			}
		}
	}

	if playlistID == "" && browseID == "" {
		return m, nil
	}

	m.setStatus("Fetching " + title + " for download...")
	m.setMainContent()
	return m, fetchListForDownload(m.ytmapiClient, playlistID, browseID)
}

