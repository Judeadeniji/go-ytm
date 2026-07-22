package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// screenKind identifies what the main pane is showing for keyboard routing.
type screenKind int

const (
	screenHome screenKind = iota
	screenSearch
	screenArtist
	screenAlbum
	screenPlaylist
	screenPodcast
	screenProfile
	screenExplore
	screenLibrary
	screenOther
)

func (m Model) currentScreen() screenKind {
	if sc, ok := m.stack.Current(); ok {
		switch sc.Kind {
		case ScreenArtist:
			return screenArtist
		case ScreenAlbum:
			return screenAlbum
		case ScreenPlaylist:
			return screenPlaylist
		case ScreenPodcast:
			return screenPodcast
		case ScreenProfile:
			return screenProfile
		case ScreenSearch:
			return screenSearch
		}
	}
	if len(m.searchResults) > 0 {
		return screenSearch
	}
	if m.activeMenu == "Home" || m.activeMenu == "" {
		return screenHome
	}
	if m.activeMenu == "Explore" {
		return screenExplore
	}
	if m.activeMenu == "Library" {
		return screenLibrary
	}
	return screenOther
}

func (m Model) onHomeScreen() bool {
	return m.currentScreen() == screenHome
}

// artistFocusItem is one keyboard-addressable row on an artist page.
type artistFocusItem struct {
	Kind  string
	Title string
	Item  map[string]any
}

func (m Model) artistFocusItems() []artistFocusItem {
	if m.artistPage == nil {
		return nil
	}
	a := m.artistPage
	var out []artistFocusItem
	add := func(kind, title string, results []map[string]any) {
		for i, item := range results {
			if i >= 12 {
				break
			}
			if artistItemZone(kind, item) == "" {
				continue
			}
			out = append(out, artistFocusItem{Kind: kind, Title: title, Item: item})
		}
	}
	if a.Songs != nil {
		add("song", "", a.Songs.Results)
	}
	if a.Albums != nil {
		add("album", "Albums", a.Albums.Results)
	}
	if a.Singles != nil {
		add("album", "Singles & EPs", a.Singles.Results)
	}
	if a.Videos != nil {
		add("video", "Videos", a.Videos.Results)
	}
	if a.Related != nil {
		add("related", "Fans Also Like", a.Related.Results)
	}
	return out
}

func (m Model) profileFocusItems() []artistFocusItem {
	if m.userPage == nil {
		return nil
	}
	u := m.userPage
	var out []artistFocusItem
	add := func(kind, title string, results []map[string]any) {
		for _, item := range results {
			if artistItemZone(kind, item) == "" {
				continue
			}
			out = append(out, artistFocusItem{Kind: kind, Title: title, Item: item})
		}
	}
	if u.Playlists != nil {
		add("playlists", "Playlists", u.Playlists.Results)
	}
	if u.Videos != nil {
		add("videos", "Videos", u.Videos.Results)
	}
	return out
}

func (m *Model) ensureArtistCarouselCursorVisible(items []artistFocusItem) {
	if m.listCursor < 0 || m.listCursor >= len(items) {
		return
	}
	focused := items[m.listCursor]
	if focused.Title == "" {
		return
	} // Songs aren't carousels

	// Find the local index of this item in its carousel
	localIndex := 0
	for i := 0; i < m.listCursor; i++ {
		if items[i].Title == focused.Title {
			localIndex++
		}
	}

	contentWidth := m.mainWidth() - 2
	cardWidth := 28
	maxVisible := contentWidth / cardWidth
	if maxVisible < 1 {
		maxVisible = 1
	}

	title := focused.Title
	offset := m.carouselOffsets[title]
	if localIndex < offset {
		m.carouselOffsets[title] = localIndex
	} else if localIndex >= offset+maxVisible {
		m.carouselOffsets[title] = localIndex - maxVisible + 1
	}
	if m.carouselOffsets[title] < 0 {
		m.carouselOffsets[title] = 0
	}
}

func clampIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// moveListFocus moves the active list cursor by delta. handled=true means
// the key should not fall through to viewport scrolling.
func (m Model) moveListFocus(delta int) (Model, bool) {
	switch m.activePane {
	case PaneSidebar:
		n := m.sidebarFocusCount()
		if n == 0 {
			return m, false
		}
		m.listCursor = clampIndex(m.listCursor+delta, n)
		m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
		m.ensureSidebarCursorInView()
		return m, true

	case PaneQueue:
		if !m.showQueuePanel() {
			return m, false
		}
		// Details (and future inspector tabs) scroll as a document, not a track list.
		if m.railTab != RailQueue {
			if delta < 0 {
				m.rightViewport.ScrollUp(1)
			} else {
				m.rightViewport.ScrollDown(1)
			}
			return m, true
		}
		if m.queue.IsEmpty() {
			return m, false
		}
		m.queueCursor = clampIndex(m.queueCursor+delta, m.queue.Len())
		m.setQueuePanelContent()
		m.ensureQueueCursorInView()
		return m, true
	}

	// PaneMain (and default)
	switch m.currentScreen() {
	case screenAlbum, screenPlaylist, screenPodcast:
		return m.moveTrackCursor(delta), true

	case screenSearch:
		results := m.searchFocusResults()
		if len(results) == 0 {
			return m, false
		}
		m.listCursor = clampIndex(m.listCursor+delta, len(results))
		m.setMainContent()
		m.ensureListCursorInView(3, 3)
		return m, true

	case screenArtist:
		items := m.artistFocusItems()
		if len(items) == 0 {
			return m, false
		}
		m.listCursor = clampIndex(m.listCursor+delta, len(items))
		m.ensureArtistCarouselCursorVisible(items)
		m.setMainContent()
		m.ensureListCursorInView(6, 1)
		return m, true

	case screenProfile:
		items := m.profileFocusItems()
		if len(items) == 0 {
			return m, false
		}
		m.listCursor = clampIndex(m.listCursor+delta, len(items))
		m.setMainContent()
		m.ensureListCursorInView(6, 1)
		return m, true

	case screenHome:
		if len(m.homeCarousels) == 0 {
			return m, false
		}
		m.activeCarousel = clampIndex(m.activeCarousel+delta, len(m.homeCarousels))
		m.homeCardCursor = m.carouselOffsets[m.homeCarousels[m.activeCarousel].Title]
		m.ensureHomeCardVisible()
		m.setMainContent()
		return m, true

	case screenExplore:
		if m.exploreSubTab == "overview" {
			cars := m.exploreOverviewCarousels()
			if len(cars) == 0 {
				return m, false
			}
			m.activeCarousel = clampIndex(m.activeCarousel+delta, len(cars))
			m.homeCardCursor = m.carouselOffsets[cars[m.activeCarousel].Title]
			m.ensureHomeCardVisible()
			m.setMainContent()
			return m, true
		}
		// moods playlist grid: up/down scrolls rows (2 playlists per row)
		if m.exploreSubTab == "moods" && m.activeMoodParams != "" && len(m.moodPlaylists) > 0 {
			rows := (len(m.moodPlaylists) + 1) / 2
			m.listCursor = clampIndex(m.listCursor+delta, rows)
			m.setMainContent()
			return m, true
		}
		return m, false

	case screenLibrary:
		switch m.libraryTab {
		case "playlists":
			if len(m.libPlaylists) == 0 {
				return m, false
			}
			m.listCursor = clampIndex(m.listCursor+delta, len(m.libPlaylists))
			m.ensureListCursorInViewGrid(m.listCursor, (m.mainWidth()-2)/(artWidth+2), artHeight+4)
			m.setMainContent()
			return m, true
		case "albums":
			if len(m.libAlbums) == 0 {
				return m, false
			}
			m.listCursor = clampIndex(m.listCursor+delta, len(m.libAlbums))
			m.ensureListCursorInViewGrid(m.listCursor, (m.mainWidth()-2)/(artWidth+2), artHeight+4)
			m.setMainContent()
			return m, true
		case "artists":
			if len(m.libArtists) == 0 {
				return m, false
			}
			m.listCursor = clampIndex(m.listCursor+delta, len(m.libArtists))
			m.ensureListCursorInViewGrid(m.listCursor, (m.mainWidth()-2)/(artWidth+2), artHeight+4)
			m.setMainContent()
			return m, true
		case "songs":
			if len(m.libSongs) == 0 {
				return m, false
			}
			m.listCursor = clampIndex(m.listCursor+delta, len(m.libSongs))
			m.ensureListCursorInView(3, 3)
			m.setMainContent()
			return m, true
		case "downloads":
			items := m.downloadsFocusItems()
			if len(items) == 0 {
				return m, false
			}
			m.listCursor = clampIndex(m.listCursor+delta, len(items))
			if m.downloadsSubTab == "songs" || m.downloadsSubTab == "active" {
				m.ensureListCursorInView(6, 1)
			} else {
				cols := (m.mainWidth() - 2) / (artWidth + 2)
				if cols < 1 {
					cols = 1
				}
				m.ensureListCursorInViewGrid(m.listCursor, cols, artHeight+4)
			}
			m.setMainContent()
			return m, true
		}
		return m, false
	}

	return m, false
}

