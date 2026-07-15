package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// playableTracks returns playlist/album tracks that have a videoId, preserving index.
func playableTracks(tracks []ytmapi.TrackItem) []ytmapi.TrackItem {
	out := make([]ytmapi.TrackItem, 0, len(tracks))
	for _, tr := range tracks {
		if tr.VideoID != "" {
			out = append(out, tr)
		}
	}
	return out
}

func (m Model) onTracklistScreen() bool {
	sc, ok := m.stack.Current()
	if !ok {
		return false
	}
	return sc.Kind == ScreenPlaylist || sc.Kind == ScreenAlbum
}

func (m Model) tracklistTracks() []ytmapi.TrackItem {
	sc, ok := m.stack.Current()
	if !ok {
		return nil
	}
	switch sc.Kind {
	case ScreenPlaylist:
		if m.playlistPage == nil {
			return nil
		}
		return playableTracks(m.playlistPage.Tracks)
	case ScreenAlbum:
		if m.albumPage == nil {
			return nil
		}
		return playableTracks(m.albumPage.Tracks)
	}
	return nil
}

func (m Model) cachedArtAt(url string, width, height int) string {
	if url == "" {
		return sizedPlaceholder(width, height)
	}
	key := imageCacheKey(url, width, height)
	if kitty, ok := m.imageCache[key]; ok && kitty != nil && kitty.Spacer != "" {
		return kitty.Spacer
	}
	return sizedPlaceholder(width, height)
}

func firstThumbURL(thumbs []ytmapi.Thumbnail) string {
	if len(thumbs) == 0 {
		return ""
	}
	// Prefer larger thumbs when available.
	best := thumbs[0]
	for _, t := range thumbs[1:] {
		if t.Width >= best.Width {
			best = t
		}
	}
	return best.URL
}

// renderTrackRow draws one full-width tracklist line with focus and now-playing state.
// viewsW > 0 reserves a right-aligned views/plays column before duration.
func (m Model) renderTrackRow(i int, tr ytmapi.TrackItem, mainWidth int, focused bool, viewsW int) string {
	playing := m.currentTrack != nil && m.currentTrack.VideoID != "" && m.currentTrack.VideoID == tr.VideoID

	bg := colorBg
	switch {
	case focused:
		bg = colorFocusBg
	case playing:
		// Keep the playing row visibly distinct even when the cursor is elsewhere.
		bg = colorSearchBg
	}

	indicator := "  "
	indStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(2)
	if playing {
		if m.isPlaying {
			indicator = "❚❚"
		} else {
			indicator = "▶"
		}
		indStyle = indStyle.Foreground(colorAccent)
	} else if focused {
		indicator = "›"
		indStyle = indStyle.Foreground(colorText)
	}

	numW := 4
	durW := 7
	viewsGap := 0
	if viewsW > 0 {
		viewsGap = 1
	}
	// Full width: indicator + number + title + artist + optional views + duration
	textBudget := mainWidth - 4 - 2 - numW - durW - viewsW - viewsGap - 4
	if textBudget < 16 {
		textBudget = 16
	}
	titleW := textBudget * 3 / 5
	artistW := textBudget - titleW
	if artistW < 10 {
		artistW = 10
		titleW = textBudget - artistW
	}

	titleColor := colorText
	numColor := colorSubtext
	if playing {
		titleColor = colorAccent
		numColor = colorAccent
	}

	num := lipgloss.NewStyle().Foreground(numColor).Background(bg).Width(numW).Render(fmt.Sprintf("%d", i+1))
	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Width(titleW).MaxWidth(titleW).Render(tr.Title)
	artist := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(artistW).MaxWidth(artistW).Render(tr.ArtistName())
	dur := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(durW).Align(lipgloss.Right).Render(tr.DurationLabel())

	parts := []string{
		indStyle.Render(indicator),
		" ",
		num,
		title,
		" ",
		artist,
	}
	if viewsW > 0 {
		views := ytmapi.FormatCount(tr.Views)
		parts = append(parts, " ",
			lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(viewsW).Align(lipgloss.Right).Render(views),
		)
	}
	parts = append(parts, " ", dur)

	row := lipgloss.JoinHorizontal(lipgloss.Center, parts...)

	row = lipgloss.NewStyle().Background(bg).Width(mainWidth - 2).MaxWidth(mainWidth - 2).Render(row)

	if tr.VideoID != "" {
		row = m.zone.Mark("play_video_"+tr.VideoID, row)
	}
	return row
}

// tracklistViewsWidth returns a views column width when any track has a count, else 0.
func tracklistViewsWidth(tracks []ytmapi.TrackItem) int {
	w := 0
	for _, tr := range tracks {
		v := ytmapi.FormatCount(tr.Views)
		if v == "" {
			continue
		}
		if len(v) > w {
			w = len(v)
		}
	}
	if w == 0 {
		return 0
	}
	if w < 4 {
		w = 4
	}
	if w > 8 {
		w = 8
	}
	return w
}

// ensureTrackCursorInView nudges the main viewport so the focused row stays visible.
func (m *Model) ensureTrackCursorInView(headerLines, rowHeight int) {
	tracks := m.tracklistTracks()
	if len(tracks) == 0 {
		return
	}
	if m.trackCursor < 0 {
		m.trackCursor = 0
	}
	if m.trackCursor >= len(tracks) {
		m.trackCursor = len(tracks) - 1
	}

	viewH := m.mainViewport.Height
	if viewH <= 0 {
		return
	}
	cursorLine := headerLines + m.trackCursor*rowHeight
	top := m.mainViewport.YOffset
	bottom := top + viewH - 1

	if cursorLine < top {
		m.mainViewport.SetYOffset(cursorLine)
	} else if cursorLine+rowHeight-1 > bottom {
		m.mainViewport.SetYOffset(cursorLine + rowHeight - viewH)
	}
}

func (m Model) syncTrackCursorToPlaying() Model {
	tracks := m.tracklistTracks()
	if m.currentTrack == nil || len(tracks) == 0 {
		return m
	}
	for i, tr := range tracks {
		if tr.VideoID == m.currentTrack.VideoID {
			m.trackCursor = i
			break
		}
	}
	return m
}

func (m Model) moveTrackCursor(delta int) Model {
	tracks := m.tracklistTracks()
	if len(tracks) == 0 {
		return m
	}
	m.trackCursor += delta
	if m.trackCursor < 0 {
		m.trackCursor = 0
	}
	if m.trackCursor >= len(tracks) {
		m.trackCursor = len(tracks) - 1
	}
	m.ensureTrackCursorInView(10, 1)
	m.setMainContent()
	return m
}

func (m Model) playFocusedTrack() (Model, tea.Cmd) {
	tracks := m.tracklistTracks()
	if len(tracks) == 0 || m.trackCursor < 0 || m.trackCursor >= len(tracks) {
		return m, nil
	}
	return m.playTracklistFrom(m.trackCursor)
}
