package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/tui"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func init() {
	// U+10EEEE (Kitty Graphics Protocol Unicode placeholder) sits above U+20000
	// which go-runewidth classifies as East Asian wide (width=2). This breaks
	// lipgloss layout: it thinks 24 placeholder chars = 48 cols and truncates them,
	// so Kitty only receives 12 chars and renders a broken/black image.
	// Setting EastAsianWidth=false makes U+10EEEE return width=1, matching
	// Kitty's own treatment of the character.
	runewidth.DefaultCondition.EastAsianWidth = false
}

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