// moveArtistCarousel shifts focus horizontally within an artist carousel.
func (m Model) moveArtistCarousel(delta int) (Model, bool) {
	if m.currentScreen() != screenArtist {
		return m, false
	}
	items := m.artistFocusItems()
	if m.listCursor < 0 || m.listCursor >= len(items) {
		return m, false
	}
	focused := items[m.listCursor]
	if focused.Title == "" {
		return m, false // Not a carousel
	}

	target := m.listCursor + delta
	if target < 0 || target >= len(items) {
		return m, false
	}
	if items[target].Title != focused.Title {
		return m, false
	}

	m.listCursor = target
	m.ensureArtistCarouselCursorVisible(items)
	m.setMainContent()
	return m, true
}

// moveHomeCard shifts focus within the active home or explore carousel (horizontal).
func (m *Model) moveHomeCard(delta int) (tea.Model, tea.Cmd) {
	if m.currentScreen() == screenExplore && m.exploreSubTab == "overview" {
		cars := m.exploreOverviewCarousels()
		if len(cars) == 0 {
			return *m, nil
		}
		m.activeCarousel = clampIndex(m.activeCarousel, len(cars))
		car := cars[m.activeCarousel]
		if len(car.Contents) == 0 {
			return *m, nil
		}
		m.homeCardCursor = clampIndex(m.homeCardCursor+delta, len(car.Contents))
		m.ensureHomeCardVisible()
		m.setMainContent()
		return *m, m.enqueueVisibleImages(m.mainWidth())
	}
	if m.currentScreen() == screenLibrary {
		var count int
		switch m.libraryTab {
		case "playlists":
			count = len(m.libPlaylists)
		case "albums":
			count = len(m.libAlbums)
		case "artists":
			count = len(m.libArtists)
		}
		if count == 0 {
			return *m, nil
		}
		m.homeCardCursor = clampIndex(m.homeCardCursor+delta, count)
		m.ensureHomeCardVisible()
		m.setMainContent()
		return *m, m.enqueueVisibleImages(m.mainWidth())
	}

	if !m.onHomeScreen() || len(m.homeCarousels) == 0 {
		return *m, nil
	}
	m.activeCarousel = clampIndex(m.activeCarousel, len(m.homeCarousels))
	car := m.homeCarousels[m.activeCarousel]
	if len(car.Contents) == 0 {
		return *m, nil
	}
	m.homeCardCursor = clampIndex(m.homeCardCursor+delta, len(car.Contents))
	m.ensureHomeCardVisible()
	m.setMainContent()
	return *m, m.enqueueVisibleImages(m.mainWidth())
}

func (m *Model) ensureHomeCardVisible() {
	var title string
	if m.onHomeScreen() && len(m.homeCarousels) > 0 {
		m.activeCarousel = clampIndex(m.activeCarousel, len(m.homeCarousels))
		title = m.homeCarousels[m.activeCarousel].Title
	} else if m.currentScreen() == screenExplore && m.exploreSubTab == "overview" {
		cars := m.exploreOverviewCarousels()
		if len(cars) > 0 {
			m.activeCarousel = clampIndex(m.activeCarousel, len(cars))
			title = cars[m.activeCarousel].Title
		}
	} else if m.currentScreen() == screenLibrary {
		switch m.libraryTab {
		case "playlists":
			title = "Playlists"
		case "albums":
			title = "Albums"
		case "artists":
			title = "Artists"
		}
	}
	if title == "" {
		return
	}
	contentWidth := m.mainWidth() - 2
	cardWidth := 28
	maxVisible := contentWidth / cardWidth
	if maxVisible < 1 {
		maxVisible = 1
	}
	offset := m.carouselOffsets[title]
	if m.homeCardCursor < offset {
		m.carouselOffsets[title] = m.homeCardCursor
	} else if m.homeCardCursor >= offset+maxVisible {
		m.carouselOffsets[title] = m.homeCardCursor - maxVisible + 1
	}
	if m.carouselOffsets[title] < 0 {
		m.carouselOffsets[title] = 0
	}
}

