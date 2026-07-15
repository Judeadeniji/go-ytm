package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/lyrics"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

const (
	npArtWidth  = 36
	npArtHeight = 16
	npMinWide   = 100 // below this, stack art above lyrics

	// After browsing, resume auto-follow once idle and the singing line is off-screen.
	lyricsIdleResync = 4 * time.Second
)

func lyricsTrackKey(t *Track) string {
	if t == nil {
		return ""
	}
	if t.VideoID != "" {
		return t.VideoID
	}
	return t.Title + "|" + t.Artist
}

// generateNowPlayingBody renders the full-screen now-playing surface
// (above the persistent player bar).
func (m Model) generateNowPlayingBody(width, height int) string {
	if width < 20 || height < 4 {
		return ""
	}

	var meta strings.Builder
	if m.currentTrack != nil {
		aw, ah := npArtWidth, npArtHeight
		if aw > width-4 {
			aw = max(12, width-4)
			ah = max(6, (aw*6)/13)
		}
		art := m.cachedArtAt(m.currentTrack.ThumbnailURL, aw, ah)
		meta.WriteString(art)
		meta.WriteString("\n\n")
		meta.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).
			MaxWidth(width - 4).Render(m.currentTrack.Title))
		meta.WriteString("\n")
		if m.currentTrack.Artist != "" {
			meta.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).
				MaxWidth(width - 4).Render(m.currentTrack.Artist))
			meta.WriteString("\n")
		}
		if m.currentTrack.Album != "" {
			meta.WriteString(lipgloss.NewStyle().Foreground(colorDivider).
				MaxWidth(width - 4).Render(m.currentTrack.Album))
			meta.WriteString("\n")
		}
	} else {
		meta.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).
			Render("Nothing playing"))
		meta.WriteString("\n")
	}
	meta.WriteString("\n")
	meta.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).
		Render("f / esc  close"))

	if width >= npMinWide && m.currentTrack != nil {
		leftW := npArtWidth + 6
		if leftW > width/2 {
			leftW = width / 2
		}
		rightW := width - leftW - 2
		if rightW < 24 {
			rightW = 24
			leftW = width - rightW - 2
		}
		left := lipgloss.NewStyle().
			Width(leftW).Height(height).MaxHeight(height).
			Padding(1, 2).
			Render(meta.String())
		right := lipgloss.NewStyle().
			Width(rightW).Height(height).MaxHeight(height).
			Padding(1, 2, 1, 0).
			Render(m.renderLyricsPane(rightW-2, height-2))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	// Narrow: stack metadata then lyrics.
	metaBlock := lipgloss.NewStyle().Width(width).Padding(1, 2).Render(meta.String())
	metaH := lipgloss.Height(metaBlock)
	lyricsH := height - metaH
	if lyricsH < 3 {
		lyricsH = 3
	}
	lyricsBlock := lipgloss.NewStyle().
		Width(width).Height(lyricsH).MaxHeight(lyricsH).
		Padding(0, 2).
		Render(m.renderLyricsPane(width-4, lyricsH))
	return lipgloss.JoinVertical(lipgloss.Left, metaBlock, lyricsBlock)
}

func (m Model) lyricsHeaderLabel() string {
	switch {
	case len(m.lyricsLines) == 0 && m.lyricsPlain == "":
		return "Lyrics"
	case m.lyricsFollow:
		return "Lyrics · following"
	default:
		return "Lyrics · browsing · c follow"
	}
}

func (m Model) lyricsHintLine(width int) string {
	hint := "j/k · wheel scroll · enter/click seek · c follow"
	return lipgloss.NewStyle().Foreground(colorSubtext).MaxWidth(max(8, width)).Render(hint)
}

