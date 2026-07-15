package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/session"
)

const (
	// Extra seconds before the fade window to append+buffer the next URL.
	crossfadeArmPrepSec = 8
)

var crossfadeSecSteps = []int{1, 2, 3, 5, 8, 12}

// streamArmMsg is the result of appending the next track into mpv's playlist.
type streamArmMsg struct {
	VideoID string
	Err     error
}

func (m Model) playbackPrefs() session.PlaybackPrefs {
	return session.PlaybackPrefs{
		Crossfade:    m.crossfade,
		CrossfadeSec: session.ClampCrossfadeSec(m.crossfadeSec),
	}
}

func (m *Model) setCrossfadeEnabled(on bool) {
	m.crossfade = on
	m.markSessionDirty()
}

func (m *Model) setCrossfadeSec(sec int) {
	m.crossfadeSec = session.ClampCrossfadeSec(sec)
	m.markSessionDirty()
}

func (m Model) crossfadeSecLabel() string {
	if !m.crossfade {
		return "Off"
	}
	return fmt.Sprintf("%ds", session.ClampCrossfadeSec(m.crossfadeSec))
}

func (m Model) toggleCrossfade() (Model, tea.Cmd) {
	m.setCrossfadeEnabled(!m.crossfade)
	if m.crossfade {
		m.statusMsg = fmt.Sprintf("Crossfade on · %ds", session.ClampCrossfadeSec(m.crossfadeSec))
		m.setQueuePanelContent()
		return m, m.ensureUpcomingArmed()
	}
	m.statusMsg = "Crossfade off"
	m.setQueuePanelContent()
	mm, cmd := m.disarmCrossfade(true)
	return mm, cmd
}

func (m Model) cycleCrossfadeSec() (Model, tea.Cmd) {
	cur := session.ClampCrossfadeSec(m.crossfadeSec)
	idx := 0
	for i, s := range crossfadeSecSteps {
		if s == cur {
			idx = i
			break
		}
	}
	next := crossfadeSecSteps[(idx+1)%len(crossfadeSecSteps)]
	m.setCrossfadeSec(next)
	if !m.crossfade {
		m.setCrossfadeEnabled(true)
	}
	m.statusMsg = fmt.Sprintf("Crossfade · %ds", next)
	m.setQueuePanelContent()
	return m, m.ensureUpcomingArmed()
}

// clearCrossfadeArmState drops local arm flags without touching mpv.
func (m *Model) clearCrossfadeArmState() {
	m.armedVideoID = ""
	m.crossfadeFading = false
	m.armInflight = ""
}

// disarmCrossfade drops an armed next entry from mpv and restores volume.
func (m Model) disarmCrossfade(restoreVol bool) (Model, tea.Cmd) {
	had := m.armedVideoID != "" || m.armInflight != ""
	m.clearCrossfadeArmState()
	var cmds []tea.Cmd
	if restoreVol && !m.muted {
		cmds = append(cmds, setVolumeOnlyCmd(m.player, m.volume))
	}
	if had {
		cmds = append(cmds, dropArmedPlaylistEntry(m.player))
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func setVolumeOnlyCmd(p *player.Player, volume float64) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		if err := p.SetVolume(volume); err != nil {
			return playerErrMsg{Op: "volume", Err: err}
		}
		return nil
	}
}

func dropArmedPlaylistEntry(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		n, err := p.PlaylistCount()
		if err != nil || n < 2 {
			return nil
		}
		pos, err := p.PlaylistPos()
		if err != nil || pos < 0 {
			return nil
		}
		// Drop everything after the current entry.
		for i := n - 1; i > pos; i-- {
			_ = p.PlaylistRemove(i)
		}
		return nil
	}
}

func appendStreamCmd(p *player.Player, videoID, url string) tea.Cmd {
	return func() tea.Msg {
		if p == nil || url == "" {
			return streamArmMsg{VideoID: videoID, Err: fmt.Errorf("missing player or url")}
		}
		if err := p.Append(url); err != nil {
			return streamArmMsg{VideoID: videoID, Err: err}
		}
		return streamArmMsg{VideoID: videoID}
	}
}