// ensureListCursorInView scrolls main viewport so an estimated cursor line stays visible.
func (m *Model) ensureListCursorInView(headerLines, rowHeight int) {
	viewH := m.mainViewport.Height
	if viewH <= 0 || rowHeight <= 0 {
		return
	}
	cursorLine := headerLines + m.listCursor*rowHeight
	top := m.mainViewport.YOffset
	bottom := top + viewH - 1
	if cursorLine < top {
		m.mainViewport.SetYOffset(cursorLine)
	} else if cursorLine+rowHeight-1 > bottom {
		m.mainViewport.SetYOffset(cursorLine + rowHeight - viewH)
	}
}

func (m *Model) ensureQueueCursorInView() {
	// Approximate layout: now-playing card (~18) + queue header (~3)
	// + optional played block + divider (~3) + 3 lines per track.
	viewH := m.rightViewport.Height
	if viewH <= 0 {
		return
	}
	cur := m.queue.CurrentIndex()
	headerLines := 21
	cursorLine := headerLines
	if cur > 0 {
		// played rows before divider
		played := cur
		if m.queueCursor < cur {
			cursorLine += m.queueCursor * 3
		} else {
			cursorLine += played*3 + 3 // divider + "Up next" label
			cursorLine += (m.queueCursor - cur) * 3
		}
	} else {
		cursorLine += m.queueCursor * 3
	}
	top := m.rightViewport.YOffset
	bottom := top + viewH - 1
	if cursorLine < top {
		m.rightViewport.SetYOffset(cursorLine)
	} else if cursorLine+2 > bottom {
		m.rightViewport.SetYOffset(cursorLine + 3 - viewH)
	}
}

