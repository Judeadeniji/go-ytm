package tui

import (
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const toastTTL = 3 * time.Second

type toastTickMsg struct{}

func tickToast() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return toastTickMsg{}
	})
}

// setStatus shows an in-app toast. Auto-dismisses after toastTTL (refreshed on each call).
func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.toastAt = time.Now()
}

// notifyDesktop shows an in-app toast and pipes a copy to the OS notification daemon
// (notify-send on Linux). Failures are ignored — toast still works offline.
func (m *Model) notifyDesktop(title, body string) {
	m.setStatus(body)
	if title == "" {
		title = "Music"
	}
	go func() {
		path, err := exec.LookPath("notify-send")
		if err != nil {
			return
		}
		_ = exec.Command(path, "-a", "Music", "-t", "4000", "--hint", "string:desktop-entry:music", title, body).Run()
	}()
}

func (m *Model) clearToast() {
	m.statusMsg = ""
	m.toastAt = time.Time{}
}

// invalidateAuthCaches clears all page- and data-caches that depend on the
// authenticated identity. Call this whenever auth state changes so the next
// navigation/library access fetches fresh, personalised data.
func (m *Model) invalidateAuthCaches() {
	// Content pages (artist / album / playlist / podcast / profile)
	m.pageCache = make(map[string]any)
	m.artistPage = nil
	m.albumPage = nil
	m.playlistPage = nil
	m.podcastPage = nil
	m.userPage = nil

	// Home & explore feeds
	m.homeCarousels = nil
	m.exploreData = nil
	m.moodCategories = nil
	m.moodPlaylists = nil
	m.chartsData = nil

	// Library tabs
	m.libPlaylists = nil
	m.libSongs = nil
	m.libAlbums = nil
	m.libArtists = nil

	// User profile badge
	m.userProfile = nil

	// Search results are user-specific when auth is present
	m.searchResults = nil
	m.lastSearchQuery = ""
}

func (m *Model) expireToast() bool {
	if m.statusMsg == "" || m.toastAt.IsZero() {
		return false
	}
	if time.Since(m.toastAt) < toastTTL {
		return false
	}
	m.clearToast()
	return true
}

func (m Model) renderToast() string {
	if m.statusMsg == "" || m.toastAt.IsZero() {
		return ""
	}
	msg := strings.TrimSpace(m.statusMsg)
	if msg == "" {
		return ""
	}
	const maxRunes = 56
	if utf8.RuneCountInString(msg) > maxRunes {
		runes := []rune(msg)
		msg = string(runes[:maxRunes-1]) + "…"
	}

	fg := colorText
	accent := colorAccent
	switch {
	case strings.Contains(strings.ToLower(msg), "fail"),
		strings.Contains(strings.ToLower(msg), "error"),
		strings.Contains(strings.ToLower(msg), "unavailable"):
		accent = colorRed
	case strings.Contains(strings.ToLower(msg), "download"),
		strings.HasPrefix(msg, "Playing"),
		strings.HasPrefix(msg, "Successfully"):
		accent = lipgloss.Color("#3FB950")
	}

	inner := lipgloss.NewStyle().
		Foreground(fg).
		Background(colorSearchBg).
		Padding(0, 2).
		Render(msg)

	bar := lipgloss.NewStyle().
		Foreground(accent).
		Background(colorSearchBg).
		Bold(true).
		Render("▌")

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorDivider).
		Background(colorSearchBg).
		Render(bar + inner)
}

// placeOverlay draws fg on top of bg at cell (x, y). ANSI-aware via charmbracelet/x/ansi.
func placeOverlay(bg, fg string, x, y int) string {
	if fg == "" {
		return bg
	}
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	for i, fl := range fgLines {
		yi := y + i
		if yi < 0 || yi >= len(bgLines) {
			continue
		}
		bgLines[yi] = overlayLine(bgLines[yi], fl, x)
	}
	return strings.Join(bgLines, "\n")
}

func overlayLine(bgLine, fgLine string, x int) string {
	if x < 0 {
		x = 0
	}
	bgW := ansi.StringWidth(bgLine)
	fgW := ansi.StringWidth(fgLine)
	if fgW == 0 {
		return bgLine
	}
	if x >= bgW {
		// Pad then append.
		return bgLine + strings.Repeat(" ", x-bgW) + fgLine
	}
	left := ansi.Cut(bgLine, 0, x)
	rightStart := x + fgW
	right := ""
	if rightStart < bgW {
		right = ansi.Cut(bgLine, rightStart, bgW)
	}
	return left + fgLine + right
}
