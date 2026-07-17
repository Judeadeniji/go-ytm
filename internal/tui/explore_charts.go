package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) renderExploreCharts(w int) string {
	if m.chartsLoading {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading charts...")
	}
	if m.chartsData == nil {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("No charts available")
	}

	var mb strings.Builder
	
	colW := (w - 4) / 2

	var topSection strings.Builder
	var bottomSection strings.Builder

	var artistsCol strings.Builder
	var videosCol strings.Builder

	// ── Top Left: Artists ──
	if len(m.chartsData.Artists) > 0 {
		artistsCol.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1).Render("📈 Top Artists"))
		artistsCol.WriteString("\n")
		for i, artist := range m.chartsData.Artists {
			if i >= 10 { break }
			
			trendColor := colorSubtext
			trendIcon := "━"
			switch artist.Trend {
			case "up":
				trendColor = lipgloss.Color("#4ade80")
				trendIcon = "▲"
			case "down":
				trendColor = colorRed
				trendIcon = "▼"
			}
			
			rank := lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(colorText).Bold(true).Render(artist.Rank)
			trend := lipgloss.NewStyle().Width(2).Foreground(trendColor).Render(trendIcon)
			
			details := lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(artist.Title),
				lipgloss.NewStyle().Foreground(colorSubtext).Render(artist.Subscribers),
			)
			
			thumb := ""
			if len(artist.Thumbnails) > 0 {
				thumb = m.cachedArtAt(artist.Thumbnails[0].URL, 8, 4) + " "
			}
			
			row := lipgloss.JoinHorizontal(lipgloss.Top, rank, " ", trend, " ", thumb, details)
			row = lipgloss.NewStyle().
				Width(colW - 2).
				Padding(1, 0).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(colorDivider).
				Render(row)
				
			if artist.BrowseID != "" {
				row = m.zone.Mark("open_artist_"+artist.BrowseID, row)
			}
			artistsCol.WriteString(row)
			artistsCol.WriteString("\n")
		}
	}

	// ── Top Right: Videos ──
	if len(m.chartsData.Videos) > 0 {
		videosCol.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1).Render("🎬 Top Music Videos"))
		videosCol.WriteString("\n")
		
		var videoRows []string
		var currentVideoRow []string
		cardW := (colW - 4) / 2
		
		for i, v := range m.chartsData.Videos {
			if i >= 6 { break }
			
			thumb := sizedPlaceholder(cardW-2, 6)
			if len(v.Thumbnails) > 0 {
				thumb = m.cachedArtAt(v.Thumbnails[0].URL, cardW-2, 6)
			}
			
			title := v.Title
			if len(title) > 28 { title = title[:25] + "..." }
			
			artist := ""
			if len(v.Artists) > 0 { artist = v.Artists[0].Name }
			
			card := lipgloss.JoinVertical(lipgloss.Left,
				thumb,
				lipgloss.NewStyle().Bold(true).Foreground(colorText).MarginTop(1).Render(title),
				lipgloss.NewStyle().Foreground(colorSubtext).Render(artist),
			)
			card = lipgloss.NewStyle().
				Width(cardW).
				Padding(1).
				Background(colorSearchBg).
				MarginRight(2).
				MarginBottom(1).
				Render(card)
				
			zid := entityZoneID(v.VideoID, v.BrowseID, v.PlaylistID)
			if zid != "" { card = m.zone.Mark(zid, card) }
			
			currentVideoRow = append(currentVideoRow, card)
			if len(currentVideoRow) == 2 {
				videoRows = append(videoRows, lipgloss.JoinHorizontal(lipgloss.Top, currentVideoRow...))
				currentVideoRow = nil
			}
		}
		if len(currentVideoRow) > 0 {
			videoRows = append(videoRows, lipgloss.JoinHorizontal(lipgloss.Top, currentVideoRow...))
		}
		videosCol.WriteString(strings.Join(videoRows, "\n"))
	}

	topSection.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colW).MarginRight(4).Render(artistsCol.String()),
		lipgloss.NewStyle().Width(colW).Render(videosCol.String()),
	))

	// ── Bottom Section: Daily & Weekly Trends ──
	var dailyCol strings.Builder
	var weeklyCol strings.Builder

	renderTrendList := func(title string, items []ytmapi.HomeCarouselItem, col *strings.Builder) {
		if len(items) == 0 { return }
		col.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1).Render(title))
		col.WriteString("\n")
		
		for i, s := range items {
			if i >= 10 { break }
			
			rank := lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(colorText).Bold(true).PaddingTop(1).Render(fmt.Sprintf("%d.", i+1))
			
			thumb := ""
			if len(s.Thumbnails) > 0 {
				thumb = m.cachedArtAt(s.Thumbnails[0].URL, 8, 4) + " "
			}
			
			titleStr := s.Title
			if len(titleStr) > 35 { titleStr = titleStr[:32] + "..." }
			
			artist := ""
			if len(s.Artists) > 0 { artist = s.Artists[0].Name }
			if len(artist) > 35 { artist = artist[:32] + "..." }
			
			info := lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(titleStr),
				lipgloss.NewStyle().Foreground(colorSubtext).Render(artist),
			)
			
			row := lipgloss.JoinHorizontal(lipgloss.Top, rank, " ", thumb, info)
			row = lipgloss.NewStyle().
				Width(colW - 2).
				Padding(1, 0).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(colorDivider).
				Render(row)
				
			zid := entityZoneID(s.VideoID, s.BrowseID, s.PlaylistID)
			if zid != "" { row = m.zone.Mark(zid, row) }
			
			col.WriteString(row)
			col.WriteString("\n")
		}
	}

	renderTrendList("🔥 Daily Top Songs", m.chartsData.Daily, &dailyCol)
	renderTrendList("🌟 Weekly Top Songs", m.chartsData.Weekly, &weeklyCol)

	if dailyCol.Len() > 0 || weeklyCol.Len() > 0 {
		bottomSection.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(colW).MarginRight(4).Render(dailyCol.String()),
			lipgloss.NewStyle().Width(colW).Render(weeklyCol.String()),
		))
	}

	mb.WriteString(topSection.String())
	mb.WriteString("\n\n")
	mb.WriteString(bottomSection.String())

	return mb.String()
}
