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

	// New Playlist button (display-only — no create endpoint yet)
	newBtn := lipgloss.NewStyle().Foreground(colorText).Render("  +  New playlist")
	sb.WriteString(newBtn)
	sb.WriteString("\n\n")

	// Playlists from library API
	for i, pl := range m.libPlaylists {
		title := mapStr(pl, "title")
		if title == "" {
			continue
		}
		if len(title) > leftWidth-8 && leftWidth > 8 {
			title = title[:leftWidth-9] + "…"
		}

		pid := mapStr(pl, "playlistId")
		icon := "" // Music note
		if pid == "LM" || strings.Contains(strings.ToLower(title), "liked") {
			icon = "" // Heart
		}

		focused := m.focusedSidebarPlaylist(i)
		var line string
		if focused {
			line = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("› ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Background(colorFocusBg).Render(icon+"  "+title)
		} else {
			line = lipgloss.NewStyle().Foreground(colorSubtext).Render("  " + icon + "  " + title)
		}
		if pid != "" {
			line = m.zone.Mark("sidebar_playlist_"+pid, line)
		}
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}

	sb.WriteString("\n\n")
	return sb.String()
}

// sidebarFocusCount is menu items + library playlists.
func (m Model) sidebarFocusCount() int {
	return len(m.menuItems) + len(m.libPlaylists)
}

// focusedSidebarPlaylist reports whether playlist i has keyboard focus.
func (m Model) focusedSidebarPlaylist(i int) bool {
	return m.activePane == PaneSidebar && m.listCursor == len(m.menuItems)+i
}

// ensureSidebarCursorInView keeps the focused sidebar row visible.
func (m *Model) ensureSidebarCursorInView() {
	viewH := m.leftViewport.Height
	if viewH <= 0 {
		return
	}
	// Approximate: header(~4) + menu items (2 each) + divider/header/new (~8) + playlists (2 each)
	cursorLine := 4
	if m.listCursor < len(m.menuItems) {
		cursorLine += m.listCursor * 2
	} else {
		cursorLine += len(m.menuItems)*2 + 8
		cursorLine += (m.listCursor - len(m.menuItems)) * 2
	}
	top := m.leftViewport.YOffset
	bottom := top + viewH - 1
	if cursorLine < top {
		m.leftViewport.SetYOffset(cursorLine)
	} else if cursorLine+1 > bottom {
		m.leftViewport.SetYOffset(cursorLine + 2 - viewH)
	}
}
