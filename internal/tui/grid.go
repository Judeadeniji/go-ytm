package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) generateGridContent(mainWidth int) string {
	var mb strings.Builder

	// Filters
	var filters []string
	for _, f := range m.filters {
		filters = append(filters, lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 1).Render(f))
	}
	mb.WriteString("  ")
	mb.WriteString(strings.Join(filters, "   "))
	mb.WriteString("\n\n\n")

	// Helper to render horizontal grid row
	renderGrid := func(preTitle, title string, cards []AlbumCard) string {
		var row strings.Builder
		if preTitle != "" {
			row.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render(preTitle))
			row.WriteString("\n")
		}

		contentWidth := mainWidth - 2 // mainWidth minus left/right padding
		titleStr := lipgloss.NewStyle().Bold(true).Render(title)

		// Button Styles
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
		row.WriteString(titleStr)
		row.WriteString(strings.Repeat(" ", space))
		row.WriteString(rightControls)
		row.WriteString("\n\n")

		var blocks []string

		// Apply carousel scrolling offset
		offset := m.carouselOffsets[title]
		if offset < 0 {
			offset = 0
		}
		if offset > len(cards) {
			offset = len(cards)
		}
		visibleCards := cards[offset:]

		for _, card := range visibleCards {
			t := card.Title
			if len(t) > 20 {
				t = t[:17] + "..."
			}
			s := card.Subtitle
			if len(s) > 20 {
				s = s[:17] + "..."
			}

			// Use the pre-rendered cached ANSI image to prevent lag
			art := m.cachedArt

			titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(t)
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
