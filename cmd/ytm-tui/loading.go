package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/apirunner"
)

type apiResultMsg struct {
	api *apirunner.Runner
	err error
}

type loadingModel struct {
	spinner   spinner.Model
	err       error
	quitting  bool
	api       *apirunner.Runner
	firstTime bool // true only when the venv hasn't been built yet
}

func initialLoadingModel() loadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return loadingModel{
		spinner:   s,
		firstTime: !apirunner.VenvReady(),
	}
}

func (m loadingModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		api, err := apirunner.Start()
		return apiResultMsg{api: api, err: err}
	})
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			m.err = fmt.Errorf("user aborted startup")
			return m, tea.Quit
		}
	case apiResultMsg:
		m.api = msg.api
		m.err = msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m loadingModel) View() string {
	if m.quitting {
		return ""
	}
	msg := "Starting..."
	if m.firstTime {
		msg = "Setting things up for the first time... This might take a minute."
	}
	return fmt.Sprintf("\n\n   %s %s\n\n", m.spinner.View(), msg)
}
