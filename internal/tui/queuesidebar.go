package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// generateQueuePanelContent draws the right rail: expanded now-playing + up next.
func (m Model) generateQueuePanelContent(width int) string {
	var sb strings.Builder
	inner := width - 2
	if inner < 8 {
		inner = 8
	}

	header := lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true).
		Render("Now playing")
	closeHint := lipgloss.NewStyle().Foreground(colorSubtext).Render(" \\")
	top := lipgloss.JoinHorizontal(lipgloss.Top, header, closeHint)
	sb.WriteString(lipgloss.NewStyle().Padding(1, 1, 0, 1).Render(top))
	sb.WriteString("\n")

	if m.currentTrack != nil {
		artURL := m.currentTrack.ThumbnailURL
		art := m.cachedArtAt(artURL, queueArtWidth, queueArtHeight)
		art = lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).Render(art)
		sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(art))
		sb.WriteString("\n")

		title := lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true).
			Width(inner).
			MaxWidth(inner).
			Render(m.currentTrack.Title)
		artist := lipgloss.NewStyle().
			Foreground(colorSubtext).
			Width(inner).
			MaxWidth(inner).
			Render(m.currentTrack.Artist)
		sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(title))
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(artist))
		sb.WriteString("\n")
	} else {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(1, 1).
			Render("Nothing playing"))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
		Render(strings.Repeat("─", max(4, inner))))
	sb.WriteString("\n")

	upcomingCount := 0
	if m.queue.CurrentIndex() >= 0 {
		upcomingCount = m.queue.Len() - m.queue.CurrentIndex() - 1
	} else {
		upcomingCount = m.queue.Len()
	}
	upLabel := "Up next"
	if upcomingCount > 0 {
		upLabel = fmt.Sprintf("Up next · %d", upcomingCount)
	}
	sb.WriteString(lipgloss.NewStyle().
		Padding(1, 1, 0, 1).
		Foreground(colorText).
		Bold(true).
		Render(upLabel))
	sb.WriteString("\n\n")

	tracks := m.queue.Tracks()
	cur := m.queue.CurrentIndex()
	if len(tracks) == 0 {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 1).
			Render("Queue empty"))
		return sb.String()
	}

	for i, tr := range tracks {
		playing := i == cur
		upcoming := cur >= 0 && i > cur
		past := cur >= 0 && i < cur
		focused := m.activePane == PaneQueue && m.queueCursor == i

		// Skip past tracks except keep a tight history of 1 previous.
		if past && i < cur-1 {
			continue
		}

		ind := "  "
		titleColor := colorText
		artistColor := colorSubtext
		bg := colorBg
		if playing {
			ind = "▶ "
			titleColor = colorAccent
		}
		if focused {
			bg = colorFocusBg
			if !playing {
				ind = "› "
			}
		} else if past {
			titleColor = colorSubtext
		}

		num := ""
		if upcoming || playing {
			if playing {
				num = ""
			} else {
				num = fmt.Sprintf("%d ", i-cur)
			}
		}

		lineBudget := inner - lipgloss.Width(ind) - lipgloss.Width(num) - 1
		if lineBudget < 6 {
			lineBudget = 6
		}
		title := lipgloss.NewStyle().
			Foreground(titleColor).
			Bold(playing || focused).
			Background(bg).
			MaxWidth(lineBudget).
			Render(tr.Title)
		row1 := ind + num + title

		artistBudget := inner - 2
		artist := lipgloss.NewStyle().
			Foreground(artistColor).
			Background(bg).
			MaxWidth(artistBudget).
			Render("  " + tr.Artist)

		block := lipgloss.JoinVertical(lipgloss.Left, row1, artist)
		block = lipgloss.NewStyle().
			Background(bg).
			Width(inner).
			MaxWidth(inner).
			Padding(0, 1).
			Render(block)

		block = m.zone.Mark(fmt.Sprintf("queue_track_%d", i), block)
		sb.WriteString(block)
		sb.WriteString("\n")
	}

	return sb.String()
}
