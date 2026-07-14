package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// centerBlock left-pads each line so the block is centered in width cells.
// Unlike lipgloss Width/Align, this leaves ANSI sequences intact.
func centerBlock(s string, width int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" || width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for i, line := range lines {
		pad := (width - lipgloss.Width(line)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// padBlock adds left horizontal space without restyling ANSI content.
func padBlock(s string, left int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" || left <= 0 {
		return s
	}
	pad := strings.Repeat(" ", left)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

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
		aw, ah := m.queueArtDims()
		if aw > inner {
			aw = inner
		}
		art := m.cachedArtAt(artURL, aw, ah)
		// Don't lipgloss-Width wrap halfblock ANSI — it strips colors / clips art.
		sb.WriteString(padBlock(centerBlock(art, inner), 1))
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

	// Only upcoming tracks — now-playing is already shown above.
	start := cur + 1
	if cur < 0 {
		start = 0
	}
	if start >= len(tracks) {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 1).
			Render("Nothing up next"))
		return sb.String()
	}

	for n, i := 1, start; i < len(tracks); n, i = n+1, i+1 {
		tr := tracks[i]
		focused := m.activePane == PaneQueue && m.queueCursor == i

		ind := "  "
		titleColor := colorText
		artistColor := colorSubtext
		bg := colorBg
		if focused {
			bg = colorFocusBg
			ind = "› "
			titleColor = colorAccent
		}

		num := fmt.Sprintf("%d ", n)
		lineBudget := inner - lipgloss.Width(ind) - lipgloss.Width(num) - 1
		if lineBudget < 6 {
			lineBudget = 6
		}
		title := lipgloss.NewStyle().
			Foreground(titleColor).
			Bold(focused).
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
