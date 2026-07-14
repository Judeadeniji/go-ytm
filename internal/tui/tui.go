package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
	zone "github.com/lrstanley/bubblezone"
)

type Model struct {
	width  int
	height int

	activePane     Pane
	activeCarousel int

	menuItems  []string
	activeMenu string
	playlists  [][2]string

	filters []string

	homeCarousels     []ytmapi.HomeCarousel
	carouselOffsets   map[string]int
	cachedArt         *KittyImage
	imageCache        map[string]*KittyImage
	mainViewport      viewport.Model
	leftViewport      viewport.Model
	searchInput       textinput.Model
	searchSuggestions []SearchSuggestion
	zone              *zone.Manager

	searchResults    []ytmapi.SearchResult
	searchFilter     string // api filter: "", songs, albums, artists, playlists
	lastSearchQuery  string
	ytmapiClient     *ytmapi.Client

	player    *player.Player
	extractor *search.Extractor
	statusMsg string

	// Playback state
	queue        Queue
	currentTrack *Track
	isPlaying    bool

	// Navigation / detail pages
	stack        ViewStack
	pageLoading  bool
	pageErr      string
	artistPage   *ytmapi.ArtistPage
	albumPage    *ytmapi.AlbumPage
	playlistPage *ytmapi.PlaylistPage
	trackCursor  int // focus index within playlist/album tracklist

	// imageDirty is true when thumbs arrived and a debounced redraw is pending.
	imageDirty bool
}

func NewModel(p *player.Player, ext *search.Extractor, apiClient *ytmapi.Client) Model {
	// Pre-render the image once at startup!
	artStr := RenderLocalImage(".build_assets/2026-07-14_05-43.png", artWidth, artHeight, hashString(".build_assets/2026-07-14_05-43.png"))

	// Initialize interactive search input
	ti := textinput.New()
	ti.Placeholder = "Search songs, albums, artists, podcasts"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSubtext)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorText)
	ti.CharLimit = 156
	ti.Width = 56 // Leave room for padding

	return Model{
		activePane:     PaneMain,
		activeCarousel: 0,
		menuItems:      []string{"Home", "Explore", "Library", "Upgrade"},
		activeMenu:     "Home",
		playlists: [][2]string{
			{"Liked Music", "📌 Auto playlist"},
			{"TikTok Songs", "Oluwaferanmi A.J"},
			{"Elite Raps..", "Misfit"},
			{"2022 Dump", "Oluwaferanmi A.J"},
			{"2025 Recap", "Made for Oluwaferanmi A.J"},
			{"This ain't Odumodu Blvck", "Oluwaferanmi A.J"},
			{"Violin Classics", "Oluwaferanmi A.J"},
		},
		filters:           []string{"All", "Songs", "Albums", "Artists", "Playlists"},
		searchFilter:      "",
		homeCarousels:     nil,
		searchSuggestions: []SearchSuggestion{},
		carouselOffsets:   make(map[string]int),
		cachedArt:         &artStr,
		imageCache:        make(map[string]*KittyImage),
		mainViewport:      viewport.New(0, 0),
		leftViewport:      viewport.New(0, 0),
		searchInput:       ti,
		zone:              zone.New(),
		searchResults:     nil,
		ytmapiClient:      apiClient,
		player:            p,
		extractor:         ext,
		statusMsg:         "Ready",
		queue:             Queue{current: -1},
	}
}

