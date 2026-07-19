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
		if m.currentTrack.Album != "" || (m.songDetails != nil && m.songDetails.Album != nil && m.songDetails.Album.Name != "") {
			albumName, _ := m.playingAlbumRef()
			if albumName != "" {
				albumLine := lipgloss.NewStyle().Foreground(colorAccent).
					MaxWidth(width - 4).Render(albumName)
				meta.WriteString(m.zone.Mark("np_album", albumLine))
				meta.WriteString("\n")
				viewAlbum := lipgloss.NewStyle().Foreground(colorSubtext).
					Render("View Album")
				meta.WriteString(m.zone.Mark("np_view_album", viewAlbum))
				meta.WriteString("\n")
			}
		}
	} else {
		meta.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).
			Render("Nothing playing"))
		meta.WriteString("\n")
	}
	meta.WriteString("\n")
	meta.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).
		Render("f / esc  close  ·  a View Album"))

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
			Render(m.renderNowPlayingRightPane(rightW-2, height-2))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	// Narrow: stack metadata then right pane.
	metaBlock := lipgloss.NewStyle().Width(width).Padding(1, 2).Render(meta.String())
	metaH := lipgloss.Height(metaBlock)
	rightH := height - metaH
	if rightH < 3 {
		rightH = 3
	}
	rightBlock := lipgloss.NewStyle().
		Width(width).Height(rightH).MaxHeight(rightH).
		Padding(0, 2).
		Render(m.renderNowPlayingRightPane(width-4, rightH))
	return lipgloss.JoinVertical(lipgloss.Left, metaBlock, rightBlock)
}

