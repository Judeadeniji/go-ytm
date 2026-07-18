package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// generateExploreContent renders the explore page with sub-tabs for Overview, Moods, and Charts.
func (m Model) generateExploreContent(mainWidth int) string {
	if m.exploreErr != "" {
		errText := lipgloss.NewStyle().Foreground(colorRed).Render("Error: " + m.exploreErr)
		retryBtn := lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 2).Render("Retry")
		retryBtn = m.zone.Mark("retry_page", retryBtn)
		return lipgloss.NewStyle().Padding(2).Render(errText + "\n\n" + retryBtn)
	}

	if m.exploreData == nil && !m.exploreLoading {
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
	mb.WriteString(strings.Join(renderedTabs, "  "))
	mb.WriteString("\n\n")

	if m.exploreLoading || m.pageLoading {
		mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading explore..."))
		return mb.String()
	}

	// Render the selected sub-tab content
	switch m.exploreSubTab {
	case "overview":
		mb.WriteString(m.renderExploreOverview(mainWidth))
	case "moods":
		mb.WriteString(m.renderExploreMoods(mainWidth))
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

	// ── Playlist grid (after a mood category is selected) ────────────────
	if m.activeMoodParams != "" {
		backBtn := m.zone.Mark("mood_back", lipgloss.NewStyle().
			Padding(0, 1).Background(colorFocusBg).Foreground(colorAccent).
			Render("< Back to Moods"))
		mb.WriteString(backBtn)
		mb.WriteString("\n\n")

		if len(m.moodPlaylists) == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No playlists found for this mood."))
			return mb.String()
		}

		const thumbW, thumbH = 8, 4
		colWidth := (w - 4) / 2
		var rows []string
		for i := 0; i < len(m.moodPlaylists); i += 2 {
			var cols []string
			for j := 0; j < 2 && i+j < len(m.moodPlaylists); j++ {
				p := m.moodPlaylists[i+j]
				title := mapStr(p, "title")
				if len(title) > 35 {
					title = title[:32] + "..."
				}

				thumb := sizedPlaceholder(thumbW, thumbH)
				if thumbs, ok := p["thumbnails"].([]any); ok && len(thumbs) > 0 {
					if t, ok := thumbs[0].(map[string]any); ok {
						if url, _ := t["url"].(string); url != "" {
							thumb = m.cachedArtAt(url, thumbW, thumbH)
						}
					}
				}

				textWidth := colWidth - thumbW - 4
				if textWidth < 10 {
					textWidth = 10
				}
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



