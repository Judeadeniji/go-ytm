package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/library"
	"github.com/judeadeniji/go-ytm/internal/session"
)

type sessionLoadedMsg struct {
	Snap *session.Snapshot
	Err  error
}

type sessionSavedMsg struct {
	Err error
}

type sessionPersistTickMsg time.Time

func loadSession(db *library.DB) tea.Cmd {
	return func() tea.Msg {
		if db == nil {
			return sessionLoadedMsg{}
		}
		snap, err := db.LoadSession()
		return sessionLoadedMsg{Snap: snap, Err: err}
	}
}

func saveSession(db *library.DB, snap session.Snapshot) tea.Cmd {
	return func() tea.Msg {
		if db == nil {
			return sessionSavedMsg{}
		}
		return sessionSavedMsg{Err: db.SaveSession(snap)}
	}
}

func closeSession(db *library.DB) tea.Cmd {
	return func() tea.Msg {
		if db == nil {
			return nil
		}
		_ = db.Close()
		return nil
	}
}

func tickSessionPersist() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return sessionPersistTickMsg(t)
	})
}

func (m Model) snapshot() session.Snapshot {
	dur := m.playDuration
	if dur < 0.5 {
		dur = m.effectiveDuration()
	}
	snap := session.Snapshot{
		ActiveMenu:       m.activeMenu,
		QueuePanelHidden: m.queuePanelHidden,
		SearchFilter:     m.searchFilter,
		LastSearchQuery:  m.lastSearchQuery,
		ActiveCarousel:   m.activeCarousel,
		HomeCardCursor:   m.homeCardCursor,
		TrackCursor:      m.trackCursor,
		ListCursor:       m.listCursor,
		QueueCursor:      m.queueCursor,
		PlayPos:          m.playPos,
		PlayDuration:     dur,
		Volume:           m.volume,
		Muted:            m.muted,
		Normalize:        m.normalize,
		SilenceSkip:      m.silenceSkip,
		Tempo:            m.tempo,
		Pitch:            m.pitch,
		EQPreset:         m.eqPreset,
		RepeatMode:       m.repeatMode,
		Shuffle:          m.shuffle,
		Crossfade:        m.crossfade,
		CrossfadeSec:     session.ClampCrossfadeSec(m.crossfadeSec),
		WasPlaying:       false, // always restore paused; ignore prior play flag
		NowPlayingOpen:   m.nowPlayingOpen,
		QueueIndex:       m.queue.CurrentIndex(),
		ShowSearch:       len(m.searchResults) > 0,
		ExploreSubTab:    m.exploreSubTab,
		LibraryTab:       m.libraryTab,
	}
	for _, t := range m.queue.Tracks() {
		snap.Queue = append(snap.Queue, session.Track{
			VideoID:      t.VideoID,
			Title:        t.Title,
			Artist:       t.Artist,
			ArtistID:     t.ArtistID,
			Album:        t.Album,
			AlbumID:      t.AlbumID,
			Duration:     t.Duration,
			ThumbnailURL: t.ThumbnailURL,
			IsExplicit:   t.IsExplicit,
		})
	}
	for _, sc := range m.stack.Items() {
		snap.Nav = append(snap.Nav, session.NavItem{
			Kind:  screenKindString(sc.Kind),
			ID:    sc.ID,
			Title: sc.Title,
		})
	}
	return snap
}

func screenKindString(k ScreenKind) string {
	switch k {
	case ScreenArtist:
		return "artist"
	case ScreenAlbum:
		return "album"
	case ScreenPlaylist:
		return "playlist"
	case ScreenSearch:
		return "search"
	default:
		return "home"
	}
}

func screenKindFromString(s string) ScreenKind {
	switch s {
	case "artist":
		return ScreenArtist
	case "album":
		return ScreenAlbum
	case "playlist":
		return ScreenPlaylist
	case "search":
		return ScreenSearch
	default:
		return ScreenHome
	}
}

