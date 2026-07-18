package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpBinding struct {
	Keys string
	Desc string
}

type helpSection struct {
	Title    string
	Bindings []helpBinding
}

func helpSections() []helpSection {
	return []helpSection{
		{
			Title: "General",
			Bindings: []helpBinding{
				{"?", "Toggle this shortcuts sheet"},
				{"q / ctrl+c", "Quit"},
				{"esc / backspace", "Back / close now playing"},
				{"/", "Focus search"},
				{"tab / shift+tab", "Cycle focus panes"},
				{"\\", "Show / hide queue rail"},
				{"[ / ]", "Cycle queue rail tabs"},
			},
		},
		{
			Title: "Navigation",
			Bindings: []helpBinding{
				{"↑ ↓ / k j", "Move focus up / down"},
				{"← → / h l", "Move focus / seek ±5s"},
				{"enter / space", "Activate focused item"},
				{"pgup / ctrl+u", "Page up"},
				{"pgdn / ctrl+d", "Page down"},
				{", / .", "Seek −5s / +5s (or artist carousel)"},
				{"< / >", "Artist carousel (or tempo −/+)"},
			},
		},
		{
			Title: "Playback",
			Bindings: []helpBinding{
				{"p / space", "Play / pause"},
				{"n / b", "Next / previous track"},
				{"s", "Stop playback"},
				{"f", "Toggle now playing stage"},
				{"a", "Go to playing album"},
				{"R", "Cycle repeat (off → all → one)"},
				{"S", "Toggle shuffle"},
			},
		},
		{
			Title: "Audio",
			Bindings: []helpBinding{
				{"- / =+", "Volume down / up"},
				{"m", "Mute / unmute"},
				{"o", "Toggle loudness normalize"},
				{"v", "Toggle silence skip"},
				{"x / X", "Toggle crossfade / cycle duration"},
				{"t", "Cycle sleep timer"},
				{"{ / }", "Pitch − / + semitone"},
				{"E", "Cycle EQ preset"},
			},
		},
		{
			Title: "Library & downloads",
			Bindings: []helpBinding{
				{"d", "Download / remove focused track"},
				{"D", "Download all tracks in view"},
			},
		},
		{
			Title: "Now playing",
			Bindings: []helpBinding{
				{"tab", "Cycle lyrics / related / queue"},
				{"c / r", "Resync lyrics follow"},
				{"enter", "Seek to focused lyric line"},
				{"← →", "Seek ±5s"},
			},
		},
		{
			Title: "Search",
			Bindings: []helpBinding{
				{"↑ ↓", "Move suggestion"},
				{"enter", "Open suggestion / search"},
				{"esc", "Leave search"},
			},
		},
		{
			Title: "Settings",
			Bindings: []helpBinding{
				{"1–5", "Switch settings tab"},
				{"↑ ↓ / k j", "Move row"},
				{"← → / h l", "Adjust value"},
				{"enter / space", "Activate row"},
			},
		},
	}
}

func (m Model) renderHelpSheet(maxW, maxH int) string {
	sheetW := maxW - 8
	if sheetW > 72 {
		sheetW = 72
	}
	if sheetW < 40 {
		sheetW = max(28, maxW-4)
	}
	innerW := sheetW - 4
	if innerW < 20 {
		innerW = 20
	}

	title := lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorSearchBg).
		Bold(true).
		Render("Keyboard shortcuts")
	hint := lipgloss.NewStyle().
		Foreground(colorSubtext).
		Background(colorSearchBg).
		Render("? or esc to close  ·  ↑↓ to scroll")

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		title,
		lipgloss.NewStyle().Background(colorSearchBg).Render(strings.Repeat(" ", max(1, innerW-lipgloss.Width(title)-lipgloss.Width(hint)))),
		hint,
	)
	if lipgloss.Width(header) > innerW {
		header = lipgloss.JoinVertical(lipgloss.Left, title, hint)
	}

	keyStyle := lipgloss.NewStyle().Foreground(colorAccent).Background(colorSearchBg).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSearchBg)
	secStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(colorSearchBg).Bold(true)
	blank := lipgloss.NewStyle().Background(colorSearchBg).Width(innerW).Render("")

	var lines []string
	lines = append(lines, header, blank)

	for _, sec := range helpSections() {
		lines = append(lines, secStyle.Width(innerW).Render(strings.ToUpper(sec.Title)))
		for _, b := range sec.Bindings {
			row := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(b.Keys),
				descStyle.MaxWidth(innerW-17).Render(b.Desc),
			)
			pad := innerW - lipgloss.Width(row)
			if pad > 0 {
				row += lipgloss.NewStyle().Background(colorSearchBg).Render(strings.Repeat(" ", pad))
			}
			lines = append(lines, row)
		}
		lines = append(lines, blank)
	}

	// Scroll window
	bodyH := maxH - 4
	if bodyH < 6 {
		bodyH = 6
	}
	total := len(lines)
	off := m.helpOffset
	if off < 0 {
		off = 0
	}
	maxOff := total - bodyH
	if maxOff < 0 {
		maxOff = 0
	}
	if off > maxOff {
		off = maxOff
	}
	end := off + bodyH
	if end > total {
		end = total
	}
	visible := lines[off:end]
	for len(visible) < bodyH {
		visible = append(visible, blank)
	}

	body := strings.Join(visible, "\n")
	scrollHint := ""
	if total > bodyH {
		scrollHint = fmt.Sprintf(" %d–%d / %d ", off+1, end, total)
	}
	footer := lipgloss.NewStyle().
		Foreground(colorSubtext).
		Background(colorSearchBg).
		Width(innerW).
		Align(lipgloss.Right).
		Render(scrollHint)

	content := lipgloss.JoinVertical(lipgloss.Left, body, footer)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDivider).
		Background(colorSearchBg).
		Padding(0, 1).
		Width(sheetW).
		Render(content)
}

func (m *Model) clampHelpOffset(viewH int) {
	total := 0
	for _, sec := range helpSections() {
		total += 1 + len(sec.Bindings) + 1 // title + bindings + blank
	}
	total += 2 // header + blank
	bodyH := viewH - 4
	if bodyH < 6 {
		bodyH = 6
	}
	maxOff := total - bodyH
	if maxOff < 0 {
		maxOff = 0
	}
	if m.helpOffset < 0 {
		m.helpOffset = 0
	}
	if m.helpOffset > maxOff {
		m.helpOffset = maxOff
	}
}

func (m Model) withHelpOverlay(base string) string {
	if !m.helpOpen {
		return base
	}
	sheetH := m.height - playerBarHeight - 2
	if sheetH < 10 {
		sheetH = m.height - 2
	}
	sheet := m.renderHelpSheet(m.width, sheetH)
	sw := lipgloss.Width(sheet)
	sh := lipgloss.Height(sheet)
	x := (m.width - sw) / 2
	if x < 0 {
		x = 0
	}
	y := (m.contentHeight() - sh) / 2
	if y < 0 {
		y = 0
	}
	return placeOverlay(base, sheet, x, y)
}