func (m Model) renderNowPlayingRightPane(width, height int) string {
	if m.nowPlayingTab == "" {
		m.nowPlayingTab = "lyrics"
	}

	var tabs []string
	for _, t := range []string{"lyrics", "related", "queue"} {
		style := lipgloss.NewStyle().Foreground(colorSubtext).Padding(0, 1)
		if m.nowPlayingTab == t {
			style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Padding(0, 1)
		}
		label := strings.ToUpper(t[:1]) + t[1:]
		tabs = append(tabs, m.zone.Mark("np_tab_"+t, style.Render(label)))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	header = lipgloss.NewStyle().PaddingBottom(1).Render(header)

	headerH := lipgloss.Height(header)
	bodyH := height - headerH
	if bodyH < 1 {
		bodyH = 1
	}

	var body string
	switch m.nowPlayingTab {
	case "lyrics":
		body = m.renderLyricsPaneBody(width, bodyH)
	case "related":
		body = m.renderRelatedPaneBody(width, bodyH)
	case "queue":
		body = m.generateQueuePanelContent(width)
	default:
		body = m.renderLyricsPaneBody(width, bodyH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *Model) hydrateRelatedViewport() {
	if m.relatedTracksLoading {
		hint := lipgloss.NewStyle().Foreground(colorSubtext).Render("Loading related content…")
		m.relatedViewport.SetContent(lipgloss.NewStyle().Padding(2).Render(hint))
		return
	}
	if m.relatedTracksErr != "" {
		m.relatedViewport.SetContent(lipgloss.NewStyle().Padding(2).Foreground(colorSubtext).Render(m.relatedTracksErr))
		return
	}
	if len(m.relatedTracks) == 0 {
		hint := lipgloss.NewStyle().Foreground(colorSubtext).Render("No related content available.")
		m.relatedViewport.SetContent(lipgloss.NewStyle().Padding(2).Render(hint))
		return
	}

	var sb strings.Builder
	for i, sec := range m.relatedTracks {
		if len(sec.Contents) == 0 {
			continue
		}
		sb.WriteString(m.renderRelatedCarouselRow(i+1000, sec.Title, sec.Contents, m.relatedViewport.Width))
		sb.WriteString("\n")
	}
	m.relatedViewport.SetContent(sb.String())
}

func (m Model) renderRelatedPaneBody(width, height int) string {
	m.relatedViewport.Width = width
	m.relatedViewport.Height = height
	return m.zone.Mark("related_pane", safeViewportView(&m.relatedViewport))
}

func (m Model) renderRelatedCarouselRow(index int, title string, cards []ytmapi.HomeCarouselItem, mainWidth int) string {
	var row strings.Builder

	contentWidth := mainWidth - 2
	maxVisible := 4

	// Calculate a dynamic card width so 4 items fit exactly
	cardWidth := contentWidth / maxVisible
	if cardWidth < 12 {
		cardWidth = 12
	}

	isActive := m.activePane == PaneMain && m.activeCarousel == index

	titleStyle := lipgloss.NewStyle().Bold(true)
	btnStyle := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2)

	if isActive {
		titleStyle = titleStyle.Foreground(colorText)
		btnStyle = btnStyle.Background(colorSearchBg).Foreground(colorText)
	} else {
		titleStyle = titleStyle.Foreground(colorSubtext)
		btnStyle = btnStyle.Background(colorBg).Foreground(colorSubtext)
	}

	titleStr := titleStyle.Render(title)
	leftBtn := m.zone.Mark(title+"_left", btnStyle.Render("<"))
	rightBtn := m.zone.Mark(title+"_right", btnStyle.Render(">"))
	arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)

	// ensure arrows align to the right edge exactly
	space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(arrows)
	if space < 1 {
		space = 1
	}
	
	row.WriteString(titleStr)
	row.WriteString(strings.Repeat(" ", space))
	row.WriteString(arrows)
	row.WriteString("\n\n")

	offset := m.carouselOffsets[title]
	if offset < 0 {
		offset = 0
	}
	if offset > len(cards) {
		offset = len(cards)
	}
	visibleCards := cards[offset:]
	if len(visibleCards) > maxVisible {
		visibleCards = visibleCards[:maxVisible]
	}

	var blocks []string
	for vi, card := range visibleCards {
		cardIndex := offset + vi
		t := card.Title
		
		// Use almost full cardWidth for art to minimize gaps
		cArtWidth := cardWidth - 2
		cArtHeight := cArtWidth / 2
		if cArtWidth < 4 {
			cArtWidth = 4
		}
		if cArtHeight < 2 {
			cArtHeight = 2
		}

		maxLen := cArtWidth
		if maxLen < 3 {
			maxLen = 3
		}
		
		if len(t) > maxLen {
			t = t[:maxLen-3] + "..."
		}
		if card.IsExplicit {
			t += explicitBadge() 
		}
		
		s := homeCardSubtitle(card)
		if len(s) > maxLen+2 && maxLen+2 > 3 {
			s = s[:maxLen-1] + "..."
		}

		art := lipgloss.NewStyle().Width(cArtWidth).Height(cArtHeight).Render("")
		if len(card.Thumbnails) > 0 {
			art = m.cachedArtAt(card.Thumbnails[0].URL, cArtWidth, cArtHeight)
		}

		titleColor := colorText
		focused := m.focusedHomeCard(index, cardIndex)
		if focused {
			titleColor = colorAccent
		}

		content := lipgloss.JoinVertical(lipgloss.Left,
			art, "",
			lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(t),
			lipgloss.NewStyle().Foreground(colorSubtext).Render(s),
		)

		if zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID); zid != "" {
			content = m.zone.Mark(zid, content)
		}

		// Use minimal gap between items (MarginRight 1)
		style := lipgloss.NewStyle().MarginRight(1).Width(cardWidth - 1)
		if focused {
			content = lipgloss.NewStyle().Background(colorFocusBg).Render(content)
		}
		blocks = append(blocks, style.Render(content))
	}

	row.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	return row.String() + "\n\n\n"
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

func (m Model) renderLyricsPaneBody(width, height int) string {
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
		// MaxWidth truncates; avoid Width() which wraps and breaks YOffset↔line index.
		style := lipgloss.NewStyle().Foreground(colorSubtext).MaxWidth(innerW)
		prefix := "  "
		switch {
		case i == active && i == cursor:
			style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).MaxWidth(innerW)
			prefix = "▶ "
		case i == active:
			style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).MaxWidth(innerW)
			prefix = "● "
		case i == cursor:
			style = lipgloss.NewStyle().Foreground(colorText).Bold(true).MaxWidth(innerW)
			prefix = "› "
		case active >= 0 && absInt(i-active) == 1:
			style = lipgloss.NewStyle().Foreground(colorText).MaxWidth(innerW)
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
// j/k ScrollUp/Down operate on real lines (View-only SetContent is not enough).
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
			m.lyricsViewport.ScrollUp(1)
		} else {
			m.lyricsViewport.ScrollDown(1)
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
	m.clearResumeSeek()
	m.applyLyricsFollowOffset()
	m.setStatus(fmt.Sprintf("Seek %s", formatClock(sec)))
	return m, tea.Batch(seekAbsoluteCmd(m.player, sec), fetchPlayProgress(m.player))
}

