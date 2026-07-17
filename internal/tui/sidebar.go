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
	sb.WriteString("\n")
	if m.userProfile != nil && m.userProfile.Name != "" {
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(colorSubtext).Render(m.userProfile.Name))
	}
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

	// Playlists Header
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(colorSubtext).Bold(true).Render("PLAYLISTS"))
	sb.WriteString("\n\n")

	// New Playlist button
	newBtn := lipgloss.NewStyle().Foreground(colorText).Render("  +  New playlist")
	sb.WriteString(newBtn)
	sb.WriteString("\n\n")

	// Playlists
	for _, pl := range m.playlists {
		// Single line compact rendering for TUI
		title := pl[0]
		// Shorten title if needed
		if len(title) > leftWidth-8 && leftWidth > 8 {
			title = title[:leftWidth-9] + "…"
		}
		
		icon := "" // Music note icon
		if strings.Contains(pl[1], "Auto playlist") || strings.Contains(pl[0], "Liked") {
			icon = "" // Heart icon
		}

		line := lipgloss.NewStyle().Foreground(colorSubtext).Render("  " + icon + "  " + title)
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}
	
	// Ensure we push content down so there's some bottom padding
	sb.WriteString("\n\n")
	return sb.String()
}
