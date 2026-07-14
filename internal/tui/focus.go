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
	return screenOther
}

func (m Model) onHomeScreen() bool {
	return m.currentScreen() == screenHome
}

// artistFocusItem is one keyboard-addressable row on an artist page.
type artistFocusItem struct {
	Kind string
	Item map[string]any
}

func (m Model) artistFocusItems() []artistFocusItem {
	if m.artistPage == nil {
		return nil
	}
	a := m.artistPage
	var out []artistFocusItem
	add := func(kind string, results []map[string]any) {
		for i, item := range results {
			if i >= 12 {
				break
			}
			if artistItemZone(kind, item) == "" {
				continue
			}
			out = append(out, artistFocusItem{Kind: kind, Item: item})
		}
	}
	if a.Songs != nil {
		add("song", a.Songs.Results)
	}
	if a.Albums != nil {
		add("album", a.Albums.Results)
	}
	if a.Singles != nil {
		add("album", a.Singles.Results)
	}
	if a.Videos != nil {
		add("video", a.Videos.Results)
	}
	if a.Related != nil {
		add("related", a.Related.Results)
	}
	return out
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
		n := len(m.menuItems)
		if n == 0 {
			return m, false
		}
		m.listCursor = clampIndex(m.listCursor+delta, n)
		m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
		return m, true

	case PaneQueue:
		if !m.showQueuePanel() || m.queue.IsEmpty() {
			return m, false
		}
		m.queueCursor = clampIndex(m.queueCursor+delta, m.queue.Len())
		m.setQueuePanelContent()
		m.ensureQueueCursorInView()
		return m, true
	}

	// PaneMain (and default)
	switch m.currentScreen() {
	case screenAlbum, screenPlaylist:
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
		m.setMainContent()
		m.ensureListCursorInView(6, 1)
		return m, true

	case screenHome:
		if len(m.homeCarousels) == 0 {
			return m, false
		}
		m.activeCarousel = clampIndex(m.activeCarousel+delta, len(m.homeCarousels))
		// Reset card focus into the new row.
		m.homeCardCursor = m.carouselOffsets[m.homeCarousels[m.activeCarousel].Title]
		m.setMainContent()
		return m, true
	}

	return m, false
}

// moveHomeCard shifts focus within the active home carousel (horizontal).
func (m Model) moveHomeCard(delta int) (Model, tea.Cmd) {
	if !m.onHomeScreen() || len(m.homeCarousels) == 0 {
		return m, nil
	}
	car := m.homeCarousels[m.activeCarousel]
	if len(car.Contents) == 0 {
		return m, nil
	}
	m.homeCardCursor = clampIndex(m.homeCardCursor+delta, len(car.Contents))
	m.ensureHomeCardVisible()
	m.setMainContent()
	return m, m.enqueueVisibleImages(m.mainWidth())
}

func (m *Model) ensureHomeCardVisible() {
	if len(m.homeCarousels) == 0 {
		return
	}
	car := m.homeCarousels[m.activeCarousel]
	title := car.Title
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
	// Approximate: header (~18 lines) + 2 lines per track from (current-1).
	viewH := m.rightViewport.Height
	if viewH <= 0 {
		return
	}
	cur := m.queue.CurrentIndex()
	start := 0
	if cur > 0 {
		start = cur - 1
	}
	rel := m.queueCursor - start
	if rel < 0 {
		rel = 0
	}
	headerLines := 18
	cursorLine := headerLines + rel*3
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
		if m.listCursor < 0 || m.listCursor >= len(m.menuItems) {
			return m, nil
		}
		item := m.menuItems[m.listCursor]
		if item == "Home" {
			m = m.goHome()
			m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
			return m, m.enqueueVisibleImages(m.mainWidth())
		}
		m.activeMenu = item
		m.stack.Clear()
		m.searchResults = nil
		m.artistPage = nil
		m.albumPage = nil
		m.playlistPage = nil
		m.pageLoading = false
		m.pageErr = ""
		m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
		m.setMainContent()
		m.mainViewport.YOffset = 0
		return m, nil

	case PaneQueue:
		if !m.showQueuePanel() {
			return m, nil
		}
		return m.playQueueIndex(m.queueCursor)
	}

	switch m.currentScreen() {
	case screenAlbum, screenPlaylist:
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

	case screenArtist:
		items := m.artistFocusItems()
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
	}

	return m, nil
}

func (m Model) moveSuggestionFocus(delta int) Model {
	if len(m.searchSuggestions) == 0 {
		return m
	}
	m.listCursor = clampIndex(m.listCursor+delta, len(m.searchSuggestions))
	return m
}

func (m Model) activateSuggestion() (Model, tea.Cmd) {
	if m.listCursor < 0 || m.listCursor >= len(m.searchSuggestions) {
		query := m.searchInput.Value()
		m.lastSearchQuery = query
		m.statusMsg = "Searching for: " + query
		m.searchInput.Blur()
		return m, doSearchFiltered(m.ytmapiClient, query, m.searchFilter)
	}
	s := m.searchSuggestions[m.listCursor]
	m.searchInput.SetValue(s.Text)
	m.lastSearchQuery = s.Text
	m.statusMsg = "Searching for: " + s.Text
	m.searchInput.Blur()
	return m, doSearchFiltered(m.ytmapiClient, s.Text, m.searchFilter)
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

// focusedHomeCard reports keyboard focus on a home card.
func (m Model) focusedHomeCard(carouselIndex, cardIndex int) bool {
	return m.activePane == PaneMain && m.onHomeScreen() &&
		m.activeCarousel == carouselIndex && m.homeCardCursor == cardIndex
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
