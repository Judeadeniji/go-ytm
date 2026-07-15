package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RailTab selects which surface the right sidebar is showing.
// The rail is a multipurpose inspector — tabs keep Queue, Details, and
// future surfaces (Related, Credits, …) from fighting for the same space.
type RailTab int

const (
	RailQueue RailTab = iota
	RailDetails
	railTabCount
)

func (t RailTab) label() string {
	switch t {
	case RailQueue:
		return "Queue"
	case RailDetails:
		return "Details"
	default:
		return "?"
	}
}

// centerBlock left-pads each line so the block is centered in width cells.
// Unlike lipgloss Width/Align, this leaves ANSI sequences intact.
func centerBlock(s string, width int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" || width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for i, line := range lines {
		pad := (width - lipgloss.Width(line)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// padBlock adds left horizontal space without restyling ANSI content.
func padBlock(s string, left int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" || left <= 0 {
		return s
	}
	pad := strings.Repeat(" ", left)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

// generateQueuePanelContent draws the right rail for the active RailTab.
func (m Model) generateQueuePanelContent(width int) string {
	inner := width - 2
	if inner < 8 {
		inner = 8
	}

	var sb strings.Builder
	sb.WriteString(m.renderRailTabs(inner))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
		Render(strings.Repeat("─", max(4, inner))))
	sb.WriteString("\n")

	switch m.railTab {
	case RailDetails:
		sb.WriteString(m.renderRailDetails(inner))
	default:
		sb.WriteString(m.renderRailQueue(inner))
	}
	return sb.String()
}

func (m Model) renderRailTabs(inner int) string {
	tabs := []RailTab{RailQueue, RailDetails}
	parts := make([]string, 0, len(tabs)*2)
	for i, t := range tabs {
		label := " " + t.label() + " "
		style := lipgloss.NewStyle().Foreground(colorSubtext).Background(colorBg)
		if t == m.railTab {
			style = lipgloss.NewStyle().
				Foreground(colorAccent).Bold(true).
				Background(colorBg).
				Underline(true)
		}
		marked := m.zone.Mark(fmt.Sprintf("rail_tab_%d", int(t)), style.Render(label))
		parts = append(parts, marked)
		if i < len(tabs)-1 {
			parts = append(parts, lipgloss.NewStyle().Foreground(colorDivider).Render("│"))
		}
	}
	hide := lipgloss.NewStyle().Foreground(colorSubtext).Render(" \\")
	row := lipgloss.JoinHorizontal(lipgloss.Center, parts...)
	// Right-align the hide hint when there's room.
	pad := inner - lipgloss.Width(row) - lipgloss.Width(hide)
	if pad < 1 {
		pad = 1
	}
	return lipgloss.NewStyle().Padding(1, 1, 0, 1).Render(
		row + strings.Repeat(" ", pad) + hide,
	)
}

func (m Model) renderRailQueue(inner int) string {
	var sb strings.Builder

	// Browse mode keeps the large cover here; NP mode does not (cover is on stage).
	if !m.nowPlayingOpen {
		if m.currentTrack != nil {
			artURL := m.currentTrack.ThumbnailURL
			aw, ah := m.queueArtDims()
			if aw > inner {
				aw = inner
			}
			art := m.cachedArtAt(artURL, aw, ah)
			sb.WriteString(padBlock(centerBlock(art, inner), 1))
			sb.WriteString("\n")

			title := lipgloss.NewStyle().
				Foreground(colorText).Bold(true).
				Width(inner).MaxWidth(inner).
				Render(m.currentTrack.Title)
			artist := lipgloss.NewStyle().
				Foreground(colorSubtext).
				Width(inner).MaxWidth(inner).
				Render(m.currentTrack.Artist)
			sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(title))
			sb.WriteString("\n")
			sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(artist))
			sb.WriteString("\n\n")
			sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
				Render(strings.Repeat("─", max(4, inner))))
			sb.WriteString("\n")
		} else {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(colorSubtext).Padding(1, 1).
				Render("Nothing playing"))
			sb.WriteString("\n")
		}
		sb.WriteString(m.renderQueueSections(inner, true))
		return sb.String()
	}

	// NP mode: queue-only inspector (history + up next).
	pos := m.playPos
	if m.scrubbing {
		pos = m.scrubPos
	}
	state := "Paused"
	if m.isPlaying {
		state = "Playing"
	}
	sb.WriteString(lipgloss.NewStyle().
		Padding(1, 1, 0, 1).
		Foreground(colorSubtext).
		MaxWidth(inner).
		Render(fmt.Sprintf("%s · %s / %s", state, formatClock(pos), formatClock(m.playDuration))))
	sb.WriteString("\n")
	sb.WriteString(m.renderQueueSections(inner, false))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorSubtext).
		MaxWidth(inner).
		Render("] tabs · enter jump · \\ hide"))
	return sb.String()
}

