package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) generatePlaylistContent(mainWidth int) string {
	if m.playlistPage == nil {
		return "Loading playlist…"
	}
	p := m.playlistPage
	var sb strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(p.Title)
	sb.WriteString(title)
	sb.WriteString("\n")

	metaParts := []string{}
	if author := ytmapi.AuthorName(p.Author); author != "" {
		metaParts = append(metaParts, author)
	}
	if p.TrackCount > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%d tracks", p.TrackCount))
	}
	if p.Duration != "" {
		metaParts = append(metaParts, p.Duration)
	}
	if p.Year != "" {
		metaParts = append(metaParts, p.Year)
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(metaParts, " · ")))
	sb.WriteString("\n\n")

	for i, tr := range p.Tracks {
		if tr.VideoID == "" {
			continue
		}
		num := lipgloss.NewStyle().Foreground(colorSubtext).Width(3).Render(fmt.Sprintf("%d", i+1))
		name := lipgloss.NewStyle().Bold(true).Foreground(colorText).Width(mainWidth / 2).Render(tr.Title)
		artist := lipgloss.NewStyle().Foreground(colorSubtext).Width(mainWidth/4).Render(tr.ArtistName())
		dur := lipgloss.NewStyle().Foreground(colorSubtext).Render(tr.DurationLabel())
		row := lipgloss.JoinHorizontal(lipgloss.Top, num, " ", name, " ", artist, " ", dur)
		row = m.zone.Mark("play_video_"+tr.VideoID, row)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	return sb.String()
}
