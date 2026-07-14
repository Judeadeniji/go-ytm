package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) generateGridContent(mainWidth int) string {
	var mb strings.Builder

	// Filters
	var filters []string
	for _, f := range m.filters {
		filters = append(filters, lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 1).Render(f))
	}
	mb.WriteString("  ")
	mb.WriteString(strings.Join(filters, "   "))
	mb.WriteString("\n\n\n")

	if len(m.searchResults) > 0 {
		return m.generateSearchResultsContent(mainWidth)
	}

	if m.activeMenu == "Library" {
		return m.generateLibraryContent(mainWidth)
	} else if m.activeMenu == "Explore" {
		return m.generateExploreContent(mainWidth)
	} else if m.activeMenu == "Upgrade" {
		return lipgloss.NewStyle().Padding(4).Foreground(colorText).Render("Upgrade to YouTube Music Premium")
	}

	// Helper to render horizontal grid row
	renderGrid := func(index int, title string, cards []ytmapi.HomeCarouselItem) string {
		var row strings.Builder

		contentWidth := mainWidth - 2 // mainWidth minus left/right padding
		cardWidth := 28

		isActive := m.activePane == PaneMain && m.activeCarousel == index

		titleStyle := lipgloss.NewStyle().Bold(true)
		btnStyle := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2)

		if isActive {
			titleStyle = titleStyle.Foreground(colorText)
			btnStyle = btnStyle.Background(colorSearchBg).Foreground(colorText) // brighter when active
		} else {
			titleStyle = titleStyle.Foreground(colorSubtext)
			btnStyle = btnStyle.Background(colorBg).Foreground(colorSubtext) // dimmer when inactive
		}

		titleStr := titleStyle.Render(title)

		leftBtn := m.zone.Mark(title+"_left", btnStyle.Render("<"))
		rightBtn := m.zone.Mark(title+"_right", btnStyle.Render(">"))
		arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)

		// "More" pill is optional, skipped for simplicity
		rightControls := arrows

		space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(rightControls)
		if space < 1 {
			space = 1
		}
		row.WriteString(titleStr)
		row.WriteString(strings.Repeat(" ", space))
		row.WriteString(rightControls)
		row.WriteString("\n\n")

		var blocks []string

		// Apply carousel scrolling offset
		offset := m.carouselOffsets[title]
		if offset < 0 {
			offset = 0
		}
		if offset > len(cards) {
			offset = len(cards)
		}
		visibleCards := cards[offset:]
		maxVisible := (contentWidth / cardWidth) + 1
		if len(visibleCards) > maxVisible {
			visibleCards = visibleCards[:maxVisible]
		}

		for _, card := range visibleCards {
			t := card.Title
			if len(t) > 20 {
				t = t[:17] + "..."
			}
			s := card.Description
			if s == "" {
				if card.VideoID != "" {
					s = "Song/Video"
				} else if card.PlaylistID != "" {
					s = "Playlist"
				} else if card.BrowseID != "" {
					s = "Album/Artist"
				}
			}
			if len(s) > 20 {
				s = s[:17] + "..."
			}

			art := artPlaceholder()
			if len(card.Thumbnails) > 0 {
				if kitty, ok := m.imageCache[card.Thumbnails[0].URL]; ok && kitty != nil && kitty.Spacer != "" {
					art = kitty.Spacer
				}
			}

			cardTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(t)
			cardSubStyle := lipgloss.NewStyle().Foreground(colorSubtext).Render(s)

			content := lipgloss.JoinVertical(lipgloss.Left, art, "", cardTitleStyle, cardSubStyle)

			// Make it clickable if it has a VideoID
			if card.VideoID != "" {
				content = m.zone.Mark("search_result_video_"+card.VideoID, content)
			}

			block := lipgloss.NewStyle().
				Padding(0, 2).
				Width(cardWidth).
				Render(content)

			blocks = append(blocks, block)
		}

		row.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
		return row.String() + "\n\n\n"
	}

	if len(m.homeCarousels) == 0 {
		mb.WriteString("Loading Home...")
	} else {
		for i, carousel := range m.homeCarousels {
			mb.WriteString(renderGrid(i, carousel.Title, carousel.Contents))
		}
	}

	return mb.String()
}