func (m Model) renderRailDetails(inner int) string {
	var sb strings.Builder
	pad := func(s string) string {
		return lipgloss.NewStyle().Padding(0, 1).Width(inner).MaxWidth(inner).Render(s)
	}

	if m.currentTrack == nil {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Padding(1, 1).
			Render("Play something to inspect it"))
		return sb.String()
	}

	t := m.currentTrack
	sb.WriteString("\n")
	sb.WriteString(pad(lipgloss.NewStyle().Bold(true).Foreground(colorText).
		MaxWidth(inner).Render(t.Title)))
	sb.WriteString("\n")

	if t.Artist != "" {
		artist := lipgloss.NewStyle().Foreground(colorAccent).MaxWidth(inner).Render(t.Artist)
		sb.WriteString(m.zone.Mark("rail_meta_artist", pad(artist)))
		sb.WriteString("\n")
		sb.WriteString(pad(lipgloss.NewStyle().Foreground(colorSubtext).
			Render("artist · click to open")))
		sb.WriteString("\n")
	}

	albumName := t.Album
	if albumName != "" {
		album := lipgloss.NewStyle().Foreground(colorAccent).MaxWidth(inner).Render(albumName)
		sb.WriteString(m.zone.Mark("rail_meta_album", pad(album)))
		sb.WriteString("\n")
		hint := "album · click to open"
		if t.AlbumID == "" {
			hint = "album · click to search"
		}
		sb.WriteString(pad(lipgloss.NewStyle().Foreground(colorSubtext).Render(hint)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
		Render(strings.Repeat("─", max(4, inner))))
	sb.WriteString("\n\n")

	pos := m.playPos
	if m.scrubbing {
		pos = m.scrubPos
	}
	state := "Paused"
	if m.isPlaying {
		state = "Playing"
	}
	if !m.audioLoaded {
		state = "Ready"
	}

	sb.WriteString(m.renderMetaRow(inner, "Status", state))
	volLabel := fmt.Sprintf("%d%%", int(m.volume+0.5))
	if m.muted {
		volLabel = "Muted"
	}
	sb.WriteString(m.renderMetaRow(inner, "Volume", volLabel))
	sb.WriteString(m.renderMetaRow(inner, "Position", formatClock(pos)))
	dur := m.effectiveDuration()
	durLabel := formatClock(dur)
	if dur <= 0 && t.Duration != "" {
		durLabel = t.Duration
	}
	sb.WriteString(m.renderMetaRow(inner, "Duration", durLabel))
	if dur > 0 {
		pct := int((pos / dur) * 100)
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		sb.WriteString(m.renderMetaRow(inner, "Progress", fmt.Sprintf("%d%%", pct)))
	}
	if t.IsExplicit {
		sb.WriteString(m.renderMetaRow(inner, "Content", "Explicit"))
	}
	if t.VideoID != "" {
		sb.WriteString(m.renderMetaRow(inner, "Video", t.VideoID))
	}

	switch {
	case m.songDetailsLoading:
		sb.WriteString(m.renderMetaRow(inner, "Meta", "Loading…"))
	case m.songDetailsErr != "":
		sb.WriteString(m.renderMetaRow(inner, "Meta", m.songDetailsErr))
	case m.songDetails != nil:
		sd := m.songDetails
		if typ := songTypeLabel(sd.MusicVideoType); typ != "" {
			sb.WriteString(m.renderMetaRow(inner, "Type", typ))
		}
		if sd.ViewCount != "" {
			sb.WriteString(m.renderMetaRow(inner, "Views", formatViewCount(sd.ViewCount)))
		}
		if sd.PublishDate != "" {
			sb.WriteString(m.renderMetaRow(inner, "Published", sd.PublishDate))
		}
		if sd.Category != "" {
			sb.WriteString(m.renderMetaRow(inner, "Category", sd.Category))
		}
		if sd.ChannelID != "" {
			sb.WriteString(m.renderMetaRow(inner, "Channel", sd.ChannelID))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
		Render(strings.Repeat("─", max(4, inner))))
	sb.WriteString("\n\n")

	sb.WriteString(m.renderMetaRow(inner, "Lyrics", m.lyricsStatusLabel()))
	refresh := lipgloss.NewStyle().Foreground(colorSubtext).Render("↻ refresh")
	sb.WriteString(m.zone.Mark("rail_meta_lyrics_refresh", pad(refresh)))
	sb.WriteString("\n")

	cur := m.queue.CurrentIndex()
	upcoming, history := 0, 0
	if cur >= 0 {
		history = cur
		upcoming = m.queue.Len() - cur - 1
		if upcoming < 0 {
			upcoming = 0
		}
	} else {
		upcoming = m.queue.Len()
	}
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
		Render(strings.Repeat("─", max(4, inner))))
	sb.WriteString("\n\n")
	sb.WriteString(m.renderMetaRow(inner, "Queue", fmt.Sprintf("%d total", m.queue.Len())))
	sb.WriteString(m.renderMetaRow(inner, "Up next", fmt.Sprintf("%d", upcoming)))
	sb.WriteString(m.renderMetaRow(inner, "History", fmt.Sprintf("%d", history)))

	if upcoming > 0 {
		sb.WriteString("\n")
		clear := lipgloss.NewStyle().Foreground(colorSubtext).Render("clear up next")
		sb.WriteString(m.zone.Mark("rail_meta_clear_upcoming", pad(clear)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(pad(lipgloss.NewStyle().Foreground(colorSubtext).
		Render("] next tab · [ prev · \\ hide")))
	return sb.String()
}

func (m Model) renderMetaRow(inner int, key, val string) string {
	k := lipgloss.NewStyle().Foreground(colorSubtext).Width(10).Render(key)
	v := lipgloss.NewStyle().Foreground(colorText).MaxWidth(inner - 12).Render(val)
	row := lipgloss.JoinHorizontal(lipgloss.Top, k, " ", v)
	return lipgloss.NewStyle().Padding(0, 1).Width(inner).MaxWidth(inner).Render(row) + "\n"
}

func (m Model) lyricsStatusLabel() string {
	switch {
	case m.currentTrack == nil:
		return "—"
	case m.lyricsLoading:
		return "Searching…"
	case m.lyricsInstrumental:
		return "Instrumental"
	case len(m.lyricsLines) > 0:
		return fmt.Sprintf("Synced · %d lines", len(m.lyricsLines))
	case m.lyricsPlain != "":
		return "Plain text"
	case m.lyricsErr != "":
		return "Not found"
	default:
		return "—"
	}
}

func songTypeLabel(musicVideoType string) string {
	switch musicVideoType {
	case "MUSIC_VIDEO_TYPE_ATV":
		return "Song"
	case "MUSIC_VIDEO_TYPE_OMV":
		return "Official video"
	case "MUSIC_VIDEO_TYPE_UGC":
		return "User upload"
	case "":
		return ""
	default:
		return musicVideoType
	}
}

func formatViewCount(s string) string {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return s
		}
		n = n*10 + int64(c-'0')
	}
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// renderQueueSections writes history + up-next. If includeCurrent, the playing
// track is listed under Up next (browse mode); otherwise only upcoming tracks.
func (m Model) renderQueueSections(inner int, includeCurrent bool) string {
	var sb strings.Builder
	tracks := m.queue.Tracks()
	cur := m.queue.CurrentIndex()

	if len(tracks) == 0 {
		sb.WriteString(lipgloss.NewStyle().
			Padding(1, 1, 0, 1).
			Foreground(colorSubtext).
			Render("Queue empty"))
		return sb.String()
	}

	upcomingStart := cur
	if !includeCurrent {
		upcomingStart = cur + 1
	}
	if upcomingStart < 0 {
		upcomingStart = 0
	}

	hasPlayed := cur > 0
	if hasPlayed {
		histLabel := fmt.Sprintf("History · %d", cur)
		sb.WriteString(lipgloss.NewStyle().
			Padding(1, 1, 0, 1).
			Foreground(colorSubtext).
			Bold(true).
			Render(histLabel))
		sb.WriteString("\n\n")
		for i := 0; i < cur; i++ {
			sb.WriteString(m.renderQueueListItem(i, tracks[i], inner, false))
			sb.WriteString("\n")
		}
		sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
			Render(strings.Repeat("─", max(4, inner))))
		sb.WriteString("\n")
	}

	upcoming := 0
	if upcomingStart < len(tracks) {
		upcoming = len(tracks) - upcomingStart
	}
	upLabel := "Up next"
	if upcoming > 0 {
		upLabel = fmt.Sprintf("Up next · %d", upcoming)
	} else if includeCurrent && cur >= 0 {
		upLabel = "Playing"
	}
	sb.WriteString(lipgloss.NewStyle().
		Padding(1, 1, 0, 1).
		Foreground(colorText).
		Bold(true).
		Render(upLabel))
	sb.WriteString("\n\n")

	if upcomingStart >= len(tracks) {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 1).
			Render("Nothing up next"))
		return sb.String()
	}

	for i := upcomingStart; i < len(tracks); i++ {
		playing := includeCurrent && cur >= 0 && i == cur
		sb.WriteString(m.renderQueueListItem(i, tracks[i], inner, playing))
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderQueueListItem draws one unordered queue row (• / › / ▶).
func (m Model) renderQueueListItem(i int, tr Track, inner int, playing bool) string {
	focused := m.activePane == PaneQueue && m.railTab == RailQueue && m.queueCursor == i

	bullet := "• "
	titleColor := colorText
	artistColor := colorSubtext
	bg := colorBg
	if playing {
		bullet = "▶ "
		titleColor = colorAccent
	}
	if focused {
		bg = colorFocusBg
		bullet = "› "
		if !playing {
			titleColor = colorAccent
		}
	}
	if !playing && m.queue.CurrentIndex() >= 0 && i < m.queue.CurrentIndex() {
		titleColor = colorSubtext
	}

	lineBudget := inner - lipgloss.Width(bullet) - 1
	if lineBudget < 6 {
		lineBudget = 6
	}
	title := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(focused || playing).
		Background(bg).
		MaxWidth(lineBudget).
		Render(tr.Title)
	row1 := bullet + title

	artistBudget := inner - 2
	artist := lipgloss.NewStyle().
		Foreground(artistColor).
		Background(bg).
		MaxWidth(artistBudget).
		Render("  " + tr.Artist)

	block := lipgloss.JoinVertical(lipgloss.Left, row1, artist)
	block = lipgloss.NewStyle().
		Background(bg).
		Width(inner).
		MaxWidth(inner).
		Padding(0, 1).
		Render(block)

	return m.zone.Mark(fmt.Sprintf("queue_track_%d", i), block)
}

func (m Model) cycleRailTab(delta int) (Model, tea.Cmd) {
	n := int(railTabCount)
	if n <= 0 {
		return m, nil
	}
	next := (int(m.railTab) + delta) % n
	if next < 0 {
		next += n
	}
	m.railTab = RailTab(next)
	m.setQueuePanelContent()
	var cmd tea.Cmd
	if m.railTab == RailDetails {
		cmd = m.ensureSongDetailsFetched()
	}
	return m, cmd
}

// handleRailPanelClick handles tab switches and Details inspector actions.
func (m Model) handleRailPanelClick(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if msg.Type != tea.MouseLeft || !m.showQueuePanel() {
		return m, nil, false
	}

	for t := RailQueue; t < railTabCount; t++ {
		if m.zone.Get(fmt.Sprintf("rail_tab_%d", int(t))).InBounds(msg) {
			m.railTab = t
			m.activePane = PaneQueue
			m.setQueuePanelContent()
			var cmd tea.Cmd
			if m.railTab == RailDetails {
				cmd = m.ensureSongDetailsFetched()
			}
			return m, cmd, true
		}
	}

	if m.zone.Get("rail_meta_artist").InBounds(msg) {
		if m.currentTrack == nil || m.currentTrack.Artist == "" {
			return m, nil, true
		}
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		if id := m.currentTrack.ArtistID; id != "" {
			m.searchInput.Blur()
			m.statusMsg = "Opening artist…"
			m.markSessionDirty()
			mm, cmd := m.openArtist(id)
			return mm, cmd, true
		}
		artist := m.currentTrack.Artist
		m.searchInput.Blur()
		m.searchInput.SetValue(artist)
		m.lastSearchQuery = artist
		m.searchFilter = "artists"
		m.statusMsg = "Searching artists: " + artist
		m.markSessionDirty()
		m.navGen++
		return m, doSearchFiltered(m.ytmapiClient, artist, "artists", m.navGen), true
	}
	if m.zone.Get("rail_meta_album").InBounds(msg) {
		if m.currentTrack == nil || m.currentTrack.Album == "" {
			return m, nil, true
		}
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		if id := m.currentTrack.AlbumID; id != "" {
			m.searchInput.Blur()
			m.statusMsg = "Opening album…"
			m.markSessionDirty()
			mm, cmd := m.openAlbum(id)
			return mm, cmd, true
		}
		album := m.currentTrack.Album
		m.searchInput.Blur()
		m.searchInput.SetValue(album)
		m.lastSearchQuery = album
		m.searchFilter = "albums"
		m.statusMsg = "Searching albums: " + album
		m.markSessionDirty()
		m.navGen++
		return m, doSearchFiltered(m.ytmapiClient, album, "albums", m.navGen), true
	}
	if m.zone.Get("rail_meta_lyrics_refresh").InBounds(msg) {
		if m.currentTrack == nil {
			return m, nil, true
		}
		m.lyricsTrackKey = ""
		m.lyricsFetchDur = 0
		m.lyricsLines = nil
		m.lyricsPlain = ""
		m.lyricsErr = ""
		m.lyricsInstrumental = false
		cmd := m.ensureLyricsFetched()
		m.setQueuePanelContent()
		return m, cmd, true
	}
	if m.zone.Get("rail_meta_clear_upcoming").InBounds(msg) {
		m.queue.TruncateAfterCurrent()
		m.markSessionDirty()
		m.setQueuePanelContent()
		return m, nil, true
	}
	return m, nil, false
}
