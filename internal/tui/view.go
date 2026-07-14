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

	leftWidth := 24
	mainWidth := m.width - leftWidth
	if mainWidth < 0 {
		mainWidth = 0
	}

	// ========================
	// 1. LEFT SIDEBAR
	// ========================
	leftSidebar := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(leftWidth).Height(m.height).MaxHeight(m.height).
		Render(m.leftViewport.View())

	// ========================
	// 2. HEADER (Search Bar)
	// ========================
	searchWidth := 60
	searchPadding := (mainWidth - searchWidth) / 2
	if searchPadding < 0 {
		searchPadding = 0
	}

	searchBox := lipgloss.NewStyle().
		Background(colorSearchBg).
		Foreground(colorText).
		Padding(0, 2).Width(searchWidth).
		Render(m.searchInput.View())

	profileIcon := lipgloss.NewStyle().Background(colorDivider).Foreground(colorText).Render(" AJ ")

	// Layout search centered and profile on right
	rightPadding := searchPadding - 26 // minus profile icon and status msg approx width
	if rightPadding < 0 {
		rightPadding = 0
	}

	statusStyle := lipgloss.NewStyle().Foreground(colorSubtext).Width(20).Align(lipgloss.Right)

	headerContent := fmt.Sprintf("%s%s%s%s   %s",
		strings.Repeat(" ", searchPadding),
		searchBox,
		strings.Repeat(" ", rightPadding),
		statusStyle.Render(m.statusMsg),
		profileIcon)

	header := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(mainWidth).Height(4).Padding(1, 0).
		Render(headerContent)

	// ========================
	// 3. MAIN CONTENT (Grids or Search Modal)
	// ========================
	var mainContent string
	if m.searchInput.Focused() {
		// Render search suggestions modal
		var sb strings.Builder
		for i, s := range m.searchSuggestions {
			var icon string
			if s.FromHistory || s.Type == SuggestionHistory {
				icon = "\ue292" // fa_history
			} else {
				icon = "\ue0e3" // fa_search
			}
			iconStyle := lipgloss.NewStyle().Foreground(colorSubtext).PaddingRight(2)

			if s.Type == SuggestionEntity {
				// Rich entity row
				img := lipgloss.NewStyle().Background(colorDivider).Foreground(colorText).Width(6).Height(3).Align(lipgloss.Center).Render("\nIMG")
				title := lipgloss.NewStyle().Foreground(colorText).Render(s.Text)
				sub := lipgloss.NewStyle().Foreground(colorSubtext).Render(s.Subtext)
				info := lipgloss.JoinVertical(lipgloss.Left, title, sub)
				row := lipgloss.JoinHorizontal(lipgloss.Top, img, "   ", info)
				row = m.zone.Mark(fmt.Sprintf("suggestion_%d", i), row)
				sb.WriteString(row)
				sb.WriteString("\n\n")
			} else {
				// Text row with runs rendering
				var textBuilder strings.Builder
				if len(s.Runs) > 0 {
					for _, run := range s.Runs {
						if run.Bold {
							textBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(run.Text))
						} else {
							textBuilder.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render(run.Text))
						}
					}
				} else {
					textBuilder.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render(s.Text))
				}
				
				row := lipgloss.JoinHorizontal(lipgloss.Top, iconStyle.Render(icon), textBuilder.String())
				row = m.zone.Mark(fmt.Sprintf("suggestion_%d", i), row)
				sb.WriteString(row)
				sb.WriteString("\n\n")
			}
		}

		modal := lipgloss.NewStyle().
			Background(colorSearchBg).
			Width(searchWidth+4). // Match search box width + padding
			Padding(1, 2).
			Render(strings.TrimSuffix(sb.String(), "\n\n"))

		mainContent = lipgloss.NewStyle().
			Background(colorBg).Foreground(colorText).
			Width(mainWidth).Height(m.height - 4).
			PaddingLeft(searchPadding).
			Render(modal)
	} else {
		mainContent = lipgloss.NewStyle().
			Background(colorBg).Foreground(colorText).
			Width(mainWidth).Height(m.height-4). // minus header
			Padding(0, 1).
			Render(m.mainViewport.View())
	}

	// Assemble Header and Main Content
	rightPane := lipgloss.JoinVertical(lipgloss.Left, header, mainContent)

	// Assemble All
	return m.zone.Scan(lipgloss.JoinHorizontal(lipgloss.Top, leftSidebar, rightPane))
}