func (m Model) Init() tea.Cmd {
	return fetchHome(m.ytmapiClient)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		leftWidth := 24
		mainWidth := m.width - leftWidth
		if mainWidth < 0 {
			mainWidth = 0
		}

		m.leftViewport.Width = leftWidth
		m.leftViewport.Height = m.height - playerBarHeight
		m.leftViewport.SetContent(m.generateSidebarContent(leftWidth))

		m.mainViewport.Width = mainWidth - 2 // Account for padding
		m.mainViewport.Height = m.height - 4 - playerBarHeight // header (4) + bottom bar
		m.mainViewport.SetContent(m.generateMainContent(mainWidth))

	case tea.KeyMsg:
		// If the search bar is focused, hijack keyboard events
		if m.searchInput.Focused() {
			switch msg.String() {
			case "enter":
				query := m.searchInput.Value()
				m.lastSearchQuery = query
				m.statusMsg = "Searching for: " + query
				m.searchInput.Blur()
				return m, doSearchFiltered(m.ytmapiClient, query, m.searchFilter)
			case "esc":
				m.searchInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			oldVal := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(msg)
			newVal := m.searchInput.Value()

			if newVal != oldVal {
				return m, tea.Batch(cmd, fetchSuggestions(m.ytmapiClient, newVal))
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m = m.popNav()
			return m, nil
		case "tab":
			if m.activePane == PaneSidebar {
				m.activePane = PaneMain
			} else {
				m.activePane = PaneSidebar
			}
			return m, nil
		case "/":
			m.searchInput.Focus()
			return m, textinput.Blink
		case "p", " ":
			if m.currentTrack != nil {
				m.isPlaying = !m.isPlaying
				if m.onTracklistScreen() {
					m.setMainContent()
				}
				return m, togglePause(m.player)
			}
		case "s":
			m.statusMsg = "Stopped playback"
			m.isPlaying = false
			return m, stopPlayback(m.player)
		case "n":
			return m.playNext()
		case "b":
			return m.playPrev()
		case "enter":
			if m.onTracklistScreen() {
				return m.playFocusedTrack()
			}
		case "right":
			if m.currentTrack != nil {
				return m, seekCmd(m.player, 5)
			}
			// Scroll only the active carousel right
			if m.activeCarousel >= 0 && m.activeCarousel < len(m.homeCarousels) {
				activeTitle := m.homeCarousels[m.activeCarousel].Title
				maxLen := len(m.homeCarousels[m.activeCarousel].Contents)
				if m.carouselOffsets[activeTitle] < maxLen-1 {
					m.carouselOffsets[activeTitle]++
				}

				leftWidth := 24
				mainWidth := m.width - leftWidth
				if mainWidth < 0 {
					mainWidth = 0
				}
				oldOffset := m.mainViewport.YOffset
				m.mainViewport.SetContent(m.generateMainContent(mainWidth))
				m.mainViewport.YOffset = oldOffset
				return m, m.enqueueVisibleImages(mainWidth)
			}
			return m, nil

		case "left":
			if m.currentTrack != nil {
				return m, seekCmd(m.player, -5)
			}
			// Scroll only the active carousel left
			if m.activeCarousel >= 0 && m.activeCarousel < len(m.homeCarousels) {
				activeTitle := m.homeCarousels[m.activeCarousel].Title
				if m.carouselOffsets[activeTitle] > 0 {
					m.carouselOffsets[activeTitle]--
				}

				leftWidth := 24
				mainWidth := m.width - leftWidth
				if mainWidth < 0 {
					mainWidth = 0
				}
				oldOffset := m.mainViewport.YOffset
				m.mainViewport.SetContent(m.generateMainContent(mainWidth))
				m.mainViewport.YOffset = oldOffset
				return m, m.enqueueVisibleImages(mainWidth)
			}
			return m, nil

		case "up", "k":
			if m.onTracklistScreen() {
				m = m.moveTrackCursor(-1)
				return m, nil
			}
			if m.activePane == PaneMain {
				if m.activeCarousel > 0 {
					m.activeCarousel--
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateMainContent(mainWidth))
					m.mainViewport.YOffset = oldOffset
				}
			}
			// Let it fall through to viewport for scrolling
		case "down", "j":
			if m.onTracklistScreen() {
				m = m.moveTrackCursor(1)
				return m, nil
			}
			if m.activePane == PaneMain {
				if m.activeCarousel < len(m.homeCarousels)-1 {
					m.activeCarousel++
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateMainContent(mainWidth))
					m.mainViewport.YOffset = oldOffset
				}
			}
			// Let it fall through to viewport for scrolling
		}

		// Pass key events to active viewport for scrolling
		if m.activePane == PaneSidebar {
			m.leftViewport, cmd = m.leftViewport.Update(msg)
		} else {
			m.mainViewport, cmd = m.mainViewport.Update(msg)
		}
	case TrackStartedMsg:
		m.currentTrack = &msg.Track
		m.isPlaying = true
		m.statusMsg = "Playing: " + msg.Track.Title
		if m.onTracklistScreen() {
			m = m.syncTrackCursorToPlaying()
			m.ensureTrackCursorInView(10, 1)
			m.setMainContent()
		}
		return m, nil
	case StreamURLMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
			return m, nil
		}

		m.statusMsg = "Playing audio!"
		return m, loadAndPlay(m.player, msg.URL)
	case SearchResultsMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Search error: %v", msg.Err)
			m.pageLoading = false
			return m, nil
		}
		m.searchResults = msg.Results
		m.stack.Clear()
		m.artistPage = nil
		m.albumPage = nil
		m.playlistPage = nil
		m.pageLoading = false
		m.pageErr = ""
		m.statusMsg = fmt.Sprintf("Found %d results", len(msg.Results))
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		return m, m.enqueueVisibleImages(m.mainWidth())
	case ArtistMsg:
		m.pageLoading = false
		if msg.Err != nil {
			m.pageErr = fmtErr(msg.Err)
			m.statusMsg = "Artist unavailable"
			m.setMainContent()
			return m, nil
		}
		m.artistPage = msg.Page
		m.pageErr = ""
		id := msg.RequestID
		if id == "" {
			id = msg.Page.ChannelID
		}
		m.stack.Push(Screen{Kind: ScreenArtist, ID: id, Title: msg.Page.Name})
		m.statusMsg = msg.Page.Name
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		return m, m.enqueueVisibleImages(m.mainWidth())
	case AlbumMsg:
		m.pageLoading = false
		if msg.Err != nil {
			m.pageErr = fmtErr(msg.Err)
			m.statusMsg = "Album unavailable"
			m.setMainContent()
			return m, nil
		}
		m.albumPage = msg.Page
		m.pageErr = ""
		m.trackCursor = 0
		m.stack.Push(Screen{Kind: ScreenAlbum, Title: msg.Page.Title})
		m.statusMsg = msg.Page.Title
		m = m.syncTrackCursorToPlaying()
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		return m, m.enqueueVisibleImages(m.mainWidth())
	case PlaylistMsg:
		m.pageLoading = false
		if msg.Err != nil {
			m.pageErr = fmtErr(msg.Err)
			m.statusMsg = "Playlist unavailable"
			m.setMainContent()
			return m, nil
		}
		m.playlistPage = msg.Page
		m.pageErr = ""
		m.trackCursor = 0
		m.stack.Push(Screen{Kind: ScreenPlaylist, ID: msg.Page.ID, Title: msg.Page.Title})
		m.statusMsg = msg.Page.Title
		m = m.syncTrackCursorToPlaying()
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		return m, m.enqueueVisibleImages(m.mainWidth())
	case WatchMsg:
		if msg.Err != nil || msg.Watch == nil {
			return m, nil
		}
		// Seed upcoming tracks after the one we just started (skip first if same).
		for i, tr := range msg.Watch.Tracks {
			if tr.VideoID == "" {
				continue
			}
			if m.currentTrack != nil && tr.VideoID == m.currentTrack.VideoID && i == 0 {
				continue
			}
			m.queue.Add(trackFromAPI(tr))
		}
		return m, nil
	case HomeMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Home error: %v", msg.Err)
			return m, nil
		}
		m.homeCarousels = msg.Carousels

		leftWidth := 24
		mainWidth := m.width - leftWidth
		if mainWidth < 0 {
			mainWidth = 0
		}
		m.mainViewport.SetContent(m.generateMainContent(mainWidth))

		return m, m.enqueueVisibleImages(mainWidth)
	case ImageLoadedMsg:
		if msg.Kitty == nil {
			msg.Kitty = &KittyImage{Spacer: sizedPlaceholder(msg.Width, msg.Height)}
		}
		w, h := msg.Width, msg.Height
		if w <= 0 {
			w = artWidth
		}
		if h <= 0 {
			h = artHeight
		}
		m.imageCache[imageCacheKey(msg.URL, w, h)] = msg.Kitty
		// Debounce grid rebuild so a burst of finishes doesn't reshape N times.
		if !m.imageDirty {
			m.imageDirty = true
			return m, debounceImagesRedraw()
		}
		return m, nil
	case imagesRedrawMsg:
		m.imageDirty = false
		leftWidth := 24
		mainWidth := m.width - leftWidth
		if mainWidth < 0 {
			mainWidth = 0
		}
		oldOffset := m.mainViewport.YOffset
		m.mainViewport.SetContent(m.generateMainContent(mainWidth))
		m.mainViewport.YOffset = oldOffset
		// Kick off any newly-visible thumbs after layout settled.
		return m, m.enqueueVisibleImages(mainWidth)
	case SearchSuggestionsMsg:
		if msg.Err == nil {
			var sugs []SearchSuggestion
			for _, s := range msg.Suggestions {
				sugs = append(sugs, SearchSuggestion{
					Type:        SuggestionQuery,
					Text:        s.Text,
					Runs:        s.Runs,
					FromHistory: s.FromHistory,
				})
			}
			m.searchSuggestions = sugs
		}
		return m, nil
	}

	// Pass other events to viewport (e.g. mouse wheel/clicks)
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if m.searchInput.Focused() && mouseMsg.Type == tea.MouseLeft {
			for i, s := range m.searchSuggestions {
				if m.zone.Get(fmt.Sprintf("suggestion_%d", i)).InBounds(mouseMsg) {
					m.searchInput.SetValue(s.Text)
					m.lastSearchQuery = s.Text
					m.statusMsg = "Searching for: " + s.Text
					m.searchInput.Blur()
					return m, doSearchFiltered(m.ytmapiClient, s.Text, m.searchFilter)
				}
			}
		}

		if mouseMsg.Type == tea.MouseLeft {
			if m.zone.Get("player_play").InBounds(mouseMsg) {
				if m.currentTrack != nil {
					m.isPlaying = !m.isPlaying
					return m, togglePause(m.player)
				}
				return m, nil
			}
			if m.zone.Get("player_next").InBounds(mouseMsg) {
				return m.playNext()
			}
			if m.zone.Get("player_prev").InBounds(mouseMsg) {
				return m.playPrev()
			}

			// Sidebar menu items
			for _, item := range m.menuItems {
				if m.zone.Get("menu_"+item).InBounds(mouseMsg) {
					if item == "Home" {
						m = m.goHome()
						m.leftViewport.SetContent(m.generateSidebarContent(24))
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
					m.leftViewport.SetContent(m.generateSidebarContent(24))
					m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
					m.mainViewport.YOffset = 0
					return m, nil
				}
			}

			// Entity / filter / play zones
			if mm, zcmd, handled := m.handleZoneClick(mouseMsg); handled {
				return mm, zcmd
			}

			for i, carousel := range m.homeCarousels {
				title := carousel.Title
				if m.zone.Get(title+"_left").InBounds(mouseMsg) {
					m.activeCarousel = i
					m.activePane = PaneMain
					if m.carouselOffsets[title] > 0 {
						m.carouselOffsets[title]--
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
					m.mainViewport.YOffset = oldOffset
					return m, m.enqueueVisibleImages(m.mainWidth())
				}

				if m.zone.Get(title+"_right").InBounds(mouseMsg) {
					m.activeCarousel = i
					m.activePane = PaneMain
					maxLen := len(carousel.Contents)
					if m.carouselOffsets[title] < maxLen-1 {
						m.carouselOffsets[title]++
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
					m.mainViewport.YOffset = oldOffset
					return m, m.enqueueVisibleImages(m.mainWidth())
				}
			}
		}

		if mouseMsg.X < 24 { // leftWidth
			m.leftViewport, cmd = m.leftViewport.Update(msg)
		} else {
			m.mainViewport, cmd = m.mainViewport.Update(msg)
		}
	}

	return m, cmd
}

// playNext advances the queue and starts the next track.
func (m Model) playNext() (Model, tea.Cmd) {
	t, ok := m.queue.Next()
	if !ok {
		m.statusMsg = "End of queue"
		return m, nil
	}
	m.currentTrack = &t
	m.isPlaying = true
	m.statusMsg = "Loading: " + t.Title
	if m.onTracklistScreen() {
		m = m.syncTrackCursorToPlaying()
		m.ensureTrackCursorInView(10, 1)
		m.setMainContent()
	}
	return m, playTrack(m.player, m.extractor, t)
}

// playPrev moves back in the queue and starts that track.
func (m Model) playPrev() (Model, tea.Cmd) {
	t, ok := m.queue.Prev()
	if !ok {
		m.statusMsg = "Start of queue"
		return m, nil
	}
	m.currentTrack = &t
	m.isPlaying = true
	m.statusMsg = "Loading: " + t.Title
	if m.onTracklistScreen() {
		m = m.syncTrackCursorToPlaying()
		m.ensureTrackCursorInView(10, 1)
		m.setMainContent()
	}
	return m, playTrack(m.player, m.extractor, t)
}

// enqueueVisibleImages fetches thumbs for the active view (home cards, search,
// playlist/album cover + track thumbs), keyed by size so layout stays stable.
func (m Model) enqueueVisibleImages(mainWidth int) tea.Cmd {
	const maxSearchThumbs = 8

	var cmds []tea.Cmd
	seen := make(map[string]struct{})

	queue := func(url string, width, height int) {
		if url == "" {
			return
		}
		key := imageCacheKey(url, width, height)
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		if _, ok := m.imageCache[key]; ok {
			return
		}
		ph := KittyImage{Spacer: sizedPlaceholder(width, height)}
		m.imageCache[key] = &ph
		cmds = append(cmds, fetchImageSized(url, width, height))
	}

	if sc, ok := m.stack.Current(); ok {
		switch sc.Kind {
		case ScreenPlaylist:
			if m.playlistPage != nil {
				queue(firstThumbURL(m.playlistPage.Thumbnails), coverWidth, coverHeight)
			}
		case ScreenAlbum:
			if m.albumPage != nil {
				queue(firstThumbURL(m.albumPage.Thumbnails), coverWidth, coverHeight)
			}
		case ScreenArtist:
			if m.artistPage != nil {
				queue(firstThumbURL(m.artistPage.Thumbnails), coverWidth, coverHeight)
			}
		}
	} else if len(m.searchResults) > 0 {
		n := 0
		for _, res := range m.searchResults {
			if n >= maxSearchThumbs {
				break
			}
			if len(res.Thumbnails) == 0 {
				continue
			}
			if res.Category == "Top result" || n == 0 {
				queue(res.Thumbnails[0].URL, artWidth, artHeight)
				n++
			}
		}
	} else {
		contentWidth := mainWidth - 2
		cardWidth := 28
		maxVisible := contentWidth / cardWidth
		if maxVisible < 1 {
			maxVisible = 1
		}
		maxVisible++

		for _, carousel := range m.homeCarousels {
			offset := m.carouselOffsets[carousel.Title]
			if offset < 0 {
				offset = 0
			}
			end := offset + maxVisible
			if end > len(carousel.Contents) {
				end = len(carousel.Contents)
			}
			if offset > end {
				continue
			}
			for _, card := range carousel.Contents[offset:end] {
				if len(card.Thumbnails) > 0 {
					queue(card.Thumbnails[0].URL, artWidth, artHeight)
				}
			}
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
