package tui

import (
	"fmt"
	"strconv"
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

	volControls := m.renderVolumeControls()

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
	volW := lipgloss.Width(volControls)
	middleLeft := lipgloss.JoinHorizontal(lipgloss.Center, controls, gap, trackInfo)
	middlePad := width - lipgloss.Width(middleLeft) - volW
	if middlePad < 1 {
		budget := width - lipgloss.Width(controls) - volW - 4
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
		middlePad = width - lipgloss.Width(middleLeft) - volW
		if middlePad < 1 {
			middlePad = 1
		}
	}
	middle := middleLeft + bg.Render(strings.Repeat(" ", middlePad)) + volControls

	// Progress line — use scrub preview while dragging/clicking the bar.
	pos := m.playPos
	if m.scrubbing {
		pos = m.scrubPos
	}
	dur := m.effectiveDuration()
	posLabel := formatClock(pos)
	durLabel := formatClock(dur)
	timeStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(colorBg)
	leftTime := timeStyle.Render(posLabel)
	rightTime := timeStyle.Render(durLabel)

	barWidth := width - lipgloss.Width(leftTime) - lipgloss.Width(rightTime) - 4
	if barWidth < 8 {
		barWidth = 8
	}

	pct := 0.0
	if dur > 0 {
		pct = pos / dur
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
		Render("f stage  ·  -/= vol  ·  m mute  ·  ]/[ rail  ·  \\ hide")
	hintsPad := width - lipgloss.Width(hints)
	if hintsPad < 0 {
		hintsPad = 0
	}
	hintsLine := hints + bg.Render(strings.Repeat(" ", hintsPad))

	return lipgloss.JoinVertical(lipgloss.Left, divider, middle, progressLine, hintsLine)
}

func (m Model) renderVolumeControls() string {
	btnStyle := lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorBg).
		Padding(0, 1)
	labelStyle := lipgloss.NewStyle().
		Foreground(colorSubtext).
		Background(colorBg)

	down := m.zone.Mark("player_vol_down", btnStyle.Render("−"))
	up := m.zone.Mark("player_vol_up", btnStyle.Render("+"))
	icon := "🔊"
	label := fmt.Sprintf("%d%%", int(m.volume+0.5))
	if m.muted || m.volume < 0.5 {
		icon = "🔇"
		if m.muted {
			label = "mute"
		}
	} else if m.volume < 35 {
		icon = "🔈"
	} else if m.volume < 70 {
		icon = "🔉"
	}
	muteBtn := m.zone.Mark("player_mute", btnStyle.Render(icon))
	pct := labelStyle.Render(label)
	return lipgloss.JoinHorizontal(lipgloss.Center, down, muteBtn, pct, up)
}

// adjustVolume changes volume by delta (percent points), unmutes on raise.
func (m Model) adjustVolume(delta float64) (Model, tea.Cmd) {
	v := m.volume + delta
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	m.volume = v
	if delta > 0 && m.muted {
		m.muted = false
	}
	m.markSessionDirty()
	cmds := []tea.Cmd{setVolumeCmd(m.player, m.volume)}
	if delta > 0 {
		cmds = append(cmds, setMuteCmd(m.player, false))
	}
	return m, tea.Batch(cmds...)
}

// toggleMute flips mute without changing the stored volume level.
func (m Model) toggleMute() (Model, tea.Cmd) {
	m.muted = !m.muted
	m.markSessionDirty()
	return m, setMuteCmd(m.player, m.muted)
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

// effectiveDuration prefers live mpv duration, then restored session, then track label.
func (m Model) effectiveDuration() float64 {
	if m.playDuration > 0 {
		return m.playDuration
	}
	if m.currentTrack != nil {
		if d := parseClock(m.currentTrack.Duration); d > 0 {
			return d
		}
	}
	return 0
}

// parseClock parses "M:SS" or "H:MM:SS" into seconds. Returns 0 on failure.
func parseClock(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0
		}
		nums[i] = n
	}
	switch len(nums) {
	case 2:
		return float64(nums[0]*60 + nums[1])
	case 3:
		return float64(nums[0]*3600 + nums[1]*60 + nums[2])
	default:
		return 0
	}
}
