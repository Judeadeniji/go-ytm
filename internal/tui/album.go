package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) generateAlbumContent(mainWidth int) string {
	if m.albumPage == nil {
		return "Loading album…"
	}
	a := m.albumPage
	var sb strings.Builder

	badge := a.Type
	if badge == "" {
		badge = "Album"
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(a.Title)
	badgeStr := lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 1).Render(badge)

	artists := make([]string, 0, len(a.Artists))
	for _, ar := range a.Artists {
		artists = append(artists, ar.Name)
	}
	meta := strings.Join(artists, ", ")
	if a.Year != "" {
		if meta != "" {
			meta += " · "
		}
		meta += a.Year
	}
	if a.TrackCount > 0 {
		if meta != "" {
			meta += " · "
		}
		meta += fmt.Sprintf("%d tracks", a.TrackCount)
	}
	if a.Duration != "" {
		if meta != "" {
			meta += " · "
		}
		meta += a.Duration
	}

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", badgeStr))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render(meta))
	sb.WriteString("\n\n")

	for i, tr := range a.Tracks {
		num := lipgloss.NewStyle().Foreground(colorSubtext).Width(3).Render(fmt.Sprintf("%d", i+1))
		name := lipgloss.NewStyle().Bold(true).Foreground(colorText).Width(mainWidth - 16).Render(tr.Title)
		dur := lipgloss.NewStyle().Foreground(colorSubtext).Render(tr.DurationLabel())
		row := lipgloss.JoinHorizontal(lipgloss.Top, num, " ", name, " ", dur)
		if tr.VideoID != "" {
			row = m.zone.Mark("play_video_"+tr.VideoID, row)
		}
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	return sb.String()
}