// activateFocused triggers the focused list item (Enter).
func (m Model) activateFocused() (Model, tea.Cmd) {
	switch m.activePane {
	case PaneSidebar:
		// Playlist below the menu
		if plIdx := m.listCursor - len(m.menuItems); plIdx >= 0 && plIdx < len(m.libPlaylists) {
			p := m.libPlaylists[plIdx]
			pid := mapStr(p, "playlistId")
			if pid == "" {
				return m, nil
			}
			mm, cmd := m.openPlaylist(pid, mapStr(p, "title"), mapStr(p, "description"))
			mm.leftViewport.SetContent(mm.generateSidebarContent(leftSidebarWidth))
			return mm, cmd
		}
		if m.listCursor < 0 || m.listCursor >= len(m.menuItems) {
			return m, nil
		}
		item := m.menuItems[m.listCursor]
		if item == "Home" {
			m = m.goHome()
			m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
			return m, m.enqueueVisibleImages(m.mainWidth())
		}
		if item == "Explore" {
			mm, cmd := m.openExplore()
			mm.leftViewport.SetContent(mm.generateSidebarContent(leftSidebarWidth))
			return mm, tea.Batch(cmd, mm.enqueueVisibleImages(mm.mainWidth()))
		}
		if item == "Library" {
			m.activeMenu = item
			m = m.leaveDetailPages()
			m.markSessionDirty()
			m.setMainContent()
			m.mainViewport.YOffset = 0
			m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
			return m, fetchLibraryTab(m.ytmapiClient, m.sessionStore, m.libraryTab)
		}
		m.activeMenu = item
		m = m.leaveDetailPages()
		m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
		m.setMainContent()
		m.mainViewport.YOffset = 0
		return m, nil

	case PaneQueue:
		if !m.showQueuePanel() || m.railTab != RailQueue {
			return m, nil
		}
		return m.playQueueIndex(m.queueCursor)
	}

	switch m.currentScreen() {
	case screenAlbum, screenPlaylist, screenPodcast:
		return m.playFocusedTrack()

	case screenSearch:
		results := m.searchFocusResults()
		if m.listCursor < 0 || m.listCursor >= len(results) {
			return m, nil
		}
		res := results[m.listCursor]
		zid := searchResultZone(res)
		if zid == "" {
			return m, nil
		}
		mm, cmd, _ := m.dispatchZone(zid, res.Title, firstArtist(res), thumbURL(res.Thumbnails))
		return mm, cmd

	case screenArtist, screenProfile:
		var items []artistFocusItem
		if m.currentScreen() == screenProfile {
			items = m.profileFocusItems()
		} else {
			items = m.artistFocusItems()
		}
		if m.listCursor < 0 || m.listCursor >= len(items) {
			return m, nil
		}
		it := items[m.listCursor]
		zid := artistItemZone(it.Kind, it.Item)
		if zid == "" {
			return m, nil
		}
		title := mapStr(it.Item, "title")
		if title == "" {
			title = mapStr(it.Item, "artist")
		}
		mm, cmd, _ := m.dispatchZone(zid, title, artistRefName(it.Item["artist"]), "")
		return mm, cmd

	case screenHome:
		if len(m.homeCarousels) == 0 {
			return m, nil
		}
		m.activeCarousel = clampIndex(m.activeCarousel, len(m.homeCarousels))
		car := m.homeCarousels[m.activeCarousel]
		if m.homeCardCursor < 0 || m.homeCardCursor >= len(car.Contents) {
			return m, nil
		}
		card := car.Contents[m.homeCardCursor]
		zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID)
		if zid == "" {
			return m, nil
		}
		artist := ""
		if len(card.Artists) > 0 {
			artist = card.Artists[0].Name
		}
		thumb := ""
		if len(card.Thumbnails) > 0 {
			thumb = card.Thumbnails[0].URL
		}
		mm, cmd, _ := m.dispatchZone(zid, card.Title, artist, thumb)
		return mm, cmd

	case screenExplore:
		if m.exploreSubTab == "overview" {
			cars := m.exploreOverviewCarousels()
			if len(cars) == 0 {
				return m, nil
			}
			m.activeCarousel = clampIndex(m.activeCarousel, len(cars))
			car := cars[m.activeCarousel]
			if m.homeCardCursor < 0 || m.homeCardCursor >= len(car.Contents) {
				return m, nil
			}
			card := car.Contents[m.homeCardCursor]
			zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID)
			if zid == "" {
				return m, nil
			}
			artist := ""
			if len(card.Artists) > 0 {
				artist = card.Artists[0].Name
			}
			thumb := ""
			if len(card.Thumbnails) > 0 {
				thumb = card.Thumbnails[0].URL
			}
			mm, cmd, _ := m.dispatchZone(zid, card.Title, artist, thumb)
			return mm, cmd
		}
		if m.exploreSubTab == "moods" && m.activeMoodParams != "" && len(m.moodPlaylists) > 0 {
			// listCursor is the row index; within a row the left cell (col 0) is focused
			row := m.listCursor
			idx := row * 2 // left cell of that row
			if idx < 0 || idx >= len(m.moodPlaylists) {
				return m, nil
			}
			p := m.moodPlaylists[idx]
			pid := mapStr(p, "playlistId")
			if pid == "" {
				return m, nil
			}
			title := mapStr(p, "title")
			thumb := ""
			if thumbs, ok := p["thumbnails"].([]any); ok && len(thumbs) > 0 {
				if t, ok := thumbs[0].(map[string]any); ok {
					thumb, _ = t["url"].(string)
				}
			}
			mm, cmd, _ := m.dispatchZone("open_playlist_"+pid, title, "", thumb)
			return mm, cmd
		}
		return m, nil

	case screenLibrary:
		switch m.libraryTab {
		case "playlists":
			if m.homeCardCursor < 0 || m.homeCardCursor >= len(m.libPlaylists) {
				return m, nil
			}
			p := m.libPlaylists[m.homeCardCursor]
			pid := mapStr(p, "playlistId")
			if pid != "" {
				mm, cmd, _ := m.dispatchZone("open_playlist_"+pid, mapStr(p, "title"), mapStr(p, "author"), "")
				return mm, cmd
			}
		case "albums":
			if m.homeCardCursor < 0 || m.homeCardCursor >= len(m.libAlbums) {
				return m, nil
			}
			a := m.libAlbums[m.homeCardCursor]
			bid := mapStr(a, "browseId")
			if bid != "" {
				mm, cmd, _ := m.dispatchZone("open_album_"+bid, mapStr(a, "title"), "", "")
				return mm, cmd
			}
		case "artists":
			if m.homeCardCursor < 0 || m.homeCardCursor >= len(m.libArtists) {
				return m, nil
			}
			a := m.libArtists[m.homeCardCursor]
			bid := mapStr(a, "browseId")
			if bid != "" {
				mm, cmd, _ := m.dispatchZone("open_artist_"+bid, mapStr(a, "artist"), "", "")
				return mm, cmd
			}
		case "songs":
			if m.listCursor < 0 || m.listCursor >= len(m.libSongs) {
				return m, nil
			}
			s := m.libSongs[m.listCursor]
			vid := mapStr(s, "videoId")
			if vid != "" {
				mm, cmd, _ := m.dispatchZone("play_video_"+vid, mapStr(s, "title"), "", "")
				return mm, cmd
			}
		case "downloads":
			item, ok := m.downloadsFocusAt(m.listCursor)
			if !ok {
				return m, nil
			}
			switch item.Kind {
			case dlFocusActive:
				// Cancel stays on `d`; Enter is a no-op for active rows.
				return m, nil
			case dlFocusPlaylist:
				cols := m.offlineCollectionsByKind("playlist")
				if item.Index < 0 || item.Index >= len(cols) {
					return m, nil
				}
				c := cols[item.Index]
				mm, cmd, _ := m.dispatchZone("open_playlist_"+c.ID, c.Title, c.Author, "")
				return mm, cmd
			case dlFocusAlbum, dlFocusEP, dlFocusSingle:
				kind := "album"
				if item.Kind == dlFocusEP {
					kind = "ep"
				} else if item.Kind == dlFocusSingle {
					kind = "single"
				}
				cols := m.offlineCollectionsByKind(kind)
				if item.Index < 0 || item.Index >= len(cols) {
					return m, nil
				}
				c := cols[item.Index]
				mm, cmd, _ := m.dispatchZone("open_album_"+c.ID, c.Title, c.Author, "")
				return mm, cmd
			case dlFocusSong:
				if item.Index < 0 || item.Index >= len(m.libDownloads) {
					return m, nil
				}
				d := m.libDownloads[item.Index]
				if d.VideoID != "" {
					mm, cmd, _ := m.dispatchZone("play_video_"+d.VideoID, d.Title, d.Artist, "")
					return mm, cmd
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) moveSuggestionFocus(delta int) Model {
	if len(m.searchSuggestions) == 0 {
		return m
	}
	m.listCursor += delta
	if m.listCursor < -1 {
		m.listCursor = -1
	} else if m.listCursor >= len(m.searchSuggestions) {
		m.listCursor = len(m.searchSuggestions) - 1
	}
	return m
}

// focusedSearchResult reports whether search result i (display order) has keyboard focus.
func (m Model) focusedSearchResult(i int) bool {
	return m.activePane == PaneMain && m.currentScreen() == screenSearch && m.listCursor == i
}

// focusedArtistZone reports whether the artist row with this zone is focused.
func (m Model) focusedArtistZone(zid string) bool {
	if zid == "" || m.activePane != PaneMain || m.currentScreen() != screenArtist {
		return false
	}
	items := m.artistFocusItems()
	if m.listCursor < 0 || m.listCursor >= len(items) {
		return false
	}
	it := items[m.listCursor]
	return artistItemZone(it.Kind, it.Item) == zid
}

// focusedHomeCard reports keyboard focus on a home or explore overview card.
func (m Model) focusedHomeCard(carouselIndex, cardIndex int) bool {
	if m.activePane != PaneMain {
		return false
	}
	if m.currentScreen() == screenHome || (m.currentScreen() == screenExplore && m.exploreSubTab == "overview") {
		return m.activeCarousel == carouselIndex && m.homeCardCursor == cardIndex
	}
	if m.currentScreen() == screenLibrary && (m.libraryTab == "playlists" || m.libraryTab == "albums" || m.libraryTab == "artists") {
		return m.activeCarousel == carouselIndex && m.homeCardCursor == cardIndex
	}
	return false
}

// focusedMenuItem reports sidebar keyboard focus.
func (m Model) focusedMenuItem(i int) bool {
	return m.activePane == PaneSidebar && m.listCursor == i
}

func (m Model) searchFocusResults() []ytmapi.SearchResult {
	var categories []string
	grouped := make(map[string][]ytmapi.SearchResult)
	for _, res := range m.searchResults {
		cat := res.Category
		if cat == "" {
			cat = titleCase(res.ResultType)
		}
		if len(grouped[cat]) == 0 {
			categories = append(categories, cat)
		}
		grouped[cat] = append(grouped[cat], res)
	}
	var out []ytmapi.SearchResult
	for _, cat := range categories {
		out = append(out, grouped[cat]...)
	}
	return out
}

func (m *Model) ensureListCursorInViewGrid(cursor, cols, rowHeight int) {
	if cols < 1 {
		cols = 1
	}
	row := cursor / cols
	cursorY := row * rowHeight

	if cursorY < m.mainViewport.YOffset {
		m.mainViewport.YOffset = cursorY
	} else if cursorY+rowHeight > m.mainViewport.YOffset+m.mainViewport.Height {
		m.mainViewport.YOffset = cursorY + rowHeight - m.mainViewport.Height
	}
}
