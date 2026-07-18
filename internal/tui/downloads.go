package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/library"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// dlProgressEntry tracks bytes transferred for an in-flight download.
type dlProgressEntry struct {
	Track library.CachedTrack
	Bytes int64
	Total int64
}

// downloadsFocusKind identifies which section a focus index maps to.
type downloadsFocusKind int

const (
	dlFocusActive downloadsFocusKind = iota
	dlFocusPlaylist
	dlFocusAlbum
	dlFocusEP
	dlFocusSingle
	dlFocusSong
)

type downloadsFocusItem struct {
	Kind  downloadsFocusKind
	Index int // index within that section's slice
}

func normalizeOfflineAlbumKind(albumType string) string {
	switch strings.ToLower(strings.TrimSpace(albumType)) {
	case "ep":
		return "ep"
	case "single":
		return "single"
	default:
		return "album"
	}
}

func isOfflineAlbumKind(kind string) bool {
	switch kind {
	case "album", "ep", "single":
		return true
	default:
		return false
	}
}

func offlineKindBadge(kind string) string {
	switch kind {
	case "playlist":
		return "Playlist"
	case "ep":
		return "EP"
	case "single":
		return "Single"
	default:
		return "Album"
	}
}

func playlistAuthorName(author any) string {
	if aStr, ok := author.(string); ok {
		return aStr
	}
	if aMap, ok := author.(map[string]any); ok {
		if n, ok := aMap["name"].(string); ok {
			return n
		}
	}
	if aList, ok := author.([]any); ok && len(aList) > 0 {
		if am, ok := aList[0].(map[string]any); ok {
			if n, ok := am["name"].(string); ok {
				return n
			}
		}
	}
	return ""
}

func formatByteSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (m *Model) upsertOfflineCollection(col library.OfflineCollection) {
	for i, c := range m.libOfflineCollections {
		if c.ID == col.ID {
			m.libOfflineCollections[i] = col
			return
		}
	}
	m.libOfflineCollections = append([]library.OfflineCollection{col}, m.libOfflineCollections...)
}

func (m *Model) noteDownloadStarted(track library.CachedTrack) {
	if m.dlProgress == nil {
		m.dlProgress = make(map[string]dlProgressEntry)
	}
	if _, ok := m.dlProgress[track.VideoID]; !ok {
		m.dlProgress[track.VideoID] = dlProgressEntry{Track: track}
	}
}

func (m *Model) pruneStaleDownloadProgress() {
	if m.downloadMgr == nil || m.dlProgress == nil {
		return
	}
	active := make(map[string]struct{})
	for _, id := range m.downloadMgr.ActiveIDs() {
		active[id] = struct{}{}
	}
	for id := range m.dlProgress {
		if _, ok := active[id]; !ok {
			delete(m.dlProgress, id)
		}
	}
}

func (m Model) activeDownloadEntries() []dlProgressEntry {
	if len(m.dlProgress) == 0 {
		return nil
	}
	out := make([]dlProgressEntry, 0, len(m.dlProgress))
	for _, e := range m.dlProgress {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Track.Title < out[j].Track.Title
	})
	return out
}

