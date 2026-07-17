package tui

import "github.com/charmbracelet/lipgloss"

// YouTube Music strict dark theme colors
var (
	colorBg       = lipgloss.Color("#030303")
	colorSearchBg = lipgloss.Color("#212121")
	colorHover    = lipgloss.Color("#1A1A1A")
	colorText     = lipgloss.Color("#FFFFFF")
	colorSubtext  = lipgloss.Color("#AAAAAA")
	colorRed      = lipgloss.Color("#FF0000")
	colorDivider  = lipgloss.Color("#333333")
	colorBuffer   = lipgloss.Color("#5A5A5A") // buffered/loaded overlay under playhead
	colorAccent   = lipgloss.Color("#FF0000") // YouTube red — active / playing
	colorFocusBg  = lipgloss.Color("#282828")
)