func (m Model) renderLyricsPane(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(m.lyricsHeaderLabel())
	hintH := 1
	bodyH := max(1, height-2-hintH) // header + blank + hint
	var body string
	switch {
	case m.currentTrack == nil:
		body = lipgloss.NewStyle().Foreground(colorSubtext).Render("Play a track to see lyrics")
	case m.lyricsLoading:
		body = lipgloss.NewStyle().Foreground(colorSubtext).Render("Searching lyrics…")
	case m.lyricsErr != "":
		body = lipgloss.NewStyle().Foreground(colorSubtext).Render(m.lyricsErr)
	case m.lyricsInstrumental:
		body = lipgloss.NewStyle().Foreground(colorSubtext).Render("Instrumental")
	case len(m.lyricsLines) > 0:
		return m.renderSyncedLyrics(width, height, header)
	case m.lyricsPlain != "":
		plain := lipgloss.NewStyle().Foreground(colorText).Width(max(8, width)).Render(m.lyricsPlain)
		m.lyricsViewport.Width = max(8, width)
		m.lyricsViewport.Height = bodyH
		m.lyricsViewport.SetContent(plain)
		hint := m.lyricsHintLine(width)
		view := m.zone.Mark("lyrics_pane", safeViewportView(&m.lyricsViewport))
		return lipgloss.JoinVertical(lipgloss.Left, header, "", view, hint)
	default:
		body = lipgloss.NewStyle().Foreground(colorSubtext).Render("No lyrics found")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body)
}

func (m Model) renderSyncedLyrics(width, height int, header string) string {
	content, _ := m.buildSyncedLyricsContent(width)
	hintH := 1
	bodyH := max(1, height-2-hintH)
	m.lyricsViewport.Width = max(8, width)
	m.lyricsViewport.Height = bodyH
	m.lyricsViewport.SetContent(content)
	if m.lyricsFollow {
		m.applyLyricsFollowOffset()
	}
	hint := m.lyricsHintLine(width)
	view := m.zone.Mark("lyrics_pane", safeViewportView(&m.lyricsViewport))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", view, hint)
}

