package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const playerBarHeight = 3

// generatePlayerBar renders the 3-line now-playing bar at the bottom of the UI.
//
//	line 1: thin divider
//	line 2: ⏮  ⏸/▶  ⏭   title — artist
//	line 3: key hints
func (m Model) generatePlayerBar(width int) string {
	divider := lipgloss.NewStyle().
		Foreground(colorDivider).
		Background(colorBg).
		Width(width).
		Render(strings.Repeat("─", width))

	btnStyle := lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorBg).
		Padding(0, 1)

	prevBtn := m.zone.Mark("player_prev", btnStyle.Render("⏮"))
	playIcon := "▶"
	if m.isPlaying {
		playIcon = "⏸"
	}
	playBtn := m.zone.Mark("player_play", btnStyle.Render(playIcon))
	nextBtn := m.zone.Mark("player_next", btnStyle.Render("⏭"))

	controls := lipgloss.JoinHorizontal(lipgloss.Center, prevBtn, playBtn, nextBtn)

	var trackInfo string
	if m.currentTrack != nil {
		title := m.currentTrack.Title
		artist := m.currentTrack.Artist
		infoStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorBg).Bold(true)
		artistStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(colorBg)
		if artist != "" {
			trackInfo = infoStyle.Render(title) + artistStyle.Render("  —  "+artist)
		} else {
			trackInfo = infoStyle.Render(title)
		}
	} else {
		trackInfo = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Background(colorBg).
			Render("Nothing playing")
	}

	gap := lipgloss.NewStyle().Background(colorBg).Render("   ")
	middleLeft := lipgloss.JoinHorizontal(lipgloss.Center, controls, gap, trackInfo)

	hints := lipgloss.NewStyle().
		Foreground(colorSubtext).
		Background(colorBg).
		Render("space/p pause  ·  ←/→ seek ±5s  ·  n/b next/prev")

	// Pad middle and hints to full width
	middlePad := width - lipgloss.Width(middleLeft)
	if middlePad < 0 {
		middlePad = 0
		// Truncate by rebuilding with a width budget
		budget := width - lipgloss.Width(controls) - 3
		if budget < 8 {
			budget = 8
		}
		trunc := lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBg).
			Bold(true).
			MaxWidth(budget).
			Render(trackLabel(m.currentTrack))
		middleLeft = lipgloss.JoinHorizontal(lipgloss.Center, controls, gap, trunc)
		middlePad = width - lipgloss.Width(middleLeft)
		if middlePad < 0 {
			middlePad = 0
		}
	}
	middle := middleLeft + lipgloss.NewStyle().Background(colorBg).Render(strings.Repeat(" ", middlePad))

	hintsPad := width - lipgloss.Width(hints)
	if hintsPad < 0 {
		hintsPad = 0
	}
	hintsLine := hints + lipgloss.NewStyle().Background(colorBg).Render(strings.Repeat(" ", hintsPad))

	return lipgloss.JoinVertical(lipgloss.Left, divider, middle, hintsLine)
}

func trackLabel(t *Track) string {
	if t == nil {
		return "Nothing playing"
	}
	if t.Artist != "" {
		return fmt.Sprintf("%s — %s", t.Title, t.Artist)
	}
	return t.Title
}
