package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// generateExploreContent renders the explore page with sub-tabs for Overview, Moods, and Charts.
func (m Model) generateExploreContent(mainWidth int) string {
	if m.exploreLoading {
		return lipgloss.NewStyle().Foreground(colorSubtext).Render("Loading explore...")
	}
	if m.exploreErr != "" {
		return lipgloss.NewStyle().Foreground(colorRed).Render("Error: " + m.exploreErr)
	}
	if m.exploreData == nil {
		return ""
	}

	var mb strings.Builder

	// Header and Tabs
	header := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Explore")
	mb.WriteString(header)
	mb.WriteString("\n\n")

	tabs := []struct {
		id    string
		label string
	}{
		{"overview", "Overview"},
		{"moods", "Moods & Genres"},
		{"charts", "Charts"},
	}

	var renderedTabs []string
	activeTab := m.exploreSubTab
	if activeTab == "moodPlaylists" {
		activeTab = "moods"
	}
	
	for _, t := range tabs {
		style := lipgloss.NewStyle().Padding(0, 2).Foreground(colorSubtext)
		if activeTab == t.id {
			style = style.Bold(true).Foreground(colorAccent).Background(colorFocusBg)
		}
		renderedTabs = append(renderedTabs, m.zone.Mark("explore_tab_"+t.id, style.Render(t.label)))
	}
	mb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...))
	mb.WriteString("\n\n")

	// Render the selected sub-tab content
	switch m.exploreSubTab {
	case "overview":
		mb.WriteString(m.renderExploreOverview(mainWidth))
	case "moods":
		mb.WriteString(m.renderExploreMoods(mainWidth))
	case "moodPlaylists":
		mb.WriteString(m.renderMoodPlaylists(mainWidth))
	case "charts":
		mb.WriteString(m.renderExploreCharts(mainWidth))
	}

	return mb.String()
}

func (m Model) exploreOverviewCarousels() []ytmapi.HomeCarousel {
	var carousels []ytmapi.HomeCarousel
	if m.exploreData == nil {
		return carousels
	}
	if len(m.exploreData.NewReleases) > 0 {
		items := make([]ytmapi.HomeCarouselItem, len(m.exploreData.NewReleases))
		for i, r := range m.exploreData.NewReleases {
			items[i] = ytmapi.HomeCarouselItem{
				Title:      r.Title,
				BrowseID:   r.BrowseID,
				PlaylistID: r.PlaylistID,
				Year:       r.Year,
				Thumbnails: r.Thumbnails,
			}
		}
		carousels = append(carousels, ytmapi.HomeCarousel{Title: "New Releases", Contents: items})
	}
	if m.exploreData.Trending != nil && len(m.exploreData.Trending.Items) > 0 {
		carousels = append(carousels, ytmapi.HomeCarousel{Title: "Trending", Contents: m.exploreData.Trending.Items})
	}
	if len(m.exploreData.NewVideos) > 0 {
		carousels = append(carousels, ytmapi.HomeCarousel{Title: "New Music Videos", Contents: m.exploreData.NewVideos})
	}
	return carousels
}

func (m Model) renderExploreOverview(w int) string {
	var mb strings.Builder
	
	carousels := m.exploreOverviewCarousels()
	for i, car := range carousels {
		mb.WriteString(m.renderCarouselRow(i, car.Title, car.Contents, w))
		mb.WriteString("\n")
	}

	return mb.String()
}