func (m *Model) applySnapshot(snap *session.Snapshot) tea.Cmd {
	if snap == nil {
		return nil
	}
	if snap.ActiveMenu != "" {
		m.activeMenu = snap.ActiveMenu
	}
	if snap.ExploreSubTab != "" {
		m.exploreSubTab = snap.ExploreSubTab
	}
	if snap.LibraryTab != "" {
		m.libraryTab = snap.LibraryTab
	}
	m.queuePanelHidden = snap.QueuePanelHidden
	m.searchFilter = snap.SearchFilter
	m.lastSearchQuery = snap.LastSearchQuery
	m.activeCarousel = snap.ActiveCarousel
	m.homeCardCursor = snap.HomeCardCursor
	m.trackCursor = snap.TrackCursor
	m.listCursor = snap.ListCursor
	m.queueCursor = snap.QueueCursor
	m.playPos = snap.PlayPos
	m.playDuration = snap.PlayDuration
	m.playBuffered = 0
	m.resumeSeek = snap.PlayPos
	m.resumeSeekTries = 0
	if snap.PlayPos >= 0.5 {
		m.resumeSeekTries = 30 // ~15s of progress-tick nudges after resume
	}
	m.isPlaying = false
	m.audioLoaded = false
	m.nowPlayingOpen = snap.NowPlayingOpen

	vol := snap.Volume
	// Schema default is 100; omit/legacy only when volume was never written.
	// Real mute-at-zero is preserved when Muted is true OR volume was explicitly saved.
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	m.volume = vol
	m.muted = snap.Muted
	m.normalize = snap.Normalize
	m.silenceSkip = snap.SilenceSkip
	m.tempo = snap.Tempo
	if m.tempo < 0.25 {
		m.tempo = 1.0
	}
	m.pitch = snap.Pitch
	m.eqPreset = snap.EQPreset
	if m.eqPreset < 0 || m.eqPreset >= len(eqPresets) {
		m.eqPreset = 0
	}
	m.repeatMode = snap.RepeatMode
	m.shuffle = snap.Shuffle
	m.crossfade = snap.Crossfade
	m.crossfadeSec = session.ClampCrossfadeSec(snap.CrossfadeSec)
	if m.crossfadeSec == 0 {
		m.crossfadeSec = session.DefaultCrossfadeSec
	}
	applyVol := applyVolumeStateCmd(m.player, m.volume, m.muted, m.normalize, m.silenceSkip, m.tempo, m.pitch, eqPresets[m.eqPreset].Filter)

	if len(snap.Queue) > 0 {
		tracks := make([]Track, 0, len(snap.Queue))
		for _, t := range snap.Queue {
			tracks = append(tracks, Track{
				VideoID:      t.VideoID,
				Title:        t.Title,
				Artist:       t.Artist,
				ArtistID:     t.ArtistID,
				Album:        t.Album,
				AlbumID:      t.AlbumID,
				Duration:     t.Duration,
				ThumbnailURL: t.ThumbnailURL,
				IsExplicit:   t.IsExplicit,
			})
		}
		m.queue.SetFrom(tracks, 0)
		idx := snap.QueueIndex
		if idx < 0 || idx >= len(tracks) {
			idx = 0
		}
		m.queue.JumpTo(idx)
		if cur, ok := m.queue.Current(); ok {
			cp := cur
			m.currentTrack = &cp
			m.statusMsg = "Ready · " + cur.Title
			if m.playDuration < 0.5 {
				m.playDuration = parseClock(cur.Duration)
			}
		}
	} else {
		m.nowPlayingOpen = false
	}

	m.stack.Clear()
	for _, n := range snap.Nav {
		kind := screenKindFromString(n.Kind)
		if kind == ScreenHome || kind == ScreenSearch {
			continue
		}
		m.stack.Push(Screen{Kind: kind, ID: n.ID, Title: n.Title})
	}

	var cmds []tea.Cmd
	cmds = append(cmds, applyVol)

	// Always restore paused — auto-play races init (home fetch, volume, mpv)
	// and leaves a half-loaded player that only recovers after changing tracks.

	// Re-fetch the top of the stack so the page is live again.
	if sc, ok := m.stack.Current(); ok && sc.ID != "" {
		m.navGen++
		m.pageLoading = true
		ctx := m.startNavCtx()
		switch sc.Kind {
		case ScreenArtist:
			cmds = append(cmds, fetchArtist(m.ytmapiClient, sc.ID, m.navGen, ctx))
		case ScreenAlbum:
			cmds = append(cmds, fetchAlbum(m.ytmapiClient, sc.ID, m.navGen, ctx))
		case ScreenPlaylist:
			cmds = append(cmds, fetchPlaylist(m.ytmapiClient, sc.ID, "", "", m.navGen, ctx))
		}
		return tea.Batch(cmds...)
	}

	if m.lastSearchQuery != "" && m.stack.IsHome() && snap.ShowSearch {
		m.navGen++
		m.pageLoading = true
		if m.lastSearchQuery != "" {
			m.searchInput.SetValue(m.lastSearchQuery)
			m.searchInput.Blur()
		}
		ctx := m.startNavCtx()
		cmds = append(cmds, doSearchFiltered(m.ytmapiClient, m.lastSearchQuery, m.searchFilter, m.navGen, ctx))
	} else if m.activeMenu == "Explore" && m.stack.IsHome() {
		m.navGen++
		ctx := m.startNavCtx()
		switch m.exploreSubTab {
		case "moods":
			m.moodCatsLoading = true
			cmds = append(cmds, fetchMoodCategories(m.ytmapiClient, m.navGen, ctx))
		case "charts":
			m.chartsLoading = true
			cmds = append(cmds, fetchCharts(m.ytmapiClient, m.chartsCountry, m.navGen, ctx))
		default:
			m.exploreSubTab = "overview"
			m.exploreLoading = true
			cmds = append(cmds, fetchExplore(m.ytmapiClient, m.navGen, ctx))
		}
	} else if m.activeMenu == "Library" && m.stack.IsHome() {
		m.pageLoading = true
		cmds = append(cmds, fetchLibraryTab(m.ytmapiClient, m.sessionStore, m.libraryTab))
	}
	return tea.Batch(cmds...)
}

