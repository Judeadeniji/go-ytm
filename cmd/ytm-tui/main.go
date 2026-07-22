package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/apirunner"
	"github.com/judeadeniji/go-ytm/internal/lyrics"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/tui"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
	runewidth "github.com/mattn/go-runewidth"
)

// Set via -ldflags at build/release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "-help", "--help", "help":
			printHelp()
			return
		case "-version", "--version", "version":
			fmt.Printf("ytm %s (commit %s, built %s)\n", version, commit, date)
			return
		case "doctor", "--doctor":
			fmt.Print(apirunner.Doctor(version).Format())
			return
		case "upgrade", "--upgrade":
			upgradeYTM()
			return
		}
	}

	paths, _ := apirunner.ResolvePaths()
	_ = paths.EnsureStateDir()

	var logPath string
	if os.Getenv("YTM_DEV") == "1" {
		_ = os.MkdirAll("tmp", 0700)
		logPath = "tmp/ytm-tui.log"
	} else {
		logPath = filepath.Join(paths.StateDir, "tui.log")
	}

	f, err := tea.LogToFile(logPath, "tui")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	logger := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	if err := apirunner.CheckMPV(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s", err, apirunner.DepHints())
		os.Exit(1)
	}

	slog.Info("Starting ytm-api supervisor with loading screen...")
	loadingProg := tea.NewProgram(initialLoadingModel())
	lm, err := loadingProg.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running loading screen: %v\n", err)
		os.Exit(1)
	}
	finalModel := lm.(loadingModel)
	if finalModel.err != nil {
		fmt.Fprintf(os.Stderr, "Error starting ytm-api: %v\n\nRun: ytm doctor\n", finalModel.err)
		os.Exit(1)
	}
	api := finalModel.api
	defer func() { _ = api.Stop() }()

	slog.Info("Starting go-ytm TUI...", "version", version)
	p, err := player.NewPlayer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting MPV player: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	ext := search.NewExtractor()
	apiClient := ytmapi.NewClient()
	lyricsClient := lyrics.NewClient()

	m := tui.NewModel(p, ext, apiClient, lyricsClient)
	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.SetProgram(prog)
	errWait := make(chan error, 1)
	go func() {
		if err := api.WaitHealthy(); err != nil {
			errWait <- err
			prog.Send(tea.Quit())
		}
	}()

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
		os.Exit(1)
	}

	select {
	case err := <-errWait:
		fmt.Fprintf(os.Stderr, "\nFatal API Error: %v\n", err)
		os.Exit(1)
	default:
	}
}

func printHelp() {
	fmt.Printf(`ytm — terminal YouTube Music client

Usage:
  ytm              Start the player (boots local ytm-api if needed)
  ytm doctor       Show paths, dependencies, and API health
  ytm upgrade      Upgrade to the latest release
  ytm --version    Print version
  ytm --help       Show this help

In-app: press ? for keyboard shortcuts.

Install:
  curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash

Requires: mpv, python3. Recommended: yt-dlp.
State:  ~/.local/state/go-ytm
API:    ~/.local/share/go-ytm/ytm-api  (or YTM_API_HOME)

`)
}

func upgradeYTM() {
	fmt.Println("==> Upgrading go-ytm to the latest version...")
	cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Upgrade failed: %v\n", err)
		os.Exit(1)
	}
}