func (m Model) renderExploreMoods(w int) string {
	if m.moodCatsLoading || (m.activeMoodParams != "" && m.exploreLoading) {
		return lipgloss.NewStyle().Foreground(colorSubtext).Render("Loading...")
	}

	var mb strings.Builder

	if m.activeMoodParams != "" && len(m.moodPlaylists) > 0 {
		backBtn := m.zone.Mark("mood_back", lipgloss.NewStyle().Padding(0, 1).Background(colorFocusBg).Foreground(colorAccent).Render("< Back to Moods"))
		mb.WriteString(backBtn)
		mb.WriteString("\n\n")

		// m.moodPlaylists is a list of playlists (dictionaries). We can render them as a generic 2-column grid.
		colWidth := (w - 4) / 2
		var rows []string
		for i := 0; i < len(m.moodPlaylists); i += 2 {
			var cols []string
			for j := 0; j < 2 && i+j < len(m.moodPlaylists); j++ {
				p := m.moodPlaylists[i+j]
				title := mapStr(p, "title")
				if len(title) > 35 { title = title[:32] + "..." }
				
				thumb := artPlaceholder()
				
				getThumb := func(item map[string]any) string {
					if thumbs, ok := item["thumbnails"].([]any); ok && len(thumbs) > 0 {
						if t, ok := thumbs[0].(map[string]any); ok {
							if url, ok := t["url"].(string); ok {
								return url
							}
						}
					}
					return ""
				}

				if url := getThumb(p); url != "" {
					thumb = m.cachedArtAt(url, 8, 4)
				}
				
				textWidth := colWidth - 12
				if textWidth < 10 { textWidth = 10 }
				
				textCol := lipgloss.NewStyle().Bold(true).Foreground(colorText).Width(textWidth).Render(title)
				cell := lipgloss.JoinHorizontal(lipgloss.Top, thumb, "  ", textCol)
				cell = lipgloss.NewStyle().Width(colWidth).Render(cell)
				
				if pid := mapStr(p, "playlistId"); pid != "" {
					cell = m.zone.Mark("open_playlist_"+pid, cell)
				}
				cols = append(cols, cell)
			}
			if len(cols) == 1 {
				rows = append(rows, cols[0])
			} else {
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cols[0], "    ", cols[1]))
			}
		}
		mb.WriteString(strings.Join(rows, "\n\n"))
		return mb.String()
	}

	if len(m.moodCategories) == 0 {
		return lipgloss.NewStyle().Foreground(colorSubtext).Render("No moods available")
	}
	
	for section, categories := range m.moodCategories {
		mb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(section))
		mb.WriteString("\n\n")
		
		var rows []string
		var currentRow []string
		currentW := 0
		
		for _, cat := range categories {
			tile := lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSearchBg).
				Padding(1, 3).
				Margin(0, 1, 1, 0).
				Render(cat.Title)
				
			tile = m.zone.Mark("mood_"+cat.Params, tile)
			
			tileW := lipgloss.Width(tile)
			if currentW+tileW > w && len(currentRow) > 0 {
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, currentRow...))
				currentRow = nil
				currentW = 0
			}
			currentRow = append(currentRow, tile)
			currentW += tileW
		}
		if len(currentRow) > 0 {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, currentRow...))
		}
		
		mb.WriteString(strings.Join(rows, "\n"))
		mb.WriteString("\n\n")
	}

	return mb.String()
}

