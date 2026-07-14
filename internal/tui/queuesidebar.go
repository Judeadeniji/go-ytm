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

// generateQueuePanelContent draws the right rail: now-playing card + full queue.
// Played tracks stay above a divider; current + upcoming sit below (Spotify/YTM style).
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

	tracks := m.queue.Tracks()
	cur := m.queue.CurrentIndex()
	if len(tracks) == 0 {
		sb.WriteString(lipgloss.NewStyle().
			Padding(1, 1, 0, 1).
			Foreground(colorText).
			Bold(true).
			Render("Queue"))
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 1).
			Render("Queue empty"))
		return sb.String()
	}

	sb.WriteString(lipgloss.NewStyle().
		Padding(1, 1, 0, 1).
		Foreground(colorText).
		Bold(true).
		Render(fmt.Sprintf("Queue · %d", len(tracks))))
	sb.WriteString("\n\n")

	// Played tracks (above the playing divider).
	hasPlayed := cur > 0
	if hasPlayed {
		for i := 0; i < cur; i++ {
			sb.WriteString(m.renderQueueListItem(i, tracks[i], inner, false))
			sb.WriteString("\n")
		}
		sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
			Render(strings.Repeat("─", max(4, inner))))
		sb.WriteString("\n")
		nextLabel := "Up next"
		upcoming := len(tracks) - cur
		if upcoming > 0 {
			nextLabel = fmt.Sprintf("Up next · %d", upcoming)
		}
		sb.WriteString(lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorSubtext).
			Bold(true).
			Render(nextLabel))
		sb.WriteString("\n\n")
	}

	// Current + upcoming (or full list when nothing is playing yet).
	start := cur
	if start < 0 {
		start = 0
	}
	for i := start; i < len(tracks); i++ {
		playing := cur >= 0 && i == cur
		sb.WriteString(m.renderQueueListItem(i, tracks[i], inner, playing))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderQueueListItem draws one unordered queue row (• / › / ▶).
func (m Model) renderQueueListItem(i int, tr Track, inner int, playing bool) string {
	focused := m.activePane == PaneQueue && m.queueCursor == i

	bullet := "• "
	titleColor := colorText
	artistColor := colorSubtext
	bg := colorBg
	if playing {
		bullet = "▶ "
		titleColor = colorAccent
	}
	if focused {
		bg = colorFocusBg
		bullet = "› "
		if !playing {
			titleColor = colorAccent
		}
	}
	// Played tracks above the divider read slightly muted.
	if !playing && m.queue.CurrentIndex() >= 0 && i < m.queue.CurrentIndex() {
		titleColor = colorSubtext
	}

	lineBudget := inner - lipgloss.Width(bullet) - 1
	if lineBudget < 6 {
		lineBudget = 6
	}
	title := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(focused || playing).
		Background(bg).
		MaxWidth(lineBudget).
		Render(tr.Title)
	row1 := bullet + title

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

	return m.zone.Mark(fmt.Sprintf("queue_track_%d", i), block)
}
