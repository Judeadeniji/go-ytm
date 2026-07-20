package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func (m Model) generateArtistContent(mainWidth int) string {
	if m.artistPage == nil {
		return "Loading artist…"
	}
	a := m.artistPage
	var sb strings.Builder

	// 1. Artist Image + Centered Banner
	art := artPlaceholder()
	if len(a.Thumbnails) > 0 {
		// Use the largest thumbnail for banner
		art = m.cachedArtAt(a.Thumbnails[len(a.Thumbnails)-1].URL, artWidth, artHeight)
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	title := titleStyle.Render(a.Name)

	metaParts := []string{}
	if a.Subscribers != "" {
		sub := ytmapi.FormatCount(a.Subscribers)
		if sub == "" {
			sub = a.Subscribers
		}
		metaParts = append(metaParts, sub+" subscribers")
	}
	if a.MonthlyListeners != "" {
		ml := ytmapi.FormatCount(a.MonthlyListeners)
		if ml == "" {
			ml = a.MonthlyListeners
		}
		metaParts = append(metaParts, ml+" monthly listeners")
	}
	metaStyle := lipgloss.NewStyle().Foreground(colorSubtext)
	meta := metaStyle.Render(strings.Join(metaParts, "  ·  "))

	bannerText := lipgloss.JoinVertical(lipgloss.Left, title, meta)

	// Join art and text
	banner := lipgloss.JoinHorizontal(lipgloss.Center, art, "   ", bannerText)
	sb.WriteString(lipgloss.NewStyle().Width(mainWidth).Align(lipgloss.Left).Render(banner))
	sb.WriteString("\n\n")

	if a.Description != "" {
		desc := a.Description
		if len(desc) > 280 {
			desc = desc[:277] + "..."
		}
		descStyle := lipgloss.NewStyle().Foreground(colorSubtext).Align(lipgloss.Left).Width(mainWidth)
		sb.WriteString(descStyle.Render(desc))
		sb.WriteString("\n\n")
	}

	// Helper to extract thumbnail URL from item
	getThumb := func(item map[string]any) string {
		if tList, ok := item["thumbnails"].([]any); ok && len(tList) > 0 {
			if t, ok := tList[0].(map[string]any); ok {
				if url, ok := t["url"].(string); ok {
					return url
				}
			}
		}
		return ""
	}

	// 2. Top Songs (2-col full grid with images)
	if a.Songs != nil && len(a.Songs.Results) > 0 {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("Top Songs"))
		sb.WriteString("\n\n")
		colWidth := (mainWidth - 4) / 2

		var rows []string
		for i := 0; i < len(a.Songs.Results); i += 2 {
			var cols []string
			for j := 0; j < 2; j++ {
				if i+j >= len(a.Songs.Results) || i+j >= 10 { // limit to 10
					break
				}
				item := a.Songs.Results[i+j]
				rowTitle := mapStr(item, "title")
				if b, ok := item["isExplicit"].(bool); ok && b {
					rowTitle += explicitBadge()
				}
				sub := artistRefName(item["album"])
				if v := ytmapi.FormatCount(mapStr(item, "views")); v != "" {
					if sub != "" {
						sub = sub + " · " + v
					} else {
						sub = v
					}
				}
				if sub == "" {
					sub = "Song"
				}

				// Truncate to fit column minus image width (approx 8 chars for art + padding)
				textWidth := colWidth - 12
				if textWidth < 10 {
					textWidth = 10
				}
				if len(rowTitle) > textWidth {
					rowTitle = rowTitle[:textWidth-3] + "..."
				}
				if len(sub) > textWidth {
					sub = sub[:textWidth-3] + "..."
				}

				zoneID := artistItemZone("song", item)
				focused := m.focusedArtistZone(zoneID)
				bg := colorBg
				titleColor := colorText
				if focused {
					bg = colorFocusBg
					titleColor = colorAccent
				}

				songArt := sizedPlaceholder(sugArtWidth, sugArtHeight)
				if url := getThumb(item); url != "" {
					songArt = m.cachedArtAt(url, sugArtWidth, sugArtHeight)
				}

				textCol := lipgloss.JoinVertical(lipgloss.Left,
					lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Width(textWidth).Render(rowTitle),
					lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Width(textWidth).Render(sub),
				)

				cell := lipgloss.JoinHorizontal(lipgloss.Top, songArt, "  ", textCol)
				cell = lipgloss.NewStyle().Background(bg).Width(colWidth).Render(cell)
				if zoneID != "" {
					cell = m.zone.Mark(zoneID, cell)
				}
				cols = append(cols, cell)
			}
			if len(cols) == 1 {
				rows = append(rows, cols[0])
			} else if len(cols) == 2 {
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cols[0], "    ", cols[1]))
			}
		}
		sb.WriteString(strings.Join(rows, "\n\n"))
		sb.WriteString("\n\n")
	}

	// 3. Carousels for Albums, Singles, Fans Also Like
	renderArtistCarousel := func(title string, items []map[string]any, kind string) {
		if len(items) == 0 {
			return
		}
		contentWidth := mainWidth - 2
		cardWidth := 28

		titleStr := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(title)

		btnStyle := lipgloss.NewStyle().Padding(0, 1)
		leftBtn := m.zone.Mark(title+"_left", btnStyle.Render("<"))
		rightBtn := m.zone.Mark(title+"_right", btnStyle.Render(">"))
		arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)

		space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(arrows)
		if space < 1 {
			space = 1
		}
		header := lipgloss.JoinHorizontal(lipgloss.Top, titleStr, strings.Repeat(" ", space), arrows)
		sb.WriteString(header)
		sb.WriteString("\n\n")

		maxVisible := (contentWidth / cardWidth)
		if maxVisible < 1 {
			maxVisible = 1
		}

		offset := m.carouselOffsets[title]
		if offset < 0 {
			offset = 0
		}
		if offset > len(items) {
			offset = len(items)
		}

		visibleItems := items[offset:]
		if len(visibleItems) > maxVisible {
			visibleItems = visibleItems[:maxVisible]
		}

		var blocks []string
		for _, item := range visibleItems {
			t := mapStr(item, "title")
			if t == "" {
				t = mapStr(item, "artist")
			}
			if len(t) > 20 {
				t = t[:17] + "..."
			}

			s := ""
			switch kind {
			case "album":
				s = mapStr(item, "year")
				if typ := mapStr(item, "type"); typ != "" {
					if s != "" {
						s = typ + " · " + s
					} else {
						s = typ
					}
				}
			case "related":
				s = mapStr(item, "subscribers")
				if s == "" {
					s = "Artist"
				}
			}
			if len(s) > 22 {
				s = s[:19] + "..."
			}

			art := artPlaceholder()
			if url := getThumb(item); url != "" {
				art = m.cachedArtAt(url, artWidth, artHeight)
			}

			zoneID := artistItemZone(kind, item)
			focused := m.focusedArtistZone(zoneID)
			bg := colorBg
			titleColor := colorText
			if focused {
				bg = colorFocusBg
				titleColor = colorAccent
			}

			content := lipgloss.JoinVertical(lipgloss.Left,
				art, "",
				lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Render(t),
				lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(s),
			)

			content = lipgloss.NewStyle().Background(bg).Width(cardWidth - 2).Render(content)
			if zoneID != "" {
				content = m.zone.Mark(zoneID, content)
			}
			blocks = append(blocks, content)
		}

		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
		sb.WriteString("\n\n")
	}

	if a.Albums != nil {
		renderArtistCarousel("Albums", a.Albums.Results, "album")
	}
	if a.Singles != nil {
		renderArtistCarousel("Singles & EPs", a.Singles.Results, "album")
	}
	if a.Related != nil {
		renderArtistCarousel("Fans Also Like", a.Related.Results, "related")
	}

	return sb.String()
}

