package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) generateSidebarContent(leftWidth int) string {
	var sb strings.Builder

	// Top Header
	logo := lipgloss.NewStyle().Foreground(colorRed).Render("▶ ") + lipgloss.NewStyle().Bold(true).Render("Music")
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Render("≡   " + logo))
	sb.WriteString("\n\n")

	// Menu Items
	for i, item := range m.menuItems {
		icon := "\ue0f5" // fa_home
		if item == "Explore" {
			icon = "\ue20f" // fa_compass
		}
		if item == "Library" {
			icon = "\ue39d" // md_library_music
		}
		if item == "Settings" {
			icon = "\uf013" // fa_gear
		}

		focused := m.focusedMenuItem(i)
		active := item == m.activeMenu

		switch {
		case focused:
			line := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("› ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Background(colorFocusBg).Render(icon+"  "+item)
			line = m.zone.Mark("menu_"+item, line)
			sb.WriteString(line)
			sb.WriteString("\n\n")
		case active:
			line := lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("┃ ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(icon+"  "+item)
			line = m.zone.Mark("menu_"+item, line)
			sb.WriteString(line)
			sb.WriteString("\n\n")
		default:
			line := lipgloss.NewStyle().Foreground(colorSubtext).Render("  " + icon + "  " + item)
			line = m.zone.Mark("menu_"+item, line)
			sb.WriteString(line)
			sb.WriteString("\n\n")
		}
	}

	// Divider
	if leftWidth > 8 {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).Render(strings.Repeat("─", leftWidth-8)))
	}
	sb.WriteString("\n\n")

	// New Playlist button
	newBtn := lipgloss.NewStyle().
		Background(colorHover).
		Foreground(colorText).
		Padding(0, 2).
		Render("+ New playlist")
	sb.WriteString("   ")
	sb.WriteString(newBtn)
	sb.WriteString("\n\n\n")

	// Playlists
	for _, pl := range m.playlists {
		title := lipgloss.NewStyle().Bold(true).Render(pl[0])
		sub := lipgloss.NewStyle().Foreground(colorSubtext).Render(pl[1])
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(title + "\n" + sub))
		sb.WriteString("\n\n")
	}
	
	// Ensure we push content down so there's some bottom padding
	sb.WriteString("\n\n")
	return sb.String()
}