func (m Model) renderExploreCharts(w int) string {
	if m.chartsLoading {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading charts...")
	}
	if m.chartsData == nil {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("No charts available")
	}

	var mb strings.Builder
	
	// Create columns.
	leftColW := (w * 4) / 10
	rightColW := w - leftColW - 4

	var leftCol strings.Builder
	var rightCol strings.Builder

	// ── Left Column: Top Artists ──
	if len(m.chartsData.Artists) > 0 {
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1)
		leftCol.WriteString(headerStyle.Render("Top Artists"))
		leftCol.WriteString("\n")
		
		for i, artist := range m.chartsData.Artists {
			if i >= 15 { break } // limit to top 15 for aesthetics
			
			trendColor := colorSubtext
			trendIcon := "━"
			if artist.Trend == "up" {
				trendColor = lipgloss.Color("#4ade80") // light green
				trendIcon = "▲"
			} else if artist.Trend == "down" {
				trendColor = colorRed
				trendIcon = "▼"
			}
			
			rank := lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(colorText).Bold(true).Render(artist.Rank)
			trend := lipgloss.NewStyle().Width(2).Foreground(trendColor).Render(trendIcon)
			
			titleStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
			subStyle := lipgloss.NewStyle().Foreground(colorSubtext)
			
			details := lipgloss.JoinVertical(lipgloss.Left,
				titleStyle.Render(artist.Title),
				subStyle.Render(artist.Subscribers),
			)
			
			// Try to get artist thumbnail if available
			thumb := ""
			if len(artist.Thumbnails) > 0 {
				thumb = m.cachedArtAt(artist.Thumbnails[0].URL, 8, 4) + " "
			}
			
			row := lipgloss.JoinHorizontal(lipgloss.Top, rank, " ", trend, " ", thumb, details)
			row = lipgloss.NewStyle().
				Width(leftColW - 2).
				Padding(1, 0).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(colorDivider).
				Render(row)
				
			if artist.BrowseID != "" {
				row = m.zone.Mark("open_artist_"+artist.BrowseID, row)
			}
			
			leftCol.WriteString(row)
			leftCol.WriteString("\n")
		}
	}

	// ── Right Column: Top Videos & Daily Trends ──
	
	// Top Videos Grid
	if len(m.chartsData.Videos) > 0 {
		rightCol.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1).Render("Top Music Videos"))
		rightCol.WriteString("\n")
		
		var videoRows []string
		var currentVideoRow []string
		cardW := (rightColW - 4) / 2
		
		for i, v := range m.chartsData.Videos {
			if i >= 6 { break } // max 6 videos
			
			thumb := sizedPlaceholder(cardW-2, 6)
			if len(v.Thumbnails) > 0 {
				thumb = m.cachedArtAt(v.Thumbnails[0].URL, cardW-2, 6)
			}
			
			title := v.Title
			if len(title) > 30 { title = title[:27] + "..." }
			
			artist := ""
			if len(v.Artists) > 0 {
				artist = v.Artists[0].Name
			}
			
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
			if zid != "" {
				card = m.zone.Mark(zid, card)
			}
			
			currentVideoRow = append(currentVideoRow, card)
			if len(currentVideoRow) == 2 {
				videoRows = append(videoRows, lipgloss.JoinHorizontal(lipgloss.Top, currentVideoRow...))
				currentVideoRow = nil
			}
		}
		if len(currentVideoRow) > 0 {
			videoRows = append(videoRows, lipgloss.JoinHorizontal(lipgloss.Top, currentVideoRow...))
		}
		rightCol.WriteString(strings.Join(videoRows, "\n"))
		rightCol.WriteString("\n\n")
	}

	// Daily Top Songs (List view)
	if len(m.chartsData.Daily) > 0 {
		rightCol.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1).Render("Daily Top Songs"))
		rightCol.WriteString("\n")
		
		for i, s := range m.chartsData.Daily {
			if i >= 10 { break }
			
			rank := lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(colorText).Bold(true).Render(fmt.Sprintf("%d.", i+1))
			
			title := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(s.Title)
			artist := ""
			if len(s.Artists) > 0 { artist = s.Artists[0].Name }
			
			info := lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.NewStyle().Foreground(colorSubtext).Render(artist))
			
			row := lipgloss.JoinHorizontal(lipgloss.Top, rank, "   ", info)
			row = lipgloss.NewStyle().
				Width(rightColW - 2).
				Padding(0, 0, 1, 0).
				Render(row)
				
			zid := entityZoneID(s.VideoID, s.BrowseID, s.PlaylistID)
			if zid != "" {
				row = m.zone.Mark(zid, row)
			}
			
			rightCol.WriteString(row)
			rightCol.WriteString("\n")
		}
	}

	mb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftColW).MarginRight(4).Render(leftCol.String()),
		lipgloss.NewStyle().Width(rightColW).Render(rightCol.String()),
	))

	return mb.String()
}

func (m Model) renderMoodPlaylists(w int) string {
	if m.exploreLoading {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading mood playlists...")
	}
	if len(m.moodPlaylists) == 0 {
		return lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("No playlists found for this mood.")
	}

	var mb strings.Builder
	
	mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("← Back to Moods & Genres (click Moods tab or shift+tab)"))
	mb.WriteString("\n\n")

	items := make([]ytmapi.HomeCarouselItem, 0, len(m.moodPlaylists))
	for _, rawItem := range m.moodPlaylists {
		ci := ytmapi.HomeCarouselItem{}
		if t, ok := rawItem["title"].(string); ok { ci.Title = t }
		if v, ok := rawItem["videoId"].(string); ok { ci.VideoID = v }
		if b, ok := rawItem["browseId"].(string); ok { ci.BrowseID = b }
		if p, ok := rawItem["playlistId"].(string); ok { ci.PlaylistID = p }
		
		if thumbs, ok := rawItem["thumbnails"].([]any); ok {
			for _, th := range thumbs {
				if thumbMap, ok := th.(map[string]any); ok {
					url, _ := thumbMap["url"].(string)
					if url != "" {
						ci.Thumbnails = append(ci.Thumbnails, ytmapi.Thumbnail{URL: url})
					}
				}
			}
		}
		items = append(items, ci)
	}
	
	// Render them using renderCarouselRow as a single giant carousel, 
	// or better, render them as a grid wrapping if we have many.
	// But `renderCarouselRow` already limits to visible width and handles arrows.
	// Let's just use `renderCarouselRow` for simplicity and pass the whole list.
	// We'll give it a title matching the active params for offsets.
	title := "Mood: " + m.activeMoodParams
	mb.WriteString(m.renderCarouselRow(0, title, items, w))

	return mb.String()
}
