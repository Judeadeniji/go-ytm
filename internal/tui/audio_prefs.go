package tui

import (
	"fmt"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/player"
)

type sleepTickMsg time.Time

var sleepCycleMinutes = []int{0, 15, 30, 45, 60}

func tickSleep() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return sleepTickMsg(t)
	})
}

type EQPreset struct {
	Name   string
	Filter string
}

var eqPresets = []EQPreset{
	{"Flat", ""},
	{"Bass Boost", "bass=g=8:f=110:w=0.6"},
	{"Treble Boost", "treble=g=6:f=8000:w=0.5"},
	{"V-Shape", "bass=g=6:f=100:w=0.5,treble=g=6:f=8000:w=0.5"},
	{"Vocal", "bass=g=-2:f=100:w=0.5,treble=g=-2:f=8000:w=0.5"},
}

func setAudioFiltersCmd(p *player.Player, normalize, silenceSkip bool, pitch float64, eqFilter string) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		if err := p.SetAudioFilters(normalize, silenceSkip, pitch, eqFilter); err != nil {
			return playerErrMsg{Op: "audio_filters", Err: err}
		}
		return nil
	}
}

func setTempoCmd(p *player.Player, tempo float64) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		if err := p.SetTempo(tempo); err != nil {
			return playerErrMsg{Op: "tempo", Err: err}
		}
		return nil
	}
}

func pausePlayback(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		if err := p.Pause(); err != nil {
			return playerErrMsg{Op: "pause", Err: err}
		}
		return nil
	}
}

func (m Model) toggleNormalize() (Model, tea.Cmd) {
	m.normalize = !m.normalize
	m.markSessionDirty()
	if m.normalize {
		m.setStatus("Loudness normalize on")
	} else {
		m.setStatus("Loudness normalize off")
	}
	m.setQueuePanelContent()
	return m, setAudioFiltersCmd(m.player, m.normalize, m.silenceSkip, m.pitch, eqPresets[m.eqPreset].Filter)
}

func (m Model) toggleSilenceSkip() (Model, tea.Cmd) {
	m.silenceSkip = !m.silenceSkip
	m.markSessionDirty()
	if m.silenceSkip {
		m.setStatus("Silence skip on")
	} else {
		m.setStatus("Silence skip off")
	}
	m.setQueuePanelContent()
	return m, setAudioFiltersCmd(m.player, m.normalize, m.silenceSkip, m.pitch, eqPresets[m.eqPreset].Filter)
}

func (m Model) adjustTempo(delta float64) (Model, tea.Cmd) {
	m.tempo += delta
	m.tempo = math.Round(m.tempo*100) / 100
	if m.tempo < 0.25 {
		m.tempo = 0.25
	}
	if m.tempo > 4.0 {
		m.tempo = 4.0
	}
	m.markSessionDirty()
	m.setStatus(fmt.Sprintf("Tempo · %.2fx", m.tempo))
	m.setQueuePanelContent()
	return m, setTempoCmd(m.player, m.tempo)
}

func (m Model) resetTempo() (Model, tea.Cmd) {
	m.tempo = 1.0
	m.markSessionDirty()
	m.setStatus("Tempo · 1.00x")
	m.setQueuePanelContent()
	return m, setTempoCmd(m.player, m.tempo)
}

func (m Model) adjustPitch(delta float64) (Model, tea.Cmd) {
	m.pitch += delta
	if m.pitch < -12 {
		m.pitch = -12
	}
	if m.pitch > 12 {
		m.pitch = 12
	}
	m.markSessionDirty()
	m.setStatus(fmt.Sprintf("Pitch · %+.1f semi", m.pitch))
	m.setQueuePanelContent()
	return m, setAudioFiltersCmd(m.player, m.normalize, m.silenceSkip, m.pitch, eqPresets[m.eqPreset].Filter)
}

func (m Model) resetPitch() (Model, tea.Cmd) {
	m.pitch = 0
	m.markSessionDirty()
	m.setStatus("Pitch · +0.0 semi")
	m.setQueuePanelContent()
	return m, setAudioFiltersCmd(m.player, m.normalize, m.silenceSkip, m.pitch, eqPresets[m.eqPreset].Filter)
}

func (m Model) cycleEQPreset() (Model, tea.Cmd) {
	m.eqPreset = (m.eqPreset + 1) % len(eqPresets)
	m.markSessionDirty()
	m.setStatus(fmt.Sprintf("EQ · %s", eqPresets[m.eqPreset].Name))
	m.setQueuePanelContent()
	return m, setAudioFiltersCmd(m.player, m.normalize, m.silenceSkip, m.pitch, eqPresets[m.eqPreset].Filter)
}

// cycleSleepTimer steps Off → 15 → 30 → 45 → 60 → Off.
func (m Model) cycleSleepTimer() (Model, tea.Cmd) {
	idx := 0
	for i, step := range sleepCycleMinutes {
		if step == m.sleepMinutes {
			idx = i
			break
		}
	}
	next := sleepCycleMinutes[(idx+1)%len(sleepCycleMinutes)]
	if next == 0 {
		m.sleepUntil = time.Time{}
		m.sleepMinutes = 0
		m.setStatus("Sleep timer off")
		m.setQueuePanelContent()
		return m, nil
	}
	m.sleepMinutes = next
	m.sleepUntil = time.Now().Add(time.Duration(next) * time.Minute)
	m.setStatus(fmt.Sprintf("Sleep timer · %d min", next))
	m.setQueuePanelContent()
	return m, tickSleep()
}

func (m Model) sleepRemainingLabel() string {
	if m.sleepUntil.IsZero() || m.sleepMinutes == 0 {
		return "Off"
	}
	left := time.Until(m.sleepUntil)
	if left <= 0 {
		return "…"
	}
	total := int(left.Round(time.Second) / time.Second)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func (m Model) handleSleepTick() (Model, tea.Cmd) {
	if m.sleepUntil.IsZero() {
		return m, nil
	}
	if time.Now().Before(m.sleepUntil) {
		m.setQueuePanelContent()
		return m, tickSleep()
	}
	m.sleepUntil = time.Time{}
	m.sleepMinutes = 0
	m.isPlaying = false
	m.setStatus("Sleep timer · paused")
	m.markSessionDirty()
	m.setQueuePanelContent()
	if m.onTracklistScreen() {
		m.setMainContent()
	}
	return m, tea.Batch(pausePlayback(m.player), fetchPlayProgress(m.player))
}
