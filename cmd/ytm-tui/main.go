package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/tui"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func main() {
	p, err := player.NewPlayer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting MPV player: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	ext := search.NewExtractor()
	apiClient := ytmapi.NewClient()

	m := tui.NewModel(p, ext, apiClient)
	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}