func (m Model) offlineCollectionsByKind(kind string) []library.OfflineCollection {
	var out []library.OfflineCollection
	for _, c := range m.libOfflineCollections {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) collectionCompleteness(col library.OfflineCollection) (have, total int) {
	total = len(col.TrackIDs)
	if total == 0 {
		return 0, 0
	}
	haveSet := make(map[string]struct{}, len(m.libDownloads))
	for _, d := range m.libDownloads {
		haveSet[d.VideoID] = struct{}{}
	}
	for _, id := range col.TrackIDs {
		if _, ok := haveSet[id]; ok {
			have++
		}
	}
	return have, total
}

func (m Model) downloadsFocusItems() []downloadsFocusItem {
	var items []downloadsFocusItem

	sub := m.downloadsSubTab
	if sub == "" {
		sub = "playlists"
	}

	switch sub {
	case "active":
		for i := range m.activeDownloadEntries() {
			items = append(items, downloadsFocusItem{Kind: dlFocusActive, Index: i})
		}
	case "songs":
		for i := range m.libDownloads {
			items = append(items, downloadsFocusItem{Kind: dlFocusSong, Index: i})
		}
	case "albums":
		for i := range m.offlineCollectionsByKind("album") {
			items = append(items, downloadsFocusItem{Kind: dlFocusAlbum, Index: i})
		}
		for i := range m.offlineCollectionsByKind("ep") {
			items = append(items, downloadsFocusItem{Kind: dlFocusEP, Index: i})
		}
		for i := range m.offlineCollectionsByKind("single") {
			items = append(items, downloadsFocusItem{Kind: dlFocusSingle, Index: i})
		}
	default: // playlists
		for i := range m.offlineCollectionsByKind("playlist") {
			items = append(items, downloadsFocusItem{Kind: dlFocusPlaylist, Index: i})
		}
	}
	return items
}

func (m Model) downloadsFocusAt(cursor int) (downloadsFocusItem, bool) {
	items := m.downloadsFocusItems()
	if cursor < 0 || cursor >= len(items) {
		return downloadsFocusItem{}, false
	}
	return items[cursor], true
}

func (m Model) downloadsCollectionCards(kind string) []ytmapi.HomeCarouselItem {
	cols := m.offlineCollectionsByKind(kind)
	cards := make([]ytmapi.HomeCarouselItem, 0, len(cols))
	for _, col := range cols {
		thumb := col.ThumbnailURL
		if thumb == "" {
			thumb = m.offlineCollectionThumbFallback(col)
		}
		have, total := m.collectionCompleteness(col)
		sub := col.Author
		if sub != "" {
			sub = fmt.Sprintf("%s · %d/%d", sub, have, total)
		} else {
			sub = fmt.Sprintf("%s · %d/%d", offlineKindBadge(col.Kind), have, total)
		}
		card := ytmapi.HomeCarouselItem{
			Title:       col.Title,
			Description: sub,
		}
		if thumb != "" {
			card.Thumbnails = []ytmapi.Thumbnail{{URL: thumb}}
		}
		if col.Kind == "playlist" {
			card.ZoneID = "open_playlist_" + col.ID
			card.PlaylistID = col.ID
		} else {
			card.ZoneID = "open_album_" + col.ID
			card.BrowseID = col.ID
		}
		cards = append(cards, card)
	}
	return cards
}

func (m Model) offlineCollectionThumbFallback(col library.OfflineCollection) string {
	if col.Kind == "playlist" {
		if cached, ok := m.pageCache["playlist_"+col.ID].(*ytmapi.PlaylistPage); ok && cached != nil {
			return firstThumbURL(cached.Thumbnails)
		}
		return ""
	}
	if cached, ok := m.pageCache["album_"+col.ID].(*ytmapi.AlbumPage); ok && cached != nil {
		return firstThumbURL(cached.Thumbnails)
	}
	return ""
}

func (m *Model) backfillOfflineCollectionThumb(id, thumbURL string) {
	if id == "" || thumbURL == "" {
		return
	}
	for i, c := range m.libOfflineCollections {
		if c.ID != id || c.ThumbnailURL != "" {
			continue
		}
		m.libOfflineCollections[i].ThumbnailURL = thumbURL
		if m.sessionStore != nil {
			_ = m.sessionStore.SaveOfflineCollection(m.libOfflineCollections[i])
		}
		return
	}
}

// generateDownloadsContent renders the Downloads library tab body (under chips).
func (m Model) generateDownloadsContent(mainWidth int) string {
	var mb strings.Builder

	active := m.activeDownloadEntries()
	sub := m.downloadsSubTab
	if sub == "" {
		sub = "playlists"
	}

	// Sub-chips: Playlists | Albums | Songs | Active
	type subChip struct{ label, value string }
	activeLabel := "Active"
	if n := len(active); n > 0 {
		activeLabel = fmt.Sprintf("Active (%d)", n)
	}
	subs := []subChip{
		{"Playlists", "playlists"},
		{"Albums", "albums"},
		{"Songs", "songs"},
		{activeLabel, "active"},
	}
	var chips []string
	for _, c := range subs {
		s := lipgloss.NewStyle().Padding(0, 2).Foreground(colorSubtext)
		if sub == c.value {
			s = s.Foreground(colorText).Bold(true).
				BorderBottom(true).BorderForeground(colorAccent)
		} else if c.value == "active" && len(active) > 0 {
			s = s.Foreground(colorAccent)
		}
		chips = append(chips, m.zone.Mark("dl_sub_"+c.value, s.Render(c.label)))
	}
	mb.WriteString(strings.Join(chips, "  "))
	mb.WriteString("\n\n")

	nPlaylists := len(m.offlineCollectionsByKind("playlist"))
	nAlbums := len(m.offlineCollectionsByKind("album")) +
		len(m.offlineCollectionsByKind("ep")) +
		len(m.offlineCollectionsByKind("single"))
	emptySongs := len(m.libDownloads) == 0
	emptyActive := len(active) == 0

	if nPlaylists == 0 && nAlbums == 0 && emptySongs && emptyActive {
		mb.WriteString(m.renderDownloadsEmpty())
		return mb.String()
	}

	mb.WriteString(m.renderDownloadsSummary(len(active)))
	mb.WriteString("\n\n")

	focusIdx := 0

	switch sub {
	case "active":
		if emptyActive {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No downloads in progress."))
			mb.WriteString("\n")
			break
		}
		for _, e := range active {
			focused := m.activePane == PaneMain && m.listCursor == focusIdx
			mb.WriteString(m.renderActiveDownloadRow(e, mainWidth, focused))
			mb.WriteString("\n")
			focusIdx++
		}

	case "songs":
		if emptySongs {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No downloaded songs."))
			mb.WriteString("\n")
			break
		}
		sizeW := downloadsSizeWidth(m.libDownloads)
		for i, t := range m.libDownloads {
			focused := m.activePane == PaneMain && m.listCursor == focusIdx
			mb.WriteString(m.renderDownloadedSongRow(i, t, mainWidth, focused, sizeW))
			mb.WriteString("\n")
			focusIdx++
		}

	case "albums":
		if nAlbums == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No downloaded albums."))
			mb.WriteString("\n")
			break
		}
		type albumSection struct {
			title string
			kind  string
			cards []ytmapi.HomeCarouselItem
		}
		var sections []albumSection
		for _, s := range []struct{ title, kind string }{
			{"Albums", "album"},
			{"EPs", "ep"},
			{"Singles", "single"},
		} {
			cards := m.downloadsCollectionCards(s.kind)
			if len(cards) == 0 {
				continue
			}
			sections = append(sections, albumSection{title: s.title, kind: s.kind, cards: cards})
		}
		showTitles := len(sections) > 1
		for _, s := range sections {
			title := ""
			if showTitles {
				title = s.title
			}
			mb.WriteString(m.renderGridAt(title, s.cards, mainWidth, focusIdx))
			focusIdx += len(s.cards)
		}

	default: // playlists
		if nPlaylists == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No downloaded playlists."))
			mb.WriteString("\n")
			break
		}
		cards := m.downloadsCollectionCards("playlist")
		mb.WriteString(m.renderGridAt("", cards, mainWidth, focusIdx))
	}

	return mb.String()
}

