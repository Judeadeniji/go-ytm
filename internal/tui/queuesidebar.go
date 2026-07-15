package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
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
	switch {
	case m.audioLoading():
		state = "Loading"
	case m.isPlaying:
		state = "Playing"
	case m.audioPending():
		state = "Ready"
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
	sd := m.songDetails

	title := t.Title
	if sd != nil && sd.Title != "" {
		title = sd.Title
	}
	sb.WriteString("\n")
	sb.WriteString(pad(lipgloss.NewStyle().Bold(true).Foreground(colorText).
		MaxWidth(inner).Render(title)))
	sb.WriteString("\n")

	artistName := t.Artist
	if sd != nil {
		if names := sd.ArtistNames(); names != "" {
			artistName = names
		}
	}
	if artistName != "" {
		artist := lipgloss.NewStyle().Foreground(colorAccent).MaxWidth(inner).Render(artistName)
		sb.WriteString(m.zone.Mark("rail_meta_artist", pad(artist)))
		sb.WriteString("\n")
	}

	albumName := t.Album
	if sd != nil && sd.Album != nil && sd.Album.Name != "" {
		albumName = sd.Album.Name
	}
	if albumName != "" {
		album := lipgloss.NewStyle().Foreground(colorAccent).MaxWidth(inner).Render(albumName)
		sb.WriteString(m.zone.Mark("rail_meta_album", pad(album)))
		sb.WriteString("\n")
		viewAlbum := lipgloss.NewStyle().Foreground(colorSubtext).Render("View Album")
		sb.WriteString(m.zone.Mark("rail_meta_view_album", pad(viewAlbum)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorDivider).
		Render(strings.Repeat("─", max(4, inner))))
	sb.WriteString("\n\n")

	switch {
	case m.songDetailsLoading:
		sb.WriteString(pad(lipgloss.NewStyle().Foreground(colorSubtext).Render("Loading song metadata…")))
		sb.WriteString("\n\n")
	case m.songDetailsErr != "":
		sb.WriteString(pad(lipgloss.NewStyle().Foreground(colorSubtext).Render(m.songDetailsErr)))
		sb.WriteString("\n\n")
	case sd != nil:
		if sd.AlbumType != "" {
			sb.WriteString(m.renderMetaRow(inner, "Release", sd.AlbumType))
		}
		if sd.Year != "" {
			sb.WriteString(m.renderMetaRow(inner, "Year", sd.Year))
		}
		if sd.TrackNumber != nil {
			track := fmt.Sprintf("%d", *sd.TrackNumber)
			if sd.AlbumTrackCount > 0 {
				track = fmt.Sprintf("%d of %d", *sd.TrackNumber, sd.AlbumTrackCount)
			}
			sb.WriteString(m.renderMetaRow(inner, "Track", track))
		}
		dur := sd.Duration
		if dur == "" && t.Duration != "" {
			dur = t.Duration
		}
		if dur != "" {
			sb.WriteString(m.renderMetaRow(inner, "Length", dur))
		}
		if sd.IsExplicit || t.IsExplicit {
			sb.WriteString(m.renderMetaRow(inner, "Content", "Explicit"))
		}

		if sd.Credits != nil {
			sb.WriteString("\n")
			sb.WriteString(pad(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Credits")))
			sb.WriteString("\n")
			sb.WriteString(m.renderCreditSection(inner, sd.Credits.PerformedBy))
			sb.WriteString(m.renderCreditSection(inner, sd.Credits.WrittenBy))
			sb.WriteString(m.renderCreditSection(inner, sd.Credits.ProducedBy))
			for i := range sd.Credits.OtherSections {
				sb.WriteString(m.renderCreditSection(inner, &sd.Credits.OtherSections[i]))
			}
		}
		sb.WriteString("\n")
	default:
		if t.Duration != "" {
			sb.WriteString(m.renderMetaRow(inner, "Length", t.Duration))
		}
		if t.IsExplicit {
			sb.WriteString(m.renderMetaRow(inner, "Content", "Explicit"))
		}
		sb.WriteString("\n")
	}

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

func (m Model) renderCreditSection(inner int, sec *ytmapi.CreditSection) string {
	if sec == nil || len(sec.Data) == 0 {
		return ""
	}
	title := sec.LocalizedTitle
	if title == "" {
		title = "Credits"
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorSubtext).
		Width(inner).MaxWidth(inner).Render(title))
	sb.WriteString("\n")
	names := strings.Join(sec.Data, ", ")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).
		Width(inner).MaxWidth(inner).Render(names))
	sb.WriteString("\n")
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
		if m.currentTrack == nil {
			return m, nil, true
		}
		artist := m.currentTrack.Artist
		artistID := m.currentTrack.ArtistID
		if m.songDetails != nil {
			if names := m.songDetails.ArtistNames(); names != "" {
				artist = names
			}
			if len(m.songDetails.Artists) > 0 && m.songDetails.Artists[0].ID != "" {
				artistID = m.songDetails.Artists[0].ID
			}
		}
		if artist == "" {
			return m, nil, true
		}
		if m.nowPlayingOpen {
			m = m.closeNowPlaying()
		}
		if artistID != "" {
			m.searchInput.Blur()
			m.statusMsg = "Opening artist…"
			m.markSessionDirty()
			mm, cmd := m.openArtist(artistID)
			return mm, cmd, true
		}
		m.searchInput.Blur()
		m.searchInput.SetValue(artist)
		m.lastSearchQuery = artist
		m.searchFilter = "artists"
		m.statusMsg = "Searching artists: " + artist
		m.markSessionDirty()
		m.cancelNavFetch()
		m.navGen++
		m.pageLoading = true
		ctx := m.startNavCtx()
		m.setMainContent()
		return m, doSearchFiltered(m.ytmapiClient, artist, "artists", m.navGen, ctx), true
	}
	if m.zone.Get("rail_meta_album").InBounds(msg) || m.zone.Get("rail_meta_view_album").InBounds(msg) {
		mm, cmd := m.goToPlayingAlbum()
		return mm, cmd, true
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
