package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
)

type Model struct {
	width  int
	height int

	activePane Pane

	menuItems []string
	playlists [][2]string

	filters []string

	listenAgain        []AlbumCard
	albumsForYou       []AlbumCard
	forgottenFavorites []AlbumCard

	cachedArt         string
	mainViewport      viewport.Model
	leftViewport      viewport.Model
	carouselOffsets   map[string]int
	searchInput       textinput.Model
	searchSuggestions []SearchSuggestion

	player    *player.Player
	extractor *search.Extractor
	statusMsg string
}

func NewModel(p *player.Player, ext *search.Extractor) Model {
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
		activePane: PaneMain,
		menuItems:  []string{"Home", "Explore", "Library", "Upgrade"},
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
		listenAgain: []AlbumCard{
			{"HOLD SOMETHING", "Album • Islambo", colorCardArt[0], ""},
			{"Nep's Storybook", "Album • nep", colorCardArt[1], ""},
			{"TikTok Songs", "Oluwaferanmi A.J", colorCardArt[2], ""},
			{"Ca$ino", "Album • Baby Keem", colorCardArt[3], ""},
			{"Mr. Morale & The Big..", "Album • Kendrick Lamar", colorCardArt[4], ""},
			{"The Slim Shady LP", "Album • Eminem", colorCardArt[5], ""},
		},
		albumsForYou: []AlbumCard{
			{"Black Hippy 2", "Album • Black Hippy", colorCardArt[4], ""},
			{"Typical of Me EP", "EP • Laufey", colorCardArt[5], ""},
			{"Tha Carter IV", "Album • Lil Wayne", colorCardArt[0], ""},
			{"Legend Or No Legend", "Album • Wande Coal", colorCardArt[1], ""},
			{"PSYCHODRAMA", "Album • Dave", colorCardArt[2], ""},
			{"Young Preacher", "Album • Blaqbonez", colorCardArt[3], ""},
		},
		forgottenFavorites: []AlbumCard{
			{"The Off-Season", "Album • J. Cole", colorCardArt[3], ""},
			{"Friday Night Lights", "Album • J. Cole", colorCardArt[4], ""},
			{"GNX", "Album • Kendrick Lamar", colorCardArt[5], ""},
			{"999", "Album • Olamide", colorCardArt[0], ""},
			{"Lungu Boy", "Album • Asake", colorCardArt[1], ""},
			{"The Fall-Off", "Album • J. Cole", colorCardArt[2], ""},
		},
		searchSuggestions: []SearchSuggestion{
			{Type: SuggestionHistory, Text: "gnx"},
			{Type: SuggestionQuery, Text: "gnx kendrick lamar"},
			{Type: SuggestionQuery, Text: "gnx kendrick lamar full album"},
			{Type: SuggestionQuery, Text: "gnx album"},
			{Type: SuggestionEntity, Text: "GNX", Subtext: "🅴 Album • Kendrick Lamar • 2024", Image: artStr},
			{Type: SuggestionEntity, Text: "gnx (feat. Hitta J3, YoungThreat & Peysoh)", Subtext: "🅴 Song • Kendrick Lamar • 19M plays • GNX", Image: artStr},
		},
		cachedArt:       artStr,
		mainViewport:    viewport.New(0, 0),
		leftViewport:    viewport.New(0, 0),
		carouselOffsets: map[string]int{"Listen again": 0, "Albums for you": 0, "Forgotten favorites": 0},
		searchInput:     ti,
		player:          p,
		extractor:       ext,
		statusMsg:       "Ready",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
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
				m.statusMsg = "Searching for: " + m.searchInput.Value()
				m.searchInput.Blur()
				return m, nil
			case "esc":
				m.searchInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
			// Scroll carousels right
			if m.carouselOffsets["Listen again"] < len(m.listenAgain)-1 {
				m.carouselOffsets["Listen again"]++
			}
			if m.carouselOffsets["Albums for you"] < len(m.albumsForYou)-1 {
				m.carouselOffsets["Albums for you"]++
			}
			if m.carouselOffsets["Forgotten favorites"] < len(m.forgottenFavorites)-1 {
				m.carouselOffsets["Forgotten favorites"]++
			}

			// Re-render viewport
			leftWidth := 24
			mainWidth := m.width - leftWidth
			if mainWidth < 0 {
				mainWidth = 0
			}
			m.mainViewport.SetContent(m.generateGridContent(mainWidth))
			return m, nil

		case "left":
			// Scroll carousels left
			if m.carouselOffsets["Listen again"] > 0 {
				m.carouselOffsets["Listen again"]--
			}
			if m.carouselOffsets["Albums for you"] > 0 {
				m.carouselOffsets["Albums for you"]--
			}
			if m.carouselOffsets["Forgotten favorites"] > 0 {
				m.carouselOffsets["Forgotten favorites"]--
			}

			// Re-render viewport
			leftWidth := 24
			mainWidth := m.width - leftWidth
			if mainWidth < 0 {
				mainWidth = 0
			}
			m.mainViewport.SetContent(m.generateGridContent(mainWidth))
			return m, nil
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
	}

	// Pass other events to viewport (e.g. mouse wheel)
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if mouseMsg.X < 24 { // leftWidth
			m.leftViewport, cmd = m.leftViewport.Update(msg)
		} else {
			m.mainViewport, cmd = m.mainViewport.Update(msg)
		}
	}

	return m, cmd
}
