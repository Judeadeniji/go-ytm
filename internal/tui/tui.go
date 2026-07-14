package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
)

// YouTube Music strict dark theme colors
var (
	colorBg       = lipgloss.Color("#030303")
	colorSearchBg = lipgloss.Color("#212121")
	colorHover    = lipgloss.Color("#1A1A1A")
	colorText     = lipgloss.Color("#FFFFFF")
	colorSubtext  = lipgloss.Color("#AAAAAA")
	colorRed      = lipgloss.Color("#FF0000")
	colorDivider  = lipgloss.Color("#333333")
	colorCardArt  = []lipgloss.Color{
		lipgloss.Color("#3E2723"), lipgloss.Color("#1B5E20"), lipgloss.Color("#B71C1C"),
		lipgloss.Color("#4E342E"), lipgloss.Color("#263238"), lipgloss.Color("#827717"),
	}

	baseStyle = lipgloss.NewStyle().Background(colorBg).Foreground(colorText)
)

type Pane int

const (
	PaneSidebar Pane = iota
	PaneSearch
	PaneMain
)

type AlbumCard struct {
	Title    string
	Subtitle string
	ArtColor lipgloss.Color
	VideoID  string
}

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
	
	cachedArt string
	
	mainViewport    viewport.Model
	carouselOffsets map[string]int
	searchInput     textinput.Model
	
	player    *player.Player
	extractor *search.Extractor
	statusMsg string
}

type StreamURLMsg struct {
	URL string
	Err error
}

func fetchStreamURL(ext *search.Extractor, videoID string) tea.Cmd {
	return func() tea.Msg {
		url, err := ext.GetStreamURL(context.Background(), videoID)
		return StreamURLMsg{URL: url, Err: err}
	}
}

func stopPlayback(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		p.Stop()
		return nil
	}
}

func loadAndPlay(p *player.Player, url string) tea.Cmd {
	return func() tea.Msg {
		p.Load(url)
		p.Play()
		return nil
	}
}

func NewModel(p *player.Player, ext *search.Extractor) Model {
	// Pre-render the image once at startup! 
	// Decoding and resizing PNGs on every frame in View() causes massive lag.
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
		cachedArt:       artStr,
		mainViewport:    viewport.New(0, 0),
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
		
		leftWidth := 28
		mainWidth := m.width - leftWidth
		if mainWidth < 0 { mainWidth = 0 }
		
		m.mainViewport.Width = mainWidth - 8 // Account for padding
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
			if m.carouselOffsets["Listen again"] < len(m.listenAgain)-1 { m.carouselOffsets["Listen again"]++ }
			if m.carouselOffsets["Albums for you"] < len(m.albumsForYou)-1 { m.carouselOffsets["Albums for you"]++ }
			if m.carouselOffsets["Forgotten favorites"] < len(m.forgottenFavorites)-1 { m.carouselOffsets["Forgotten favorites"]++ }
			
			// Re-render viewport
			leftWidth := 28
			mainWidth := m.width - leftWidth
			if mainWidth < 0 { mainWidth = 0 }
			m.mainViewport.SetContent(m.generateGridContent(mainWidth))
			return m, nil
			
		case "left":
			// Scroll carousels left
			if m.carouselOffsets["Listen again"] > 0 { m.carouselOffsets["Listen again"]-- }
			if m.carouselOffsets["Albums for you"] > 0 { m.carouselOffsets["Albums for you"]-- }
			if m.carouselOffsets["Forgotten favorites"] > 0 { m.carouselOffsets["Forgotten favorites"]-- }
			
			// Re-render viewport
			leftWidth := 28
			mainWidth := m.width - leftWidth
			if mainWidth < 0 { mainWidth = 0 }
			m.mainViewport.SetContent(m.generateGridContent(mainWidth))
			return m, nil
		}
		
		// Pass key events to viewport for scrolling
		m.mainViewport, cmd = m.mainViewport.Update(msg)
	case StreamURLMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
			return m, nil
		}
		
		m.statusMsg = "Playing audio!"
		return m, loadAndPlay(m.player, msg.URL)
	}
	
	// Pass other events to viewport (e.g. mouse wheel)
	if _, ok := msg.(tea.MouseMsg); ok {
		m.mainViewport, cmd = m.mainViewport.Update(msg)
	}
	
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	leftWidth := 28
	mainWidth := m.width - leftWidth
	if mainWidth < 0 {
		mainWidth = 0
	}

	// ========================
	// 1. LEFT SIDEBAR
	// ========================
	var sb strings.Builder
	
	// Top Header
	logo := lipgloss.NewStyle().Foreground(colorRed).Render("▶ ") + lipgloss.NewStyle().Bold(true).Render("Music")
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Render("≡   " + logo) + "\n\n")

	// Menu Items
	for i, item := range m.menuItems {
		icon := "🏠"
		if item == "Explore" { icon = "🧭" }
		if item == "Library" { icon = "📚" }
		if item == "Upgrade" { icon = "▶️" }

		// Terminal-native active state with a left red bar
		if i == 0 {
			line := lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("┃ ") + 
			        lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(icon + "  " + item)
			sb.WriteString(line + "\n\n")
		} else {
			line := lipgloss.NewStyle().Foreground(colorSubtext).Render("  " + icon + "  " + item)
			sb.WriteString(line + "\n\n")
		}
	}

	// Divider
	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).Render(strings.Repeat("─", leftWidth-8)) + "\n\n")

	// New Playlist button (Using background instead of border to prevent lipgloss box-drawing bugs)
	newBtn := lipgloss.NewStyle().
		Background(colorHover).
		Foreground(colorText).
		Padding(0, 2).
		Render("+ New playlist")
	sb.WriteString("   " + newBtn + "\n\n\n")

	// Playlists
	for _, pl := range m.playlists {
		title := lipgloss.NewStyle().Bold(true).Render(pl[0])
		sub := lipgloss.NewStyle().Foreground(colorSubtext).Render(pl[1])
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(title+"\n"+sub) + "\n\n")
	}

	leftSidebar := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(leftWidth).Height(m.height).MaxHeight(m.height). // Enforce height clipping
		Render(sb.String())


	// ========================
	// 2. HEADER (Search Bar)
	// ========================
	searchWidth := 60
	searchPadding := (mainWidth - searchWidth) / 2
	if searchPadding < 0 { searchPadding = 0 }

	searchBox := lipgloss.NewStyle().
		Background(colorSearchBg).
		Foreground(colorText).
		Padding(0, 2).Width(searchWidth).
		Render(m.searchInput.View())

	profileIcon := lipgloss.NewStyle().Background(colorDivider).Foreground(colorText).Render(" AJ ")

	
	header := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(mainWidth).Height(4).Padding(1, 0).
		Render(headerContent)


	// ========================
	// 3. MAIN CONTENT (Grids)
	// ========================
	mainContent := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(mainWidth).Height(m.height - 4). // minus header
		Padding(0, 4).
		Render(m.mainViewport.View())


	// Assemble Header and Main Content
	rightPane := lipgloss.JoinVertical(lipgloss.Left, header, mainContent)

	// Assemble All
	return lipgloss.JoinHorizontal(lipgloss.Top, leftSidebar, rightPane)
}

