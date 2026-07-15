package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) generatePlaylistContent(mainWidth int) string {
	if m.playlistPage == nil {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading playlist…")
	}
	p := m.playlistPage
	tracks := playableTracks(p.Tracks)
	var sb strings.Builder

	// —— Header: cover + meta ——
	coverURL := firstThumbURL(p.Thumbnails)
	cover := m.cachedArtAt(coverURL, coverWidth, coverHeight)

	title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(p.Title)
	badge := lipgloss.NewStyle().
		Background(colorSearchBg).Foreground(colorSubtext).
		Padding(0, 1).Render("PLAYLIST")

	metaParts := []string{}
	if author := ytmapi.AuthorName(p.Author); author != "" {
		metaParts = append(metaParts, author)
	}
	if p.TrackCount > 0 {
		metaParts = append(metaParts, pluralCount(p.TrackCount, "song", "songs"))
	} else if len(tracks) > 0 {
		metaParts = append(metaParts, pluralCount(len(tracks), "song", "songs"))
	}
	if p.Duration != "" {
		metaParts = append(metaParts, p.Duration)
	}
	if views := ytmapi.FormatCountAny(p.Views); views != "" {
		metaParts = append(metaParts, views+" plays")
	}
	if p.Year != "" {
		metaParts = append(metaParts, p.Year)
	}
	meta := lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(metaParts, "  ·  "))

	hints := lipgloss.NewStyle().Foreground(colorDivider).Render("↑/↓ or j/k move  ·  enter play  ·  esc back")

	infoW := mainWidth - coverWidth - 6
	if infoW < 20 {
		infoW = 20
	}
	info := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", badge),
		"",
		meta,
		"",
		hints,
	)
	info = lipgloss.NewStyle().Width(infoW).Render(info)

	header := lipgloss.JoinHorizontal(lipgloss.Top, cover, "   ", info)
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).Render(strings.Repeat("─", max(10, mainWidth-4))))
	sb.WriteString("\n\n")

	if len(tracks) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No playable tracks."))
		return sb.String()
	}

	viewsW := tracklistViewsWidth(tracks)
	for i, tr := range tracks {
		focused := i == m.trackCursor
		sb.WriteString(m.renderTrackRow(i, tr, mainWidth, focused, viewsW))
		sb.WriteString("\n")
	}

	return sb.String()
}
