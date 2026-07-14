package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Define layout styling
var (
	topBarHeight    = 3
	bottomBarHeight = 3

	// Catppuccin Mocha inspired colors for a modern aesthetic
	colorBorder = lipgloss.Color("#45475A")
	colorText   = lipgloss.Color("#CDD6F4")
	colorAccent = lipgloss.Color("#CBA6F7")

	topBarStyle = lipgloss.NewStyle().
		Height(topBarHeight).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colorBorder).
		Padding(0, 2).
		Foreground(colorText)

	bottomBarStyle = lipgloss.NewStyle().
		Height(bottomBarHeight).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorBorder).
		Padding(0, 2).
		Foreground(colorText)

	leftSidebarStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(colorBorder).
		Padding(1, 2)

	rightSidebarStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorBorder).
		Padding(1, 2)

	centerContentStyle = lipgloss.NewStyle().
		Padding(1, 2)
)

// Model manages bubbletea models/views (now-playing, queue, search, library)
type Model struct {
	width  int
	height int
}

func NewModel() *Model {
	return &Model{}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Calculate dimensions
	contentHeight := m.height - topBarHeight - bottomBarHeight
	if contentHeight < 0 {
		contentHeight = 0
	}

	// Fixed widths for sidebars
	leftWidth := 30
	rightWidth := 30
	
	centerWidth := m.width - leftWidth - rightWidth
	if centerWidth < 0 {
		centerWidth = 0
	}

	// Render Top Bar
	topTabs := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("Playlists")
	topBarContent := "Home   " + topTabs + "   Albums   Artists            🔍 Search"
	topBar := topBarStyle.Width(m.width).Render(topBarContent)

	// Render Left Sidebar
	leftContent := lipgloss.NewStyle().Bold(true).Render("Recent") + "\n\n" +
		"• Feel Good Hits\n" +
		"• Chill Vibes 🔊\n" +
		"• Focus Mode\n" +
		"• Weekend Tunes\n" +
		"• Morning Boost\n" +
		"• Relax & Unwind"
	leftSidebar := leftSidebarStyle.Width(leftWidth).Height(contentHeight).Render(leftContent)

	// Render Center Content (Active Playlist)
	header := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("Chill Vibes")
	subtitle := "Playlist • 67 songs • 2h 31m\n"
	
	tableHeader := lipgloss.NewStyle().Foreground(colorBorder).Render("\n#   Title                               Album                Time\n")
	tableDivider := lipgloss.NewStyle().Foreground(colorBorder).Render("──────────────────────────────────────────────────────────────────\n")
	
	tracks := "▶   Cruel Summer - Taylor Swift         Lover                2:58\n" +
		"2   Good Morning - Kanye West           Graduation           3:15\n" +
		"3   Burning Up - Madonna                Madonna              3:45\n" +
		"4   Dirty Computer - Janelle Monáe      Dirty Computer       1:59\n" +
		"5   La Vie en Rose - Grace Jones        Portfolio            7:28\n" +
		"6   bad guy - Billie Eilish             When We All Fall...  3:14\n" +
		"7   Hurricane - Kanye West              Donda                3:58"
		
	centerContentText := header + "\n" + subtitle + tableHeader + tableDivider + tracks
	centerContent := centerContentStyle.Width(centerWidth).Height(contentHeight).Render(centerContentText)

	// Render Right Sidebar
	rightContent := lipgloss.NewStyle().Bold(true).Render("Top Artists") + "\n\n" +
		"• Taylor Swift\n" +
		"  221M listeners\n\n" +
		"• The Weeknd\n" +
		"  190M listeners\n\n" +
		"• Lana Del Rey\n" +
		"  142M listeners\n\n" +
		"• Bruno Mars\n" +
		"  119M listeners"
	rightSidebar := rightSidebarStyle.Width(rightWidth).Height(contentHeight).Render(rightContent)

	// Join the three columns horizontally
	middleRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftSidebar,
		centerContent,
		rightSidebar,
	)

	// Render Bottom Bar
	bottomBarContent := "▶  Cruel Summer - Taylor Swift           [00:57 ━━━━━━━╍╍╍╍╍╍╍╍╍╍╍ 02:58]"
	bottomBar := bottomBarStyle.Width(m.width).Render(bottomBarContent)

	// Join top, middle, and bottom vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		middleRow,
		bottomBar,
	)
}
