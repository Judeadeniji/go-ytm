package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/player"
	zone "github.com/lrstanley/bubblezone"
)

const playerBarHeight = 4

type playProgressMsg struct {
	Pos      float64
	Duration float64
	Err      error
}

type playProgressTickMsg time.Time

func tickPlayProgress() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return playProgressTickMsg(t)
	})
}

func fetchPlayProgress(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return playProgressMsg{}
		}
		pos, err := p.PositionSeconds()
		if err != nil {
			return playProgressMsg{Err: err}
		}
		dur, err := p.DurationSeconds()
		if err != nil {
			return playProgressMsg{Pos: pos, Err: err}
		}
		return playProgressMsg{Pos: pos, Duration: dur}
	}
}

func newProgressBar(width int) progress.Model {
	if width < 10 {
		width = 10
	}
	m := progress.New(
		progress.WithSolidFill(string(colorAccent)),
		progress.WithoutPercentage(),
		progress.WithWidth(width),
	)
	m.EmptyColor = string(colorDivider)
	return m
}

// generatePlayerBar renders the bottom now-playing bar:
//
//	line 1: divider
//	line 2: controls + track
//	line 3: time + progress bar
//	line 4: key hints
func (m Model) generatePlayerBar(width int) string {
	bg := lipgloss.NewStyle().Background(colorBg)

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
		trackInfo = m.zone.Mark("player_nowplaying", trackInfo)
	} else {
		trackInfo = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Background(colorBg).
			Render("Nothing playing")
	}

	gap := bg.Render("   ")
	middleLeft := lipgloss.JoinHorizontal(lipgloss.Center, controls, gap, trackInfo)
	middlePad := width - lipgloss.Width(middleLeft)
	if middlePad < 0 {
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
		if m.currentTrack != nil {
			trunc = m.zone.Mark("player_nowplaying", trunc)
		}
		middleLeft = lipgloss.JoinHorizontal(lipgloss.Center, controls, gap, trunc)
		middlePad = width - lipgloss.Width(middleLeft)
		if middlePad < 0 {
			middlePad = 0
		}
	}
	middle := middleLeft + bg.Render(strings.Repeat(" ", middlePad))

	// Progress line — use scrub preview while dragging/clicking the bar.
	pos := m.playPos
	if m.scrubbing {
		pos = m.scrubPos
	}
	posLabel := formatClock(pos)
	durLabel := formatClock(m.playDuration)
	timeStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(colorBg)
	leftTime := timeStyle.Render(posLabel)
	rightTime := timeStyle.Render(durLabel)

	barWidth := width - lipgloss.Width(leftTime) - lipgloss.Width(rightTime) - 4
	if barWidth < 8 {
		barWidth = 8
	}

	pct := 0.0
	if m.playDuration > 0 {
		pct = pos / m.playDuration
		if pct < 0 {
			pct = 0
		}
		if pct > 1 {
			pct = 1
		}
	}

	pb := m.progress
	pb.Width = barWidth
	bar := m.zone.Mark("player_progress", pb.ViewAs(pct))
	progressLine := lipgloss.JoinHorizontal(lipgloss.Center,
		leftTime,
		" ",
		bar,
		" ",
		rightTime,
	)
	progPad := width - lipgloss.Width(progressLine)
	if progPad < 0 {
		progPad = 0
	}
	progressLine += bg.Render(strings.Repeat(" ", progPad))

	hints := lipgloss.NewStyle().
		Foreground(colorSubtext).
		Background(colorBg).
		Render("f stage  ·  ]/[ rail  ·  drag bar  ·  ,/. seek  ·  \\ hide rail")
	hintsPad := width - lipgloss.Width(hints)
	if hintsPad < 0 {
		hintsPad = 0
	}
	hintsLine := hints + bg.Render(strings.Repeat(" ", hintsPad))

	return lipgloss.JoinVertical(lipgloss.Left, divider, middle, progressLine, hintsLine)
}

// handleProgressScrub handles click and drag seeking on the progress bar.
// Press/drag only previews scrubPos in the UI; audio seeks on release.
func (m Model) handleProgressScrub(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if m.currentTrack == nil || m.playDuration <= 0 || !m.audioLoaded {
		if m.scrubbing && (msg.Action == tea.MouseActionRelease || msg.Type == tea.MouseRelease) {
			m.scrubbing = false
			return m, nil, true
		}
		return m, nil, false
	}

	z := m.zone.Get("player_progress")

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft,
		msg.Type == tea.MouseLeft && msg.Action == tea.MouseActionPress:
		if z.IsZero() || !z.InBounds(msg) {
			return m, nil, false
		}
		m.scrubbing = true
		m.scrubPos = m.seekPosFromProgressMouse(msg, z)
		return m, nil, true

	case m.scrubbing && (msg.Action == tea.MouseActionMotion || msg.Type == tea.MouseMotion):
		// Preview only — keep scrubbing even if the cursor leaves the bar.
		m.scrubPos = m.seekPosFromProgressMouse(msg, z)
		return m, nil, true

	case m.scrubbing && (msg.Action == tea.MouseActionRelease || msg.Type == tea.MouseRelease):
		m.scrubPos = m.seekPosFromProgressMouse(msg, z)
		m.playPos = m.scrubPos
		m.scrubbing = false
		m.markSessionDirty()
		return m, tea.Batch(seekAbsoluteCmd(m.player, m.scrubPos), fetchPlayProgress(m.player)), true
	}

	return m, nil, false
}

func (m Model) seekPosFromProgressMouse(msg tea.MouseMsg, z *zone.ZoneInfo) float64 {
	if z == nil || z.IsZero() || m.playDuration <= 0 {
		return m.playPos
	}
	w := z.EndX - z.StartX + 1
	if w <= 1 {
		return 0
	}
	x := msg.X - z.StartX
	if x < 0 {
		x = 0
	}
	if x > w-1 {
		x = w - 1
	}
	ratio := float64(x) / float64(w-1)
	pos := ratio * m.playDuration
	if pos < 0 {
		pos = 0
	}
	if pos > m.playDuration {
		pos = m.playDuration
	}
	return pos
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

func formatClock(seconds float64) string {
	if seconds < 0 || seconds != seconds { // NaN
		seconds = 0
	}
	total := int(seconds + 0.5)
	m := total / 60
	s := total % 60
	if m >= 60 {
		h := m / 60
		m = m % 60
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