func (m Model) generateGridContent(mainWidth int) string {
	var mb strings.Builder

	// Filters
	var filters []string
	for _, f := range m.filters {
		filters = append(filters, lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 1).Render(f))
	}
	mb.WriteString("  " + strings.Join(filters, "   ") + "\n\n\n")

	// Helper to render horizontal grid row
	renderGrid := func(preTitle, title string, cards []AlbumCard) string {
		var row strings.Builder
		if preTitle != "" {
			row.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render(preTitle) + "\n")
		}
		
		contentWidth := mainWidth - 8 // mainWidth minus left/right padding
		titleStr := lipgloss.NewStyle().Bold(true).Render(title)
		
		// Button Styles (Using background instead of border to prevent lipgloss box-drawing bugs)
		btnStyle := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2)
		leftBtn := btnStyle.Render("<")
		rightBtn := btnStyle.Render(">")
		arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)
		
		// "More" pill for Listen again
		var rightControls string
		if title == "Listen again" {
			morePill := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2).Render("More")
			rightControls = lipgloss.JoinHorizontal(lipgloss.Top, morePill, "   ", arrows)
		} else {
			rightControls = arrows
		}
		
		space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(rightControls)
		if space < 1 {
			space = 1
		}
		row.WriteString(titleStr + strings.Repeat(" ", space) + rightControls + "\n\n")
		
		var blocks []string
		
		// Apply carousel scrolling offset
		offset := m.carouselOffsets[title]
		if offset < 0 { offset = 0 }
		if offset > len(cards) { offset = len(cards) }
		visibleCards := cards[offset:]
		
		for _, card := range visibleCards {
			// Terminal-native Card Design
			t := card.Title
			if len(t) > 20 { t = t[:17] + "..." }
			s := card.Subtitle
			if len(s) > 20 { s = s[:17] + "..." }

			// Use the pre-rendered cached ANSI image to prevent lag
			art := m.cachedArt

			// Title
			titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(t)
			
			// Subtitle
			subStyle := lipgloss.NewStyle().Foreground(colorSubtext).Render(s)
			
			content := lipgloss.JoinVertical(lipgloss.Left, art, "", titleStyle, subStyle)
			
			block := lipgloss.NewStyle().
				Padding(0, 2).
				Width(28).
				Render(content)
				
			blocks = append(blocks, block)
		}
		
		row.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
		return row.String() + "\n\n\n"
	}

	// Section: Listen again
	mb.WriteString(renderGrid("OLUWAFERANMI A.J", "Listen again", m.listenAgain))

	// Section: Albums for you
	mb.WriteString(renderGrid("", "Albums for you", m.albumsForYou))

	// Section: Forgotten favorites
	mb.WriteString(renderGrid("", "Forgotten favorites", m.forgottenFavorites))

	return mb.String()
}