func (m Model) generateLibraryContent(mainWidth int) string {
	var mb strings.Builder

	header := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Your Library")
	mb.WriteString(header)
	mb.WriteString("\n\n")

	for _, pl := range m.playlists {
		title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(pl[0])
		sub := lipgloss.NewStyle().Foreground(colorSubtext).Render(pl[1])
		art := artPlaceholder()
		if m.cachedArt != nil && m.cachedArt.Spacer != "" {
			art = m.cachedArt.Spacer
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, art, "   ", lipgloss.JoinVertical(lipgloss.Left, title, "\n"+sub))

		// Optional bubblezone if we want to make it playable

		mb.WriteString(row)
		mb.WriteString("\n\n")
	}

	return mb.String()
}

func (m Model) generateExploreContent(mainWidth int) string {
	var mb strings.Builder

	header := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Explore")
	mb.WriteString(header)
	mb.WriteString("\n\n")

	categories := []string{"New releases", "Charts", "Moods & genres"}

	for _, cat := range categories {
		pill := lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(1, 4).Render(cat)
		mb.WriteString(pill)
		mb.WriteString("   ")
	}
	mb.WriteString("\n\n")

	return mb.String()
}

func (m Model) generateSearchResultsContent(mainWidth int) string {
	var mb strings.Builder

	// Group by category, preserving order
	var categories []string
	grouped := make(map[string][]ytmapi.SearchResult)

	for _, res := range m.searchResults {
		if len(grouped[res.Category]) == 0 {
			categories = append(categories, res.Category)
		}
		grouped[res.Category] = append(grouped[res.Category], res)
	}

	for _, cat := range categories {
		results := grouped[cat]

		if cat == "Top result" {
			mb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Top result"))
			mb.WriteString("\n\n")

			res := results[0]
			title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(res.Title)
			var subParts []string
			if res.ResultType != "" {
				subParts = append(subParts, strings.ToUpper(res.ResultType[:1])+res.ResultType[1:])
			}
			if len(res.Artists) > 0 {
				subParts = append(subParts, res.Artists[0].Name)
			} else if res.Artist != "" {
				subParts = append(subParts, res.Artist)
			} else if res.Author != "" {
				subParts = append(subParts, res.Author)
			}
			if res.Album.Name != "" {
				subParts = append(subParts, res.Album.Name)
			}
			if res.Views != "" {
				subParts = append(subParts, res.Views)
			}
			if res.Duration != "" {
				subParts = append(subParts, res.Duration)
			}
			sub := lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(subParts, " • "))

			art := artPlaceholder()
			if len(res.Thumbnails) > 0 {
				if kitty, ok := m.imageCache[res.Thumbnails[0].URL]; ok && kitty != nil && kitty.Spacer != "" {
					art = kitty.Spacer
				}
			}

			row := lipgloss.JoinHorizontal(lipgloss.Top,
				art, "  ",
				lipgloss.JoinVertical(lipgloss.Left, title, "\n"+sub))

			if res.VideoID != "" {
				row = m.zone.Mark("search_result_video_"+res.VideoID, row)
			}

			banner := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorDivider).
				Padding(1, 2).
				Width(mainWidth - 4).
				Render(row)

			mb.WriteString(banner)
			mb.WriteString("\n\n\n")
		} else {
			mb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(cat))
			mb.WriteString("\n\n")

			for _, res := range results {
				title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(res.Title)
				var subParts []string
				if len(res.Artists) > 0 {
					subParts = append(subParts, res.Artists[0].Name)
				} else if res.Artist != "" {
					subParts = append(subParts, res.Artist)
				} else if res.Author != "" {
					subParts = append(subParts, res.Author)
				}

				if res.ResultType == "song" || res.ResultType == "video" {
					if res.Album.Name != "" {
						subParts = append(subParts, res.Album.Name)
					}
					if res.Duration != "" {
						subParts = append(subParts, res.Duration)
					}
					if res.Views != "" {
						subParts = append(subParts, res.Views)
					}
				} else if res.ResultType == "album" {
					if res.Year != "" {
						subParts = append(subParts, res.Year)
					}
				} else if res.ResultType == "playlist" {
					if res.ItemCount != "" {
						subParts = append(subParts, res.ItemCount+" tracks")
					}
				}

				sub := lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(subParts, " • "))

				row := lipgloss.JoinHorizontal(lipgloss.Top,
					lipgloss.NewStyle().PaddingTop(1).Foreground(colorSubtext).Render("▪"),
					"  ",
					lipgloss.JoinVertical(lipgloss.Left, title, sub))

				if res.VideoID != "" {
					row = m.zone.Mark("search_result_video_"+res.VideoID, row)
				}

				mb.WriteString(row)
				mb.WriteString("\n\n")
			}
			mb.WriteString("\n")
		}
	}

	return mb.String()
}