func artistItemZone(kind string, item map[string]any) string {
	switch kind {
	case "song", "video":
		if id := mapStr(item, "videoId"); id != "" {
			return "play_video_" + id
		}
	case "album":
		if id := mapStr(item, "browseId"); id != "" {
			return "open_album_" + id
		}
	case "related":
		if id := mapStr(item, "browseId"); id != "" {
			return "open_artist_" + id
		}
	}
	return ""
}

func (m Model) generateProfileContent(mainWidth int) string {
	if m.userPage == nil {
		return "Loading profile…"
	}
	u := m.userPage
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	title := titleStyle.Render(u.Name)

	badge := lipgloss.NewStyle().
		Background(colorSearchBg).Foreground(colorSubtext).
		Padding(0, 1).Render("PROFILE")

	header := lipgloss.JoinVertical(lipgloss.Left, badge, "", title)
	sb.WriteString(lipgloss.NewStyle().Padding(1, 2).Render(header))
	sb.WriteString("\n\n")

	var flatList []any
	if u.Playlists != nil {
		flatList = append(flatList, "Playlists")
		for _, item := range u.Playlists.Results {
			flatList = append(flatList, item)
		}
	}
	if u.Videos != nil {
		flatList = append(flatList, "Videos")
		for _, item := range u.Videos.Results {
			flatList = append(flatList, item)
		}
	}

	for i, item := range flatList {
		switch v := item.(type) {
		case string:
			sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Padding(1, 2).Render(v))
			sb.WriteString("\n")
		case map[string]any:
			focused := m.listCursor == i
			bg := colorBg
			titleColor := colorText
			prefix := "  "
			if focused {
				bg = colorFocusBg
				titleColor = colorAccent
				prefix = "› "
			}

			titleStr := mapStr(v, "title")
			subParts := []string{}
			if s := mapStr(v, "author"); s != "" {
				subParts = append(subParts, s)
			} else if s := mapStr(v, "views"); s != "" {
				subParts = append(subParts, s)
			}
			
			sub := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(strings.Join(subParts, " · "))
			titleStyled := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Render(prefix + titleStr)
			row := lipgloss.JoinVertical(lipgloss.Left, titleStyled, lipgloss.NewStyle().Background(bg).Render("  "+sub))
			row = lipgloss.NewStyle().Background(bg).Width(mainWidth - 4).Render(row)

			zid := artistItemZone("playlists", v)
			if zid != "" {
				row = m.zone.Mark(zid, row)
			}

			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
