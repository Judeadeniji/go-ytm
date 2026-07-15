package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) generateAlbumContent(mainWidth int) string {
	if m.albumPage == nil {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading album…")
	}
	a := m.albumPage
	tracks := playableTracks(a.Tracks)
	var sb strings.Builder

	coverURL := firstThumbURL(a.Thumbnails)
	cover := m.cachedArtAt(coverURL, coverWidth, coverHeight)

	badge := a.Type
	if badge == "" {
		badge = "Album"
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(a.Title)
	badgeStr := lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 1).Render(strings.ToUpper(badge))

	artists := make([]string, 0, len(a.Artists))
	for _, ar := range a.Artists {
		artists = append(artists, ar.Name)
	}
	metaParts := []string{}
	if len(artists) > 0 {
		metaParts = append(metaParts, strings.Join(artists, ", "))
	}
	if a.Year != "" {
		metaParts = append(metaParts, a.Year)
	}
	if a.TrackCount > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%d tracks", a.TrackCount))
	}
	if a.Duration != "" {
		metaParts = append(metaParts, a.Duration)
	}
	meta := lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(metaParts, "  ·  "))
	hints := lipgloss.NewStyle().Foreground(colorDivider).Render("↑/↓ or j/k move  ·  enter play  ·  esc back")

	infoW := mainWidth - coverWidth - 6
	if infoW < 20 {
		infoW = 20
	}
	info := lipgloss.NewStyle().Width(infoW).Render(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", badgeStr),
		"",
		meta,
		"",
		hints,
	))

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cover, "   ", info))
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).Render(strings.Repeat("─", max(10, mainWidth-4))))
	sb.WriteString("\n\n")

	viewsW := tracklistViewsWidth(tracks)
	for i, tr := range tracks {
		sb.WriteString(m.renderTrackRow(i, tr, mainWidth, i == m.trackCursor, viewsW))
		sb.WriteString("\n")
	}

	return sb.String()
}
