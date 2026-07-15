package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/player"
	zone "github.com/lrstanley/bubblezone"
)

const playerBarHeight = 4

type playProgressMsg struct {
	Pos      float64
	Duration float64
	Buffered float64 // demuxer cache end (seconds); 0 if unknown
	Err      error
}

type playProgressTickMsg time.Time

func tickPlayProgress() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return playProgressTickMsg(t)
	})
}

// tickAudioLoading refreshes the indeterminate buffer pulse while audio loads.
func tickAudioLoading() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg {
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
		buf, _ := p.BufferedSeconds()
		if buf < pos {
			buf = pos
		}
		if dur > 0 && buf > dur {
			buf = dur
		}
		return playProgressMsg{Pos: pos, Duration: dur, Buffered: buf}
	}
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
	switch {
	case m.audioLoading():
		playIcon = loadingSpinnerFrame()
	case m.isPlaying:
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
		if m.audioLoading() {
			trackInfo += artistStyle.Render("  ·  loading")
		} else if m.audioPending() {
			trackInfo += artistStyle.Render("  ·  ready")
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

	var bar string
	playPct, bufPct := m.progressPercents(pos, dur)
	switch {
	case m.audioLoading():
		// Buffer layer pulses while resolving; playhead sits at resume target.
		bufPct = loadingBufferPct()
		if playPct > bufPct {
			bufPct = playPct
		}
		bar = m.zone.Mark("player_progress", renderLayeredProgress(barWidth, playPct, bufPct, colorSubtext, colorBuffer))
	case m.audioPending():
		// Restored seek target as muted playhead; no buffer yet.
		bar = m.zone.Mark("player_progress", renderLayeredProgress(barWidth, playPct, playPct, colorSubtext, colorBuffer))
	default:
		bar = m.zone.Mark("player_progress", renderLayeredProgress(barWidth, playPct, bufPct, colorAccent, colorBuffer))
	}
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
		Render("f stage  ·  a View Album  ·  -/= vol  ·  m mute  ·  ]/[ rail")
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

// pluralCount formats "1 track" / "12 tracks" (or any singular/plural pair).
func pluralCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// audioPending is true when a track is selected but mpv has not loaded it yet
// (session restore / between extracts).
func (m Model) audioPending() bool {
	return m.currentTrack != nil && !m.audioLoaded
}

// audioLoading is true while we are actively resolving/loading audio.
func (m Model) audioLoading() bool {
	return m.audioPending() && m.isPlaying
}

func loadingSpinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := int(time.Now().UnixMilli()/90) % len(frames)
	return frames[i]
}

func (m Model) progressPercents(pos, dur float64) (playPct, bufPct float64) {
	if dur > 0 {
		playPct = pos / dur
		bufPct = m.playBuffered / dur
	}
	playPct = clamp01(playPct)
	bufPct = clamp01(bufPct)
	if bufPct < playPct {
		bufPct = playPct
	}
	return playPct, bufPct
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// loadingBufferPct oscillates a fake buffer fill while the stream URL resolves.
func loadingBufferPct() float64 {
	// 0→~0.85→0 triangle so the buffer overlay reads as "loading media".
	cycle := int(time.Now().UnixMilli()/40) % 160
	if cycle > 80 {
		cycle = 160 - cycle
	}
	return float64(cycle) / 80.0 * 0.85
}

// renderLayeredProgress draws background + buffer overlay + playhead overlay.
// Columns: play (accent) > buffer (gray) > empty (divider).
func renderLayeredProgress(width int, playPct, bufPct float64, playColor, bufColor lipgloss.Color) string {
	if width < 8 {
		width = 8
	}
	playPct = clamp01(playPct)
	bufPct = clamp01(bufPct)
	if bufPct < playPct {
		bufPct = playPct
	}

	playN := int(playPct * float64(width))
	bufN := int(bufPct * float64(width))
	if playPct > 0 && playN < 1 {
		playN = 1
	}
	if bufPct > 0 && bufN < 1 {
		bufN = 1
	}
	if playN > width {
		playN = width
	}
	if bufN > width {
		bufN = width
	}
	if bufN < playN {
		bufN = playN
	}

	play := lipgloss.NewStyle().Foreground(playColor).Background(colorBg)
	buf := lipgloss.NewStyle().Foreground(bufColor).Background(colorBg)
	empty := lipgloss.NewStyle().Foreground(colorDivider).Background(colorBg)

	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i < playN:
			b.WriteString(play.Render("━"))
		case i < bufN:
			b.WriteString(buf.Render("━"))
		default:
			b.WriteString(empty.Render("─"))
		}
	}
	return b.String()
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