func downloadsSectionHeader(title string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(title) + "\n"
}

func (m Model) renderDownloadsSummary(activeCount int) string {
	var parts []string
	n := len(m.libDownloads)
	parts = append(parts, fmt.Sprintf("%d track%s", n, pluralS(n)))

	var totalBytes int64
	for _, t := range m.libDownloads {
		totalBytes += t.Bytes
	}
	if totalBytes > 0 || n > 0 {
		parts = append(parts, formatByteSize(totalBytes))
	}

	ncPlaylists := 0
	ncAlbums := 0
	for _, c := range m.libOfflineCollections {
		if c.Kind == "playlist" {
			ncPlaylists++
		} else {
			ncAlbums++
		}
	}
	if ncPlaylists > 0 {
		parts = append(parts, fmt.Sprintf("%d playlist%s", ncPlaylists, pluralS(ncPlaylists)))
	}
	if ncAlbums > 0 {
		parts = append(parts, fmt.Sprintf("%d album%s", ncAlbums, pluralS(ncAlbums)))
	}
	if activeCount > 0 {
		parts = append(parts, fmt.Sprintf("%d downloading", activeCount))
	}

	return lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(parts, " · "))
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) renderDownloadsEmpty() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("No downloads yet")
	hint := lipgloss.NewStyle().Foreground(colorSubtext).Render(
		"d  download focused track or collection\nD  download all tracks on this page")
	return title + "\n\n" + hint + "\n"
}

