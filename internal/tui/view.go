package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.width < 62 {
		return "Terminal too small"
	}

	contentHeight := m.contentHeight()
	playerBar := m.generatePlayerBar(m.width)
	left, mainWidth, right := m.layoutWidths()

	// Now playing mode: replace left/header/center chrome, keep queue rail + player bar.
	if m.nowPlayingOpen {
		npBody := lipgloss.NewStyle().
			Width(m.width).Height(contentHeight).MaxHeight(contentHeight).
			Background(colorBg).
			Render(m.generateNowPlayingBody(m.width, contentHeight))
		return m.zone.Scan(lipgloss.JoinVertical(lipgloss.Left, npBody, playerBar))
	}

	// ========================
	// 1. LEFT SIDEBAR
	// ========================
	leftSidebar := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(left).Height(contentHeight).MaxHeight(contentHeight).
		Render(safeViewportView(&m.leftViewport))

	// ========================
	// 2. HEADER (Search Bar)
	// ========================
	searchWidth := 60
	if searchWidth > mainWidth-4 {
		searchWidth = max(20, mainWidth-4)
	}
	searchPadding := (mainWidth - searchWidth) / 2
	if searchPadding < 0 {
		searchPadding = 0
	}

	searchBoxStyle := lipgloss.NewStyle().
		Background(colorSearchBg).
		Foreground(colorText).
		Padding(0, 2).Width(searchWidth)
	if m.searchInput.Focused() {
		// Keep the textbox visually on top of the dropdown panel.
		searchBoxStyle = searchBoxStyle.
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorAccent).
			BorderBackground(colorBg)
	}
	searchBox := searchBoxStyle.Render(m.searchInput.View())

	profileIcon := lipgloss.NewStyle().Background(colorDivider).Foreground(colorText).Render(" AJ ")

	// Layout search centered and profile on right
	rightPadding := searchPadding - 26 // minus profile icon and status msg approx width
	if rightPadding < 0 {
		rightPadding = 0
	}

	statusStyle := lipgloss.NewStyle().Foreground(colorSubtext).Width(28).MaxWidth(28).Align(lipgloss.Right)

	headerContent := fmt.Sprintf("%s%s%s%s   %s",
		strings.Repeat(" ", searchPadding),
		searchBox,
		strings.Repeat(" ", rightPadding),
		statusStyle.Render(m.statusMsg),
		profileIcon)

	header := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(mainWidth).Height(headerHeight).Padding(1, 0).
		Render(headerContent)

	// ========================
	// 3. MAIN CONTENT (Grids or Search Modal)
	// ========================
	mainHeight := m.mainPaneHeight()

	var mainContent string
	if m.searchInput.Focused() {
		// Dropdown sits under the search box (same width) — never wider, so it
		// doesn't visually wrap/cover the textbox.
		searchModalWidth := searchWidth
		if searchModalWidth > mainWidth-2 {
			searchModalWidth = max(24, mainWidth-2)
		}
		modal := m.renderSuggestionsModal(searchModalWidth, mainHeight-1)
		// 1-row gap keeps the textbox clear of the modal panel.
		dropdown := lipgloss.JoinVertical(lipgloss.Left, "", modal)
		mainContent = lipgloss.NewStyle().
			Background(colorBg).Foreground(colorText).
			Width(mainWidth).Height(mainHeight).
			PaddingLeft(searchPadding).
			Render(dropdown)
	} else {
		mainContent = lipgloss.NewStyle().
			Background(colorBg).Foreground(colorText).
			Width(mainWidth).Height(mainHeight).
			Padding(0, 1).
			Render(safeViewportView(&m.mainViewport))
	}

	center := lipgloss.JoinVertical(lipgloss.Left, header, mainContent)
	parts := []string{leftSidebar, center}

	if right > 0 {
		border := lipgloss.NewStyle().
			Foreground(colorDivider).
			Background(colorBg).
			Height(contentHeight).
			Render("│")
		innerW := right - 1
		if innerW < 1 {
			innerW = 1
		}
		// Avoid Foreground/Background on the wrapper — they override ANSI halfblock
		// colors in the now-playing cover.
		queuePane := lipgloss.NewStyle().
			Width(innerW).Height(contentHeight).MaxHeight(contentHeight).
			Render(safeViewportView(&m.rightViewport))
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, border, queuePane))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return m.zone.Scan(lipgloss.JoinVertical(lipgloss.Left, body, playerBar))
}
