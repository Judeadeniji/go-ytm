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

	left, mainWidth, right := m.layoutWidths()
	contentHeight := m.contentHeight()

	// ========================
	// 1. LEFT SIDEBAR
	// ========================
	leftSidebar := lipgloss.NewStyle().
		Background(colorBg).Foreground(colorText).
		Width(left).Height(contentHeight).MaxHeight(contentHeight).
		Render(m.leftViewport.View())

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
			focused := m.listCursor == i
			bg := colorSearchBg
			if focused {
				bg = colorFocusBg
			}

			if s.Type == SuggestionEntity {
				// Rich entity row
				img := lipgloss.NewStyle().Background(colorDivider).Foreground(colorText).Width(6).Height(3).Align(lipgloss.Center).Render("\nIMG")
				title := lipgloss.NewStyle().Foreground(colorText).Background(bg).Render(s.Text)
				sub := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(s.Subtext)
				info := lipgloss.JoinVertical(lipgloss.Left, title, sub)
				row := lipgloss.JoinHorizontal(lipgloss.Top, img, "   ", info)
				row = lipgloss.NewStyle().Background(bg).Render(row)
				row = m.zone.Mark(fmt.Sprintf("suggestion_%d", i), row)
				sb.WriteString(row)
				sb.WriteString("\n\n")
			} else {
				// Text row with runs rendering
				var textBuilder strings.Builder
				if len(s.Runs) > 0 {
					for _, run := range s.Runs {
						if run.Bold {
							textBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(bg).Render(run.Text))
						} else {
							textBuilder.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(run.Text))
						}
					}
				} else {
					textBuilder.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(s.Text))
				}

				prefix := "  "
				if focused {
					prefix = "› "
				}
				row := lipgloss.JoinHorizontal(lipgloss.Top, iconStyle.Render(prefix+icon), textBuilder.String())
				row = lipgloss.NewStyle().Background(bg).Render(row)
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
			Width(mainWidth).Height(mainHeight).
			PaddingLeft(searchPadding).
			Render(modal)
	} else {
		mainContent = lipgloss.NewStyle().
			Background(colorBg).Foreground(colorText).
			Width(mainWidth).Height(mainHeight).
			Padding(0, 1).
			Render(m.mainViewport.View())
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
		queuePane := lipgloss.NewStyle().
			Background(colorBg).Foreground(colorText).
			Width(innerW).Height(contentHeight).MaxHeight(contentHeight).
			Render(m.rightViewport.View())
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, border, queuePane))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	playerBar := m.generatePlayerBar(m.width)

	return m.zone.Scan(lipgloss.JoinVertical(lipgloss.Left, body, playerBar))
}
