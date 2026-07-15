package tui

import (
	"context"
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

func (m Model) renderLyricsPane(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Lyrics")
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
		m.lyricsViewport.Height = max(1, height-2)
		m.lyricsViewport.SetContent(plain)
		return lipgloss.JoinVertical(lipgloss.Left, header, "", safeViewportView(&m.lyricsViewport))
	default:
		body = lipgloss.NewStyle().Foreground(colorSubtext).Render("No lyrics found")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body)
}

func (m Model) renderSyncedLyrics(width, height int, header string) string {
	content, _ := m.buildSyncedLyricsContent(width)
	m.lyricsViewport.Width = max(8, width)
	m.lyricsViewport.Height = max(1, height-2)
	m.lyricsViewport.SetContent(content)
	if m.lyricsFollow {
		m.applyLyricsFollowOffset()
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, "", safeViewportView(&m.lyricsViewport))
}

func (m Model) buildSyncedLyricsContent(width int) (string, int) {
	pos := time.Duration(m.playPos * float64(time.Second))
	if m.scrubbing {
		pos = time.Duration(m.scrubPos * float64(time.Second))
	}
	active := lyrics.ActiveLineIndex(m.lyricsLines, pos)

	var sb strings.Builder
	for i, ln := range m.lyricsLines {
		text := ln.Text
		if text == "" {
			text = " "
		}
		style := lipgloss.NewStyle().Foreground(colorSubtext).Width(max(8, width)).MaxWidth(max(8, width))
		if i == active {
			style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Width(max(8, width)).MaxWidth(max(8, width))
		} else if active >= 0 && absInt(i-active) == 1 {
			style = lipgloss.NewStyle().Foreground(colorText).Width(max(8, width)).MaxWidth(max(8, width))
		}
		sb.WriteString(style.Render(text))
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
		content, _ := m.buildSyncedLyricsContent(w)
		prev := m.lyricsViewport.YOffset
		m.lyricsViewport.SetContent(content)
		if m.lyricsFollow {
			m.applyLyricsFollowOffset()
		} else {
			m.lyricsViewport.SetYOffset(prev)
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
	if w >= npMinWide {
		m.lyricsViewport.Width = max(24, w/2-4)
		m.lyricsViewport.Height = max(3, h-2)
	} else {
		m.lyricsViewport.Width = max(8, w-4)
		m.lyricsViewport.Height = max(3, h/2)
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
