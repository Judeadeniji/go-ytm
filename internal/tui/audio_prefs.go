package tui

import (
	"fmt"
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

func setNormalizeCmd(p *player.Player, on bool) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		if err := p.SetNormalize(on); err != nil {
			return playerErrMsg{Op: "normalize", Err: err}
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
		m.statusMsg = "Loudness normalize on"
	} else {
		m.statusMsg = "Loudness normalize off"
	}
	m.setQueuePanelContent()
	return m, setNormalizeCmd(m.player, m.normalize)
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
		m.statusMsg = "Sleep timer off"
		m.setQueuePanelContent()
		return m, nil
	}
	m.sleepMinutes = next
	m.sleepUntil = time.Now().Add(time.Duration(next) * time.Minute)
	m.statusMsg = fmt.Sprintf("Sleep timer · %d min", next)
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
	m.statusMsg = "Sleep timer · paused"
	m.markSessionDirty()
	m.setQueuePanelContent()
	if m.onTracklistScreen() {
		m.setMainContent()
	}
	return m, tea.Batch(pausePlayback(m.player), fetchPlayProgress(m.player))
}
