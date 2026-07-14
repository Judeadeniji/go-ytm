package tui

import (
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
		QueueIndex:       m.queue.CurrentIndex(),
		ShowSearch:       len(m.searchResults) > 0,
	}
	for _, t := range m.queue.Tracks() {
		snap.Queue = append(snap.Queue, session.Track{
			VideoID:      t.VideoID,
			Title:        t.Title,
			Artist:       t.Artist,
			ThumbnailURL: t.ThumbnailURL,
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
	m.queuePanelHidden = snap.QueuePanelHidden
	m.searchFilter = snap.SearchFilter
	m.lastSearchQuery = snap.LastSearchQuery
	m.activeCarousel = snap.ActiveCarousel
	m.homeCardCursor = snap.HomeCardCursor
	m.trackCursor = snap.TrackCursor
	m.listCursor = snap.ListCursor
	m.queueCursor = snap.QueueCursor
	m.playPos = snap.PlayPos
	m.resumeSeek = snap.PlayPos
	m.isPlaying = false
	m.audioLoaded = false

	if len(snap.Queue) > 0 {
		tracks := make([]Track, 0, len(snap.Queue))
		for _, t := range snap.Queue {
			tracks = append(tracks, Track{
				VideoID:      t.VideoID,
				Title:        t.Title,
				Artist:       t.Artist,
				ThumbnailURL: t.ThumbnailURL,
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
		}
	}

	m.stack.Clear()
	for _, n := range snap.Nav {
		kind := screenKindFromString(n.Kind)
		if kind == ScreenHome || kind == ScreenSearch {
			continue
		}
		m.stack.Push(Screen{Kind: kind, ID: n.ID, Title: n.Title})
	}

	// Re-fetch the top of the stack so the page is live again.
	if sc, ok := m.stack.Current(); ok && sc.ID != "" {
		m.navGen++
		m.pageLoading = true
		switch sc.Kind {
		case ScreenArtist:
			return fetchArtist(m.ytmapiClient, sc.ID, m.navGen)
		case ScreenAlbum:
			return fetchAlbum(m.ytmapiClient, sc.ID, m.navGen)
		case ScreenPlaylist:
			return fetchPlaylist(m.ytmapiClient, sc.ID, m.navGen)
		}
	}

	if m.lastSearchQuery != "" && m.stack.IsHome() && snap.ShowSearch {
		m.navGen++
		m.pageLoading = true
		// Prefer restoring a prior search results list over empty home.
		return doSearchFiltered(m.ytmapiClient, m.lastSearchQuery, m.searchFilter, m.navGen)
	}
	return nil
}

func (m *Model) markSessionDirty() {
	m.sessionDirty = true
}