// ensureUpcomingArmed appends the next queue URL into mpv when crossfade is on.
func (m *Model) ensureUpcomingArmed() tea.Cmd {
	prefs := m.playbackPrefs()
	if !prefs.Crossfade || !m.audioLoaded || m.player == nil {
		return nil
	}
	cur := m.queue.CurrentIndex()
	if cur < 0 {
		return nil
	}
	next, ok := m.queue.At(cur + 1)
	if !ok || next.VideoID == "" {
		return nil
	}
	if m.armedVideoID == next.VideoID || m.armInflight == next.VideoID {
		return nil
	}

	var cmds []tea.Cmd
	if m.armedVideoID != "" && m.armedVideoID != next.VideoID {
		had := true
		m.clearCrossfadeArmState()
		if had {
			cmds = append(cmds, dropArmedPlaylistEntry(m.player))
		}
	}

	if m.playDuration < 0.5 {
		return tea.Batch(cmds...)
	}
	left := m.playDuration - m.playPos
	window := float64(prefs.CrossfadeSec + crossfadeArmPrepSec)
	if left > window {
		return tea.Batch(cmds...)
	}
	url, hit := m.peekStreamCache(next.VideoID)
	if !hit {
		cmds = append(cmds, m.ensureUpcomingPreloaded())
		return tea.Batch(cmds...)
	}
	m.armInflight = next.VideoID
	cmds = append(cmds, appendStreamCmd(m.player, next.VideoID, url))
	return tea.Batch(cmds...)
}

// applyCrossfadeVolume returns a cmd to dip volume near EOF when armed.
func (m *Model) applyCrossfadeVolume() tea.Cmd {
	prefs := m.playbackPrefs()
	if !prefs.Crossfade || m.armedVideoID == "" || m.muted || !m.audioLoaded {
		return nil
	}
	if m.playDuration < 1 {
		return nil
	}
	left := m.playDuration - m.playPos
	sec := float64(prefs.CrossfadeSec)
	if left > sec || sec < 0.5 {
		if m.crossfadeFading {
			// Left the fade window (seek back) — restore.
			m.crossfadeFading = false
			return setVolumeOnlyCmd(m.player, m.volume)
		}
		return nil
	}
	m.crossfadeFading = true
	frac := left / sec
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return setVolumeOnlyCmd(m.player, m.volume*frac)
}

// promoteArmedTrack advances the queue after mpv already started the appended file.
func (m Model) promoteArmedTrack() (Model, tea.Cmd) {
	armed := m.armedVideoID
	m.clearCrossfadeArmState()
	t, ok := m.queue.Next()
	if !ok || t.VideoID == "" || (armed != "" && t.VideoID != armed) {
		// Mismatch — fall back to full reload if we somehow advanced wrong.
		if ok {
			return m.startQueuedTrack(t)
		}
		m.isPlaying = false
		m.statusMsg = "End of queue"
		return m, setVolumeOnlyCmd(m.player, m.volume)
	}

	m.currentTrack = &t
	m.isPlaying = true
	m.audioLoaded = true
	m.resumeSeek = 0
	m.resumeSeekTries = 0
	m.playPos = 0
	m.playDuration = 0
	m.playBuffered = 0
	m.playGen++
	sideCmd := m.onTrackChanged()
	m.queueCursor = m.queue.CurrentIndex()
	m.statusMsg = "Playing: " + t.Title
	if m.onTracklistScreen() {
		m = m.syncTrackCursorToPlaying()
		m.ensureTrackCursorInView(10, 1)
		m.setMainContent()
	}
	m.applyLayout()
	m.setQueuePanelContent()
	m.markSessionDirty()

	cmds := []tea.Cmd{
		setVolumeOnlyCmd(m.player, m.volume),
		fetchPlayProgress(m.player),
		m.enqueueVisibleImages(m.mainWidth()),
		sideCmd,
	}
	if m.nowPlayingOpen {
		m.ensureNowPlayingLayout()
		cmds = append(cmds, m.enqueueNowPlayingImage())
	}
	if cmd := m.ensureUpcomingPreloaded(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.ensureUpcomingArmed(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// invalidateArmedIfNextChanged drops arm when queue "up next" no longer matches.
func (m Model) invalidateArmedIfNextChanged() (Model, tea.Cmd) {
	if m.armedVideoID == "" && m.armInflight == "" {
		return m, nil
	}
	cur := m.queue.CurrentIndex()
	next, ok := m.queue.At(cur + 1)
	want := ""
	if ok {
		want = next.VideoID
	}
	armed := m.armedVideoID
	if armed == "" {
		armed = m.armInflight
	}
	if want != "" && want == armed {
		return m, nil
	}
	return m.disarmCrossfade(true)
}