// handleLyricsClick seeks when a lyrics line zone is clicked.
func (m Model) handleLyricsClick(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if !m.nowPlayingOpen || len(m.lyricsLines) == 0 {
		return m, nil, false
	}
	// Fire on left-button press only — not on release or motion.
	press := msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress
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

// handleRelatedWheel scrolls the related pane when the pointer is over it.
func (m Model) handleRelatedWheel(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if !m.nowPlayingOpen || m.nowPlayingTab != "related" || !tea.MouseEvent(msg).IsWheel() {
		return m, nil, false
	}
	z := m.zone.Get("related_pane")
	if z == nil || z.IsZero() || !z.InBounds(msg) {
		return m, nil, false
	}
	var cmd tea.Cmd
	m.relatedViewport, cmd = m.relatedViewport.Update(msg)
	return m, cmd, true
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
	// viewport before scrolling so ScrollUp/Down actually move.
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

// mouseOverLyrics reports whether the mouse is over the lyrics viewport.
func (m Model) mouseOverLyrics(msg tea.MouseMsg) bool {
	z := m.zone.Get("lyrics_pane")
	if z == nil || z.IsZero() {
		return false
	}
	return z.InBounds(msg)
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
		leftW := npArtWidth + 6
		if leftW > w/2 {
			leftW = w / 2
		}
		rightW := w - leftW - 2
		if rightW < 24 {
			rightW = 24
		}
		m.lyricsViewport.Width = rightW - 2
		m.lyricsViewport.Height = bodyH
		m.relatedViewport.Width = m.lyricsViewport.Width
		m.relatedViewport.Height = bodyH
	} else {
		m.lyricsViewport.Width = max(8, w-4)
		m.lyricsViewport.Height = max(3, h/2-3)
		m.relatedViewport.Width = m.lyricsViewport.Width
		m.relatedViewport.Height = m.lyricsViewport.Height
	}
	m.hydrateRelatedViewport()
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
		m.sessionStore,
		m.currentTrack.VideoID,
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

// applySongDetails fills catalog gaps on the current track from song metadata.
func (m *Model) applySongDetails(song *ytmapi.SongDetails) {
	if song == nil || m.currentTrack == nil {
		return
	}
	t := m.currentTrack
	if song.Title != "" {
		t.Title = song.Title
	}
	if names := song.ArtistNames(); names != "" {
		t.Artist = names
	}
	if len(song.Artists) > 0 && song.Artists[0].ID != "" {
		t.ArtistID = song.Artists[0].ID
	}
	if song.Album != nil {
		if song.Album.Name != "" {
			t.Album = song.Album.Name
		}
		if song.Album.ID != "" {
			t.AlbumID = song.Album.ID
		}
	}
	if song.Duration != "" {
		t.Duration = song.Duration
	}
	if song.IsExplicit {
		t.IsExplicit = true
	}
	if t.ThumbnailURL == "" && len(song.Thumbnails) > 0 {
		t.ThumbnailURL = song.Thumbnails[len(song.Thumbnails)-1].URL
		if t.ThumbnailURL == "" {
			t.ThumbnailURL = song.Thumbnails[0].URL
		}
	}
}