func (m Model) buildSyncedLyricsContent(width int) (string, int) {
	pos := time.Duration(m.playPos * float64(time.Second))
	if m.scrubbing {
		pos = time.Duration(m.scrubPos * float64(time.Second))
	}
	active := lyrics.ActiveLineIndex(m.lyricsLines, pos)
	cursor := m.lyricsCursor
	if m.lyricsFollow {
		cursor = active
	}

	innerW := max(8, width)
	var sb strings.Builder
	for i, ln := range m.lyricsLines {
		text := ln.Text
		if text == "" {
			text = " "
		}
		style := lipgloss.NewStyle().Foreground(colorSubtext).Width(innerW).MaxWidth(innerW)
		prefix := "  "
		switch {
		case i == active && i == cursor:
			style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Width(innerW).MaxWidth(innerW)
			prefix = "▶ "
		case i == active:
			style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Width(innerW).MaxWidth(innerW)
			prefix = "● "
		case i == cursor:
			style = lipgloss.NewStyle().Foreground(colorText).Bold(true).Width(innerW).MaxWidth(innerW)
			prefix = "› "
		case active >= 0 && absInt(i-active) == 1:
			style = lipgloss.NewStyle().Foreground(colorText).Width(innerW).MaxWidth(innerW)
		}
		row := style.Render(prefix + text)
		sb.WriteString(m.zone.Mark(fmt.Sprintf("lyrics_line_%d", i), row))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n"), active
}

func (m *Model) applyLyricsFollowOffset() {
	if !m.lyricsFollow || len(m.lyricsLines) == 0 {
		return
	}
	pos := time.Duration(m.playPos * float64(time.Second))
	if m.scrubbing {
		pos = time.Duration(m.scrubPos * float64(time.Second))
	}
	active := lyrics.ActiveLineIndex(m.lyricsLines, pos)
	if active < 0 {
		return
	}
	m.lyricsCursor = active
	h := m.lyricsViewport.Height
	if h < 1 {
		h = 1
	}
	mid := h / 2
	target := active - mid
	if target < 0 {
		target = 0
	}
	m.lyricsViewport.SetYOffset(target)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// syncLyricsFollowOffset keeps Model.lyricsViewport content+offset current so
// j/k LineUp/Down operate on real lines (View-only SetContent is not enough).
func (m *Model) syncLyricsFollowOffset() {
	if !m.nowPlayingOpen {
		return
	}
	w := m.lyricsViewport.Width
	if w < 8 {
		w = 8
	}
	if len(m.lyricsLines) > 0 {
		m.maybeSmartLyricsResync()
		content, _ := m.buildSyncedLyricsContent(w)
		prev := m.lyricsViewport.YOffset
		m.lyricsViewport.SetContent(content)
		if m.lyricsFollow {
			m.applyLyricsFollowOffset()
		} else {
			m.lyricsViewport.SetYOffset(prev)
			m.ensureLyricsCursorVisible()
		}
		return
	}
	if m.lyricsPlain != "" {
		plain := lipgloss.NewStyle().Foreground(colorText).Width(w).Render(m.lyricsPlain)
		prev := m.lyricsViewport.YOffset
		m.lyricsViewport.SetContent(plain)
		m.lyricsViewport.SetYOffset(prev)
	}
}

// maybeSmartLyricsResync re-enables follow after idle when the singing line
// left the viewport — never while the user is still looking near it.
func (m *Model) maybeSmartLyricsResync() {
	if m.lyricsFollow || len(m.lyricsLines) == 0 {
		return
	}
	if m.lyricsIdleAt.IsZero() || time.Since(m.lyricsIdleAt) < lyricsIdleResync {
		return
	}
	pos := time.Duration(m.playPos * float64(time.Second))
	active := lyrics.ActiveLineIndex(m.lyricsLines, pos)
	if active < 0 {
		return
	}
	if m.lyricsLineVisible(active) {
		return // still in view — don't yank scroll
	}
	m.lyricsFollow = true
	m.lyricsCursor = active
	m.lyricsIdleAt = time.Time{}
}

func (m Model) lyricsLineVisible(i int) bool {
	if i < 0 {
		return false
	}
	top := m.lyricsViewport.YOffset
	bottom := top + m.lyricsViewport.Height
	return i >= top && i < bottom
}

func (m *Model) ensureLyricsCursorVisible() {
	if m.lyricsCursor < 0 || m.lyricsViewport.Height <= 0 {
		return
	}
	top := m.lyricsViewport.YOffset
	bottom := top + m.lyricsViewport.Height
	if m.lyricsCursor < top {
		m.lyricsViewport.SetYOffset(m.lyricsCursor)
	} else if m.lyricsCursor >= bottom {
		m.lyricsViewport.SetYOffset(m.lyricsCursor - m.lyricsViewport.Height + 1)
	}
}

func (m *Model) pauseLyricsFollow() {
	m.lyricsFollow = false
	m.lyricsIdleAt = time.Now()
}

func (m *Model) resyncLyricsFollow() {
	m.lyricsFollow = true
	m.lyricsIdleAt = time.Time{}
	if len(m.lyricsLines) > 0 {
		pos := time.Duration(m.playPos * float64(time.Second))
		active := lyrics.ActiveLineIndex(m.lyricsLines, pos)
		if active >= 0 {
			m.lyricsCursor = active
		}
		m.applyLyricsFollowOffset()
	}
}

func (m *Model) moveLyricsCursor(delta int) {
	m.hydrateLyricsViewport()
	n := len(m.lyricsLines)
	if n == 0 {
		// Plain lyrics: viewport scroll only.
		if delta < 0 {
			m.lyricsViewport.LineUp(1)
		} else {
			m.lyricsViewport.LineDown(1)
		}
		m.pauseLyricsFollow()
		return
	}
	m.pauseLyricsFollow()
	if m.lyricsCursor < 0 {
		pos := time.Duration(m.playPos * float64(time.Second))
		m.lyricsCursor = lyrics.ActiveLineIndex(m.lyricsLines, pos)
		if m.lyricsCursor < 0 {
			m.lyricsCursor = 0
		}
	}
	m.lyricsCursor = clampIndex(m.lyricsCursor+delta, n)
	m.ensureLyricsCursorVisible()
	m.syncLyricsFollowOffset()
}

func (m Model) seekToLyricsLine(i int) (Model, tea.Cmd) {
	if i < 0 || i >= len(m.lyricsLines) || m.player == nil || !m.audioLoaded {
		return m, nil
	}
	sec := m.lyricsLines[i].Time.Seconds()
	m.playPos = sec
	m.lyricsCursor = i
	// Jumping to a line is an intentional "go here" — resume follow from that point.
	m.lyricsFollow = true
	m.lyricsIdleAt = time.Time{}
	m.applyLyricsFollowOffset()
	m.statusMsg = fmt.Sprintf("Seek %s", formatClock(sec))
	return m, tea.Batch(seekAbsoluteCmd(m.player, sec), fetchPlayProgress(m.player))
}

// handleLyricsClick seeks when a lyrics line zone is clicked.
func (m Model) handleLyricsClick(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if !m.nowPlayingOpen || len(m.lyricsLines) == 0 {
		return m, nil, false
	}
	// Fire on press (or legacy MouseLeft) — not on release, to avoid doubles.
	press := msg.Action == tea.MouseActionPress ||
		(msg.Type == tea.MouseLeft && msg.Action != tea.MouseActionRelease && msg.Action != tea.MouseActionMotion)
	if !press {
		return m, nil, false
	}
	for i := range m.lyricsLines {
		if m.zone.Get(fmt.Sprintf("lyrics_line_%d", i)).InBounds(msg) {
			mm, cmd := m.seekToLyricsLine(i)
			return mm, cmd, true
		}
	}
	return m, nil, false
}

// handleLyricsWheel scrolls lyrics when the pointer is over the lyrics pane.
func (m Model) handleLyricsWheel(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if !m.nowPlayingOpen || !tea.MouseEvent(msg).IsWheel() {
		return m, nil, false
	}
	if !m.mouseOverLyrics(msg) {
		return m, nil, false
	}
	// Content is often only set during View (on a copy); hydrate the Model
	// viewport before scrolling so LineUp/Down actually move.
	m.hydrateLyricsViewport()
	m.pauseLyricsFollow()
	var cmd tea.Cmd
	m.lyricsViewport, cmd = m.lyricsViewport.Update(msg)
	if len(m.lyricsLines) > 0 {
		m.lyricsCursor = clampIndex(m.lyricsViewport.YOffset, len(m.lyricsLines))
		m.ensureLyricsCursorVisible()
	}
	return m, cmd, true
}

// mouseOverLyrics reports whether the mouse is over the lyrics viewport (or
// the NP body if the zone isn't registered yet).
func (m Model) mouseOverLyrics(msg tea.MouseMsg) bool {
	if z := m.zone.Get("lyrics_pane"); z != nil && !z.IsZero() {
		return z.InBounds(msg)
	}
	if !m.nowPlayingOpen {
		return false
	}
	barTop := m.height - playerBarHeight
	if m.height > 0 && msg.Y >= barTop {
		return false
	}
	_, _, right := m.layoutWidths()
	if right > 0 && msg.X >= m.width-right {
		return false
	}
	return true
}

// hydrateLyricsViewport pushes lyrics content + size onto Model.lyricsViewport
// without changing follow mode (keeps the current YOffset when browsing).
func (m *Model) hydrateLyricsViewport() {
	m.ensureNowPlayingLayout()
	w := m.lyricsViewport.Width
	if w < 8 {
		w = 8
	}
	prev := m.lyricsViewport.YOffset
	switch {
	case len(m.lyricsLines) > 0:
		content, _ := m.buildSyncedLyricsContent(w)
		m.lyricsViewport.SetContent(content)
	case m.lyricsPlain != "":
		plain := lipgloss.NewStyle().Foreground(colorText).Width(w).Render(m.lyricsPlain)
		m.lyricsViewport.SetContent(plain)
	default:
		return
	}
	if m.lyricsFollow {
		m.applyLyricsFollowOffset()
	} else {
		m.lyricsViewport.SetYOffset(prev)
	}
}

func (m *Model) ensureNowPlayingLayout() {
	if !m.nowPlayingOpen {
		return
	}
	_, _, right := m.layoutWidths()
	w := m.width
	if right > 0 {
		w = m.width - right // NP body shares the row with the queue rail
	}
	h := m.contentHeight()
	if w < 1 || h < 1 {
		return
	}
	// Match renderLyricsPane body height: pane ≈ h-2, then −header/blank/hint.
	bodyH := max(3, h-5)
	if w >= npMinWide {
		m.lyricsViewport.Width = max(24, w/2-4)
		m.lyricsViewport.Height = bodyH
	} else {
		m.lyricsViewport.Width = max(8, w-4)
		m.lyricsViewport.Height = max(3, h/2-3)
	}
}

func (m *Model) enqueueNowPlayingImage() tea.Cmd {
	if m.currentTrack == nil || m.currentTrack.ThumbnailURL == "" {
		return nil
	}
	url := m.currentTrack.ThumbnailURL
	aw, ah := npArtWidth, npArtHeight
	key := imageCacheKey(url, aw, ah)
	if _, ok := m.imageCache[key]; ok {
		return nil
	}
	ph := KittyImage{Spacer: sizedPlaceholder(aw, ah)}
	m.putImageCache(key, &ph)
	return fetchImageSized(url, aw, ah)
}

// openNowPlaying opens the full-screen NP view and kicks off lyrics + art fetch.
func (m Model) openNowPlaying() (Model, tea.Cmd) {
	if m.currentTrack == nil {
		return m, nil
	}
	m.nowPlayingOpen = true
	m.lyricsFollow = true
	m.lyricsIdleAt = time.Time{}
	m.markSessionDirty()
	m.ensureNowPlayingLayout()
	if m.showQueuePanel() {
		m.setQueuePanelContent()
	}
	cmds := []tea.Cmd{m.enqueueNowPlayingImage()}
	if cmd := m.ensureLyricsFetched(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) closeNowPlaying() Model {
	m.nowPlayingOpen = false
	m.markSessionDirty()
	return m
}

func (m Model) toggleNowPlaying() (Model, tea.Cmd) {
	if m.nowPlayingOpen {
		return m.closeNowPlaying(), nil
	}
	return m.openNowPlaying()
}

// ensureLyricsFetched starts a lyrics fetch when the track key changed or we have none.
// Refetches once when duration becomes available after a duration=0 fetch.
func (m *Model) ensureLyricsFetched() tea.Cmd {
	if m.lyricsClient == nil || m.currentTrack == nil {
		return nil
	}
	key := lyricsTrackKey(m.currentTrack)
	haveResult := m.lyricsErr != "" || len(m.lyricsLines) > 0 || m.lyricsPlain != "" || m.lyricsInstrumental
	sameKey := key == m.lyricsTrackKey
	needDurationRefine := sameKey && haveResult && m.lyricsFetchDur < 0.5 && m.playDuration >= 0.5 && !m.lyricsLoading
	if sameKey && (m.lyricsLoading || (haveResult && !needDurationRefine)) {
		return nil
	}
	if !sameKey {
		m.lyricsTrackKey = key
		m.lyricsLines = nil
		m.lyricsPlain = ""
		m.lyricsErr = ""
		m.lyricsInstrumental = false
		m.lyricsCursor = -1
	}
	m.cancelLyrics()
	ctx, cancel := context.WithCancel(context.Background())
	m.lyricsCancel = cancel
	m.lyricsLoading = true
	m.lyricsGen++
	gen := m.lyricsGen
	m.lyricsFetchDur = m.playDuration
	album := m.currentTrack.Album
	return fetchLyrics(
		m.lyricsClient,
		key,
		m.currentTrack.Title,
		m.currentTrack.Artist,
		album,
		m.playDuration,
		gen,
		ctx,
	)
}

// ensureSongDetailsFetched loads get_song metadata for the current track once.
func (m *Model) ensureSongDetailsFetched() tea.Cmd {
	if m.ytmapiClient == nil || m.currentTrack == nil || m.currentTrack.VideoID == "" {
		return nil
	}
	vid := m.currentTrack.VideoID
	if vid == m.songDetailsVideoID && (m.songDetailsLoading || m.songDetails != nil || m.songDetailsErr != "") {
		return nil
	}
	if vid != m.songDetailsVideoID {
		m.songDetailsVideoID = vid
		m.songDetails = nil
		m.songDetailsErr = ""
	}
	m.cancelSongDetails()
	ctx, cancel := context.WithCancel(context.Background())
	m.songDetailsCancel = cancel
	m.songDetailsLoading = true
	m.songDetailsGen++
	return fetchSongDetails(m.ytmapiClient, vid, m.songDetailsGen, ctx)
}

// applySongDetails fills gaps on the current track from get_song.
func (m *Model) applySongDetails(song *ytmapi.SongDetails) {
	if song == nil || m.currentTrack == nil {
		return
	}
	t := m.currentTrack
	if t.Title == "" && song.Title != "" {
		t.Title = song.Title
	}
	if t.Artist == "" && song.Author != "" {
		t.Artist = song.Author
	}
	if t.ArtistID == "" && song.ChannelID != "" {
		t.ArtistID = song.ChannelID
	}
	if t.ThumbnailURL == "" && len(song.Thumbnails) > 0 {
		t.ThumbnailURL = song.Thumbnails[len(song.Thumbnails)-1].URL
		if t.ThumbnailURL == "" {
			t.ThumbnailURL = song.Thumbnails[0].URL
		}
	}
	if t.Duration == "" && song.LengthSeconds != "" {
		t.Duration = formatLengthSeconds(song.LengthSeconds)
	}
}

func formatLengthSeconds(s string) string {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return s
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return ""
	}
	return formatClock(float64(n))
}
