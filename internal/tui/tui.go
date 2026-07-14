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

	menuItems []string
	activeMenu string
	playlists [][2]string

	filters []string

	homeCarousels []ytmapi.HomeCarousel

	cachedArt         string
	mainViewport      viewport.Model
	leftViewport      viewport.Model
	carouselOffsets   map[string]int
	searchInput       textinput.Model
	searchSuggestions []SearchSuggestion
	zone              *zone.Manager

	searchResults []ytmapi.SearchResult
	ytmapiClient  *ytmapi.Client

	player    *player.Player
	extractor *search.Extractor
	statusMsg string
}

func NewModel(p *player.Player, ext *search.Extractor, apiClient *ytmapi.Client) Model {
	// Pre-render the image once at startup!
	artStr := RenderLocalImage(".build_assets/2026-07-14_05-43.png", 24, 10)

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
		filters: []string{"Podcasts", "Energize", "Workout", "Relax", "Commute", "Feel good", "Sad", "Romance", "Party", "Sleep", "Focus"},
		homeCarousels: nil,
		searchSuggestions: []SearchSuggestion{
			{Type: SuggestionHistory, Text: "gnx"},
			{Type: SuggestionQuery, Text: "gnx kendrick lamar"},
			{Type: SuggestionQuery, Text: "gnx kendrick lamar full album"},
			{Type: SuggestionQuery, Text: "gnx album"},
			{Type: SuggestionEntity, Text: "GNX", Subtext: "🅴 Album • Kendrick Lamar • 2024", Image: artStr},
			{Type: SuggestionEntity, Text: "gnx (feat. Hitta J3, YoungThreat & Peysoh)", Subtext: "🅴 Song • Kendrick Lamar • 19M plays • GNX", Image: artStr},
		},
		cachedArt:         artStr,
		mainViewport:      viewport.New(0, 0),
		leftViewport:      viewport.New(0, 0),
		carouselOffsets:   make(map[string]int),
		searchInput:       ti,
		zone:              zone.New(),
		searchResults:     nil,
		ytmapiClient:      apiClient,
		player:            p,
		extractor:         ext,
		statusMsg:         "Ready",
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
		m.leftViewport.Height = m.height
		m.leftViewport.SetContent(m.generateSidebarContent(leftWidth))

		m.mainViewport.Width = mainWidth - 2 // Account for padding
		m.mainViewport.Height = m.height - 4 // Account for header
		m.mainViewport.SetContent(m.generateGridContent(mainWidth))

	case tea.KeyMsg:
		// If the search bar is focused, hijack keyboard events
		if m.searchInput.Focused() {
			switch msg.String() {
			case "enter":
				query := m.searchInput.Value()
				m.statusMsg = "Searching for: " + query
				m.searchInput.Blur()
				return m, doSearch(m.ytmapiClient, query)
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
			if len(m.searchResults) > 0 {
				m.searchResults = nil
				leftWidth := 24
				mainWidth := m.width - leftWidth
				if mainWidth < 0 {
					mainWidth = 0
				}
				m.mainViewport.SetContent(m.generateGridContent(mainWidth))
				return m, nil
			}
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
		case "p":
			m.statusMsg = "Loading audio..."
			// Proof of concept: Fetch "Not Like Us" by Kendrick Lamar
			return m, fetchStreamURL(m.extractor, "T6eK-2OQtew")
		case "s":
			m.statusMsg = "Stopped playback"
			return m, stopPlayback(m.player)
		case "right":
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
				m.mainViewport.SetContent(m.generateGridContent(mainWidth))
				m.mainViewport.YOffset = oldOffset
			}
			return m, nil

		case "left":
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
				m.mainViewport.SetContent(m.generateGridContent(mainWidth))
				m.mainViewport.YOffset = oldOffset
			}
			return m, nil

		case "up":
			if m.activePane == PaneMain {
				if m.activeCarousel > 0 {
					m.activeCarousel--
					
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateGridContent(mainWidth))
					m.mainViewport.YOffset = oldOffset
				}
			}
			// Let it fall through to viewport for scrolling
		case "down":
			if m.activePane == PaneMain {
				if m.activeCarousel < len(m.homeCarousels)-1 {
					m.activeCarousel++
					
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateGridContent(mainWidth))
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
			return m, nil
		}
		m.searchResults = msg.Results
		m.statusMsg = fmt.Sprintf("Found %d results", len(msg.Results))
		
		leftWidth := 24
		mainWidth := m.width - leftWidth
		if mainWidth < 0 {
			mainWidth = 0
		}
		m.mainViewport.SetContent(m.generateGridContent(mainWidth))
		m.mainViewport.YOffset = 0 // reset scroll
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
		m.mainViewport.SetContent(m.generateGridContent(mainWidth))
		return m, nil
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
					m.statusMsg = "Searching for: " + s.Text
					m.searchInput.Blur()
					return m, doSearch(m.ytmapiClient, s.Text)
				}
			}
		}

		if mouseMsg.Type == tea.MouseLeft {
			if !m.searchInput.Focused() && len(m.searchResults) > 0 {
				for _, res := range m.searchResults {
					if res.VideoID != "" {
						if m.zone.Get("search_result_video_"+res.VideoID).InBounds(mouseMsg) {
							m.statusMsg = "Fetching audio for: " + res.Title
							return m, fetchStreamURL(m.extractor, res.VideoID)
						}
					}
				}
			}

			// Sidebar menu items
			for _, item := range m.menuItems {
				if m.zone.Get("menu_"+item).InBounds(mouseMsg) {
					m.activeMenu = item
					m.searchResults = nil // clear search results so we can see the menu
					
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					m.leftViewport.SetContent(m.generateSidebarContent(leftWidth))
					m.mainViewport.SetContent(m.generateGridContent(mainWidth))
					m.mainViewport.YOffset = 0
					return m, nil
				}
			}

			for i, carousel := range m.homeCarousels {
				title := carousel.Title
				if m.zone.Get(title+"_left").InBounds(mouseMsg) {
					m.activeCarousel = i
					m.activePane = PaneMain
					if m.carouselOffsets[title] > 0 {
						m.carouselOffsets[title]--
					}
					
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateGridContent(mainWidth))
					m.mainViewport.YOffset = oldOffset
					return m, nil
				}
				
				if m.zone.Get(title+"_right").InBounds(mouseMsg) {
					m.activeCarousel = i
					m.activePane = PaneMain
					maxLen := len(carousel.Contents)

					if m.carouselOffsets[title] < maxLen-1 {
						m.carouselOffsets[title]++
					}
					
					leftWidth := 24
					mainWidth := m.width - leftWidth
					if mainWidth < 0 {
						mainWidth = 0
					}
					oldOffset := m.mainViewport.YOffset
					m.mainViewport.SetContent(m.generateGridContent(mainWidth))
					m.mainViewport.YOffset = oldOffset
					return m, nil
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