func (m Model) renderActiveDownloadRow(e dlProgressEntry, mainWidth int, focused bool) string {
	bg := colorBg
	if focused {
		bg = colorFocusBg
	}

	ind := "  "
	indStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(2)
	if focused {
		ind = "›"
		indStyle = indStyle.Foreground(colorAccent)
	}

	pct := 0
	if e.Total > 0 {
		pct = int(float64(e.Bytes) / float64(e.Total) * 100)
		if pct > 100 {
			pct = 100
		}
	}

	barW := 16
	if mainWidth < 60 {
		barW = 10
	}
	filled := 0
	if e.Total > 0 {
		filled = barW * pct / 100
	} else {
		filled = int(e.Bytes/65536) % (barW + 1)
		if filled > barW {
			filled = barW
		}
	}
	barFill := lipgloss.NewStyle().Foreground(colorAccent).Background(bg).Render(strings.Repeat("█", filled))
	barEmpty := lipgloss.NewStyle().Foreground(colorBuffer).Background(bg).Render(strings.Repeat("░", barW-filled))
	bar := barFill + barEmpty

	pctLabel := "…"
	if e.Total > 0 {
		pctLabel = fmt.Sprintf("%3d%%", pct)
	}
	pctStr := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(4).Align(lipgloss.Right).Render(pctLabel)

	textBudget := mainWidth - 4 - 2 - barW - 4 - 6
	if textBudget < 16 {
		textBudget = 16
	}
	titleW := textBudget * 3 / 5
	artistW := textBudget - titleW

	titleColor := colorText
	if focused {
		titleColor = colorAccent
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Width(titleW).MaxWidth(titleW).Render(e.Track.Title)
	artist := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(artistW).MaxWidth(artistW).Render(e.Track.Artist)

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		indStyle.Render(ind), " ", title, " ", artist, " ", bar, " ", pctStr,
	)
	row = lipgloss.NewStyle().Background(bg).Width(mainWidth - 2).MaxWidth(mainWidth - 2).Render(row)
	if e.Track.VideoID != "" {
		row = m.zone.Mark("dl_active_"+e.Track.VideoID, row)
	}
	return row
}

func downloadsSizeWidth(tracks []library.CachedTrack) int {
	w := 4
	for _, t := range tracks {
		s := formatByteSize(t.Bytes)
		if len(s) > w {
			w = len(s)
		}
	}
	if w > 10 {
		w = 10
	}
	return w
}

func (m Model) renderDownloadedSongRow(i int, t library.CachedTrack, mainWidth int, focused bool, sizeW int) string {
	playing := m.currentTrack != nil && m.currentTrack.VideoID != "" && m.currentTrack.VideoID == t.VideoID

	bg := colorBg
	switch {
	case focused:
		bg = colorFocusBg
	case playing:
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
	textBudget := mainWidth - 4 - 2 - numW - durW - sizeW - 4
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
	} else if focused {
		titleColor = colorAccent
	}

	num := lipgloss.NewStyle().Foreground(numColor).Background(bg).Width(numW).Render(fmt.Sprintf("%d", i+1))
	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Width(titleW).MaxWidth(titleW).Render(t.Title)
	artist := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(artistW).MaxWidth(artistW).Render(t.Artist)
	dur := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(durW).Align(lipgloss.Right).Render(t.Duration)
	size := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(sizeW).Align(lipgloss.Right).Render(formatByteSize(t.Bytes))

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		indStyle.Render(indicator), " ", num, title, " ", artist, " ", size, " ", dur,
	)
	row = lipgloss.NewStyle().Background(bg).Width(mainWidth - 2).MaxWidth(mainWidth - 2).Render(row)
	if t.VideoID != "" {
		row = m.zone.Mark("play_video_"+t.VideoID, row)
	}
	return row
}