// cmdResumeUnloadedTrack starts extraction+load for a restored track that is not yet in mpv.
func (m *Model) cmdResumeUnloadedTrack() tea.Cmd {
	if m.currentTrack == nil || m.audioLoaded {
		return nil
	}
	t := *m.currentTrack
	if t.VideoID == "" {
		m.statusMsg = "Cannot resume: missing video id"
		return nil
	}
	if m.resumeSeek < 0.5 && m.playPos >= 0.5 {
		m.resumeSeek = m.playPos
	}
	if m.resumeSeek >= 0.5 && m.resumeSeekTries <= 0 {
		m.resumeSeekTries = 30
	}
	m.isPlaying = true
	m.playGen++
	gen := m.playGen
	m.clearCrossfadeArmState()
	m.cancelPlayExtract()
	ctx, cancel := context.WithCancel(context.Background())
	m.playCancel = cancel
	m.playCtx = ctx
	if m.resumeSeek >= 0.5 {
		m.statusMsg = fmt.Sprintf("Resuming at %s: %s", formatClock(m.resumeSeek), t.Title)
	} else {
		m.statusMsg = "Resuming: " + t.Title
	}
	// Match beginPlay: stop before extract/load so mpv can't race a stale loadfile.
	cachedURL, _ := m.peekStreamCache(t.VideoID)
	cmds := []tea.Cmd{playTrackResolved(m.extractor, t, gen, ctx, cachedURL)}
	if warm := m.ensureUpcomingPreloaded(); warm != nil {
		cmds = append(cmds, warm)
	}
	return tea.Sequence(stopPlayback(m.player), tea.Batch(cmds...))
}

func (m *Model) clearResumeSeek() {
	m.resumeSeek = 0
	m.resumeSeekTries = 0
}

func (m *Model) markSessionDirty() {
	m.sessionDirty = true
}

// markPlayPosDirty marks the session dirty when the displayed second changes,
// so we don't rewrite the full queue to SQLite on every 500ms progress tick.
func (m *Model) markPlayPosDirty() {
	sec := int(m.playPos)
	if sec == m.lastSessionPosSec {
		return
	}
	m.lastSessionPosSec = sec
	m.sessionDirty = true
}
