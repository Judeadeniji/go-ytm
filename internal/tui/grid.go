package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

// generateMainContent routes the center pane by navigation stack / search / menu.
func (m Model) generateMainContent(mainWidth int) string {
	if m.pageErr != "" {
		errText := lipgloss.NewStyle().Foreground(colorRed).Render("Error: " + m.pageErr)
		retryBtn := lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorText).Padding(0, 2).Render("Retry")
		retryBtn = m.zone.Mark("retry_page", retryBtn)
		return lipgloss.NewStyle().Padding(2).Render(errText + "\n\n" + retryBtn)
	}

	if sc, ok := m.stack.Current(); ok {
		switch sc.Kind {
		case ScreenArtist:
			return m.generateArtistContent(mainWidth)
		case ScreenAlbum:
			return m.generateAlbumContent(mainWidth)
		case ScreenPlaylist:
			return m.generatePlaylistContent(mainWidth)
		case ScreenSearch:
			return m.generateSearchResultsContent(mainWidth)
		}
	}

	if len(m.searchResults) > 0 {
		return m.generateSearchResultsContent(mainWidth)
	}

	switch m.activeMenu {
	case "Library":
		return m.generateLibraryContent(mainWidth)
	case "Explore":
		return m.generateExploreContent(mainWidth)
	case "Settings":
		return m.generateSettingsContent(mainWidth)
	default:
		return m.generateHomeContent(mainWidth)
	}
}

func (m Model) generateHomeContent(mainWidth int) string {
	var mb strings.Builder

	if len(m.homeCarousels) == 0 {
		mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("Loading Home…"))
		return mb.String()
	}

	hour := time.Now().Hour()
	greeting := "Good evening"
	if hour < 12 {
		greeting = "Good morning"
	} else if hour < 17 {
		greeting = "Good afternoon"
	}
	if m.userProfile != nil && m.userProfile.Name != "" {
		// e.g. "Good morning, Oluwaferanmi"
		first_name := strings.Split(m.userProfile.Name, " ")[0]
		greeting += ", " + first_name
	}
	mb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).PaddingBottom(1).Render(greeting))
	mb.WriteString("\n\n")

	for i, carousel := range m.homeCarousels {
		titleLower := strings.ToLower(carousel.Title)
		if strings.Contains(titleLower, "quick picks") {
			mb.WriteString(m.renderQuickPicksCarousel(i, carousel.Title, carousel.Contents, mainWidth))
		} else if strings.Contains(titleLower, "mixed for you") || strings.Contains(titleLower, "listen again") {
			mb.WriteString(m.renderMixCarousel(i, carousel.Title, carousel.Contents, mainWidth))
		} else {
			mb.WriteString(m.renderCarouselRow(i, carousel.Title, carousel.Contents, mainWidth))
		}
	}
	return mb.String()
}

func (m Model) renderCarouselRow(index int, title string, cards []ytmapi.HomeCarouselItem, mainWidth int) string {
	var row strings.Builder

	contentWidth := mainWidth - 2
	cardWidth := 28

	isActive := m.activePane == PaneMain && m.activeCarousel == index

	titleStyle := lipgloss.NewStyle().Bold(true)
	btnStyle := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2)

	if isActive {
		titleStyle = titleStyle.Foreground(colorText)
		btnStyle = btnStyle.Background(colorSearchBg).Foreground(colorText)
	} else {
		titleStyle = titleStyle.Foreground(colorSubtext)
		btnStyle = btnStyle.Background(colorBg).Foreground(colorSubtext)
	}

	titleStr := titleStyle.Render(title)
	leftBtn := m.zone.Mark(title+"_left", btnStyle.Render("<"))
	rightBtn := m.zone.Mark(title+"_right", btnStyle.Render(">"))
	arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)

	space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(arrows)
	if space < 1 {
		space = 1
	}
	row.WriteString(titleStr)
	row.WriteString(strings.Repeat(" ", space))
	row.WriteString(arrows)
	row.WriteString("\n\n")

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

	var blocks []string
	for vi, card := range visibleCards {
		cardIndex := offset + vi
		t := card.Title
		if len(t) > 20 {
			t = t[:17] + "..."
		}
		if card.IsExplicit {
			t += explicitBadge()
		}
		s := homeCardSubtitle(card)
		if len(s) > 22 {
			s = s[:19] + "..."
		}

		art := artPlaceholder()
		if len(card.Thumbnails) > 0 {
			art = m.cachedArtAt(card.Thumbnails[0].URL, artWidth, artHeight)
		}

		titleColor := colorText
		focused := m.focusedHomeCard(index, cardIndex)
		bg := colorBg
		if focused {
			bg = colorFocusBg
			titleColor = colorAccent
		}

		content := lipgloss.JoinVertical(lipgloss.Left,
			art, "",
			lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Render(t),
			lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(s),
		)

		if zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID); zid != "" {
			content = m.zone.Mark(zid, content)
		}

		style := lipgloss.NewStyle().Padding(0, 2).Width(cardWidth).Background(bg)
		blocks = append(blocks, style.Render(content))
	}

	row.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	return row.String() + "\n\n\n"
}

func homeCardSubtitle(card ytmapi.HomeCarouselItem) string {
	if card.Description != "" {
		return card.Description
	}
	if len(card.Artists) > 0 {
		sub := card.Artists[0].Name
		if card.Year != "" {
			sub += " · " + card.Year
		}
		return sub
	}
	if card.Year != "" {
		return card.Year
	}
	if card.Views != "" {
		return ytmapi.FormatCount(card.Views)
	}
	if card.Subscribers != "" {
		return card.Subscribers + " subs"
	}
	if card.VideoID != "" {
		return "Song"
	}
	if card.PlaylistID != "" {
		return "Playlist"
	}
	if strings.HasPrefix(card.BrowseID, "UC") {
		return "Artist"
	}
	if strings.HasPrefix(card.BrowseID, "MPRE") {
		return "Album"
	}
	return ""
}

// entityZoneID picks a clickable zone for a home/search entity.
func entityZoneID(videoID, browseID, playlistID string) string {
	if videoID != "" {
		return "play_video_" + videoID
	}
	if strings.HasPrefix(browseID, "UC") {
		return "open_artist_" + browseID
	}
	if strings.HasPrefix(browseID, "MPREb_") || strings.HasPrefix(browseID, "MPRE") {
		return "open_album_" + browseID
	}
	if strings.HasPrefix(browseID, "OLAK5uy_") {
		return "open_olak_" + browseID
	}
	if playlistID != "" {
		if strings.HasPrefix(playlistID, "OLAK5uy_") {
			return "open_olak_" + playlistID
		}
		return "open_playlist_" + playlistID
	}
	if strings.HasPrefix(browseID, "VL") {
		return "open_playlist_" + browseID
	}
	if browseID != "" {
		return "open_browse_" + browseID
	}
	return ""
}

func (m Model) generateLibraryContent(mainWidth int) string {
	var mb strings.Builder
	headerText := "Your Library"
	if m.userProfile != nil && m.userProfile.Name != "" {
		headerText = m.userProfile.Name + "'s Library"
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(headerText)
	mb.WriteString(header)
	mb.WriteString("\n\n")

	chipFilters := []struct {
		label string
		value string
	}{
		{"Playlists", "playlists"},
		{"Songs", "songs"},
		{"Albums", "albums"},
		{"Artists", "artists"},
		{"Downloads", "downloads"},
	}

	var chips []string
	for _, c := range chipFilters {
		s := lipgloss.NewStyle().Padding(0, 2).Foreground(colorText).Background(colorSearchBg)
		if m.libraryTab == c.value {
			s = s.Background(colorText).Foreground(colorBg).Bold(true)
		}
		ch := m.zone.Mark(fmt.Sprintf("lib_tab_%s", c.value), s.Render(c.label))
		chips = append(chips, ch)
	}
	mb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, chips...))
	mb.WriteString("\n\n")

	if !m.isAuthenticated && m.libraryTab != "downloads" {
		mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("Sign in coming later — library sync needs YouTube Music auth."))
		mb.WriteString("\n\n")
		return mb.String()
	}

	if m.pageLoading {
		mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Padding(2).Render("Loading…"))
		return mb.String()
	}

	switch m.libraryTab {
	case "playlists":
		if len(m.libPlaylists) == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No playlists found."))
		} else {
			var cards []ytmapi.HomeCarouselItem
			for _, p := range m.libPlaylists {
				title, _ := p["title"].(string)
				id, _ := p["playlistId"].(string)
				cards = append(cards, ytmapi.HomeCarouselItem{
					ZoneID:     "open_playlist_" + id,
					Title:      title,
					PlaylistID: id,
					Thumbnails: mapThumbnails(p),
				})
			}
			mb.WriteString(m.renderGrid("Playlists", cards, mainWidth))
		}
	case "songs":
		if len(m.libSongs) == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No songs found."))
		} else {
			var tracks []ytmapi.TrackItem
			for _, s := range m.libSongs {
				tracks = append(tracks, mapTrackItem(s))
			}
			viewsW := tracklistViewsWidth(tracks)
			for i, tr := range tracks {
				focused := m.activePane == PaneMain && i == m.listCursor
				mb.WriteString(m.renderTrackRow(i, tr, mainWidth, focused, viewsW))
				mb.WriteString("\n")
			}
		}
	case "albums":
		if len(m.libAlbums) == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No albums found."))
		} else {
			var cards []ytmapi.HomeCarouselItem
			for _, a := range m.libAlbums {
				title, _ := a["title"].(string)
				id, _ := a["browseId"].(string)
				year, _ := a["year"].(string)
				cards = append(cards, ytmapi.HomeCarouselItem{
					ZoneID:     "open_album_" + id,
					Title:      title,
					BrowseID:   id,
					Thumbnails: mapThumbnails(a),
				})
				_ = year // Not shown in basic carousel
			}
			mb.WriteString(m.renderGrid("Albums", cards, mainWidth))
		}
	case "artists":
		if len(m.libArtists) == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No artists found."))
		} else {
			var cards []ytmapi.HomeCarouselItem
			for _, a := range m.libArtists {
				name, _ := a["artist"].(string)
				id, _ := a["browseId"].(string)
				cards = append(cards, ytmapi.HomeCarouselItem{
					ZoneID:     "open_artist_" + id,
					Title:      name,
					BrowseID:   id,
					Thumbnails: mapThumbnails(a),
				})
			}
			mb.WriteString(m.renderGrid("Artists", cards, mainWidth))
		}
	case "downloads":
		if len(m.libDownloads) == 0 {
			mb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("No downloaded tracks."))
		} else {
			var tracks []ytmapi.TrackItem
			for _, t := range m.libDownloads {
				tracks = append(tracks, ytmapi.TrackItem{
					VideoID:  t.VideoID,
					Title:    t.Title,
					Artist:   t.Artist,
					Duration: t.Duration,
					Album:    ytmapi.NamedRef{Name: t.Album},
				})
			}
			viewsW := tracklistViewsWidth(tracks)
			for i, tr := range tracks {
				focused := m.activePane == PaneMain && i == m.listCursor
				mb.WriteString(m.renderTrackRow(i, tr, mainWidth, focused, viewsW))
				mb.WriteString("\n")
			}
		}
	}

	return mb.String()
}

func mapThumbnails(item map[string]any) []ytmapi.Thumbnail {
	if thumbs, ok := item["thumbnails"].([]any); ok && len(thumbs) > 0 {
		var out []ytmapi.Thumbnail
		for _, t := range thumbs {
			if tm, ok := t.(map[string]any); ok {
				url, _ := tm["url"].(string)
				width, _ := tm["width"].(float64)
				height, _ := tm["height"].(float64)
				out = append(out, ytmapi.Thumbnail{URL: url, Width: int(width), Height: int(height)})
			}
		}
		return out
	}
	return nil
}

func mapTrackItem(item map[string]any) ytmapi.TrackItem {
	var tr ytmapi.TrackItem
	tr.Title, _ = item["title"].(string)
	tr.VideoID, _ = item["videoId"].(string)
	tr.Duration, _ = item["duration"].(string)
	
	if artists, ok := item["artists"].([]any); ok {
		for _, a := range artists {
			if am, ok := a.(map[string]any); ok {
				name, _ := am["name"].(string)
				id, _ := am["id"].(string)
				tr.Artists = append(tr.Artists, ytmapi.NamedRef{Name: name, ID: id})
			}
		}
	}
	
	if album, ok := item["album"].(map[string]any); ok {
		name, _ := album["name"].(string)
		id, _ := album["id"].(string)
		tr.Album = ytmapi.NamedRef{Name: name, ID: id}
	}
	
	tr.Thumbnails = mapThumbnails(item)
	return tr
}

func (m Model) generateSearchResultsContent(mainWidth int) string {
	var mb strings.Builder

	// Filter chips
	chipFilters := []struct {
		label string
		value string
	}{
		{"All", ""},
		{"Songs", "songs"},
		{"Albums", "albums"},
		{"Artists", "artists"},
		{"Playlists", "playlists"},
	}
	var chips []string
	for _, cf := range chipFilters {
		style := lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorSubtext).Padding(0, 1)
		if m.searchFilter == cf.value {
			style = style.Foreground(colorText).Bold(true)
		}
		chips = append(chips, m.zone.Mark("search_filter_"+cf.value, style.Render(cf.label)))
	}
	mb.WriteString(strings.Join(chips, "  "))
	mb.WriteString("\n\n")

	var categories []string
	grouped := make(map[string][]ytmapi.SearchResult)
	for _, res := range m.searchResults {
		cat := res.Category
		if cat == "" {
			cat = titleCase(res.ResultType)
		}
		if len(grouped[cat]) == 0 {
			categories = append(categories, cat)
		}
		grouped[cat] = append(grouped[cat], res)
	}

	flatIdx := 0
	for _, cat := range categories {
		results := grouped[cat]
		mb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(cat))
		mb.WriteString("\n\n")

		for _, res := range results {
			focused := m.focusedSearchResult(flatIdx)
			flatIdx++

			title := res.Title
			if res.IsExplicit {
				title += explicitBadge()
			}
			if res.VideoID != "" {
				for _, dt := range m.libDownloads {
					if dt.VideoID == res.VideoID {
						title += lipgloss.NewStyle().Foreground(colorAccent).Render(" ✓")
						break
					}
				}
			}
			badge := ""
			if res.ResultType == "album" && res.Type != "" {
				badge = res.Type
			} else if res.ResultType != "" {
				badge = titleCase(res.ResultType)
			}

			var subParts []string
			if badge != "" {
				subParts = append(subParts, badge)
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
			if res.Year != "" {
				subParts = append(subParts, res.Year)
			}
			if res.Duration != "" {
				subParts = append(subParts, res.Duration)
			}
			if res.Views != "" {
				subParts = append(subParts, ytmapi.FormatCount(res.Views)+" plays")
			}
			if res.ItemCount != "" {
				if n, err := strconv.Atoi(strings.TrimSpace(res.ItemCount)); err == nil {
					subParts = append(subParts, pluralCount(n, "track", "tracks"))
				} else {
					subParts = append(subParts, res.ItemCount+" tracks")
				}
			}

			bg := colorBg
			titleColor := colorText
			prefix := "  "
			if focused {
				bg = colorFocusBg
				titleColor = colorAccent
				prefix = "› "
			}

			sub := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(strings.Join(subParts, " · "))
			titleStyled := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Render(prefix + title)
			row := lipgloss.JoinVertical(lipgloss.Left, titleStyled, lipgloss.NewStyle().Background(bg).Render("  "+sub))
			row = lipgloss.NewStyle().Background(bg).Width(mainWidth - 4).Render(row)

			zid := searchResultZone(res)
			if zid != "" {
				row = m.zone.Mark(zid, row)
			}

			mb.WriteString(row)
			mb.WriteString("\n\n")
		}
		mb.WriteString("\n")
	}

	return mb.String()
}

func searchResultZone(res ytmapi.SearchResult) string {
	browseID := res.BrowseID
	if browseID == "" && len(res.Artists) > 0 {
		browseID = res.Artists[0].ID
	}
	switch res.ResultType {
	case "song", "video", "episode":
		if res.VideoID != "" {
			return "play_video_" + res.VideoID
		}
	case "artist":
		if browseID != "" {
			return "open_artist_" + browseID
		}
	case "album":
		if browseID != "" {
			return "open_album_" + browseID
		}
	case "playlist":
		id := browseID
		if id == "" {
			id = res.PlaylistID
		}
		if id != "" {
			return "open_playlist_" + id
		}
	}
	return entityZoneID(res.VideoID, browseID, res.PlaylistID)
}

func (m Model) renderMixCarousel(index int, title string, cards []ytmapi.HomeCarouselItem, mainWidth int) string {
	var row strings.Builder

	contentWidth := mainWidth - 2
	cardWidth := 34 // slightly wider card for mixes to look premium

	isActive := m.activePane == PaneMain && m.activeCarousel == index

	titleStyle := lipgloss.NewStyle().Bold(true)
	btnStyle := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2)

	if isActive {
		titleStyle = titleStyle.Foreground(colorText)
		btnStyle = btnStyle.Background(colorSearchBg).Foreground(colorText)
	} else {
		titleStyle = titleStyle.Foreground(colorSubtext)
		btnStyle = btnStyle.Background(colorBg).Foreground(colorSubtext)
	}

	titleStr := titleStyle.Render(title)
	leftBtn := m.zone.Mark(title+"_left", btnStyle.Render("<"))
	rightBtn := m.zone.Mark(title+"_right", btnStyle.Render(">"))
	arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)

	space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(arrows)
	if space < 1 {
		space = 1
	}
	row.WriteString(titleStr)
	row.WriteString(strings.Repeat(" ", space))
	row.WriteString(arrows)
	row.WriteString("\n\n")

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

	var blocks []string
	for vi, card := range visibleCards {
		cardIndex := offset + vi
		t := card.Title
		if len(t) > 28 {
			t = t[:25] + "..."
		}
		if card.IsExplicit {
			t += explicitBadge()
		}
		s := homeCardSubtitle(card)
		if len(s) > 30 {
			s = s[:27] + "..."
		}

		art := artPlaceholder()
		if len(card.Thumbnails) > 0 {
			art = m.cachedArtAt(card.Thumbnails[0].URL, artWidth, artHeight)
		}

		titleColor := colorText
		focused := m.focusedHomeCard(index, cardIndex)

		bg := lipgloss.Color("#1A1025") // slight purple tint for mix background
		if focused {
			bg = lipgloss.Color("#3A2055") // brighter purple for focus
			titleColor = colorText
		}

		content := lipgloss.JoinVertical(lipgloss.Left,
			art, "",
			lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Render(t),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")).Background(bg).Render(s),
		)

		style := lipgloss.NewStyle().Padding(1, 2).Width(cardWidth).Background(bg).MarginRight(2)
		styledContent := style.Render(content)
		if zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID); zid != "" {
			styledContent = m.zone.Mark(zid, styledContent)
		}
		blocks = append(blocks, styledContent)
	}

	row.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	return row.String() + "\n\n\n"
}

func (m Model) renderQuickPicksCarousel(index int, title string, cards []ytmapi.HomeCarouselItem, mainWidth int) string {
	var row strings.Builder

	contentWidth := mainWidth - 2
	cardWidth := 56 // wide horizontal card

	isActive := m.activePane == PaneMain && m.activeCarousel == index

	titleStyle := lipgloss.NewStyle().Bold(true)
	btnStyle := lipgloss.NewStyle().Background(colorHover).Foreground(colorText).Padding(0, 2)

	if isActive {
		titleStyle = titleStyle.Foreground(colorText)
		btnStyle = btnStyle.Background(colorSearchBg).Foreground(colorText)
	} else {
		titleStyle = titleStyle.Foreground(colorSubtext)
		btnStyle = btnStyle.Background(colorBg).Foreground(colorSubtext)
	}

	titleStr := titleStyle.Render(title)
	leftBtn := m.zone.Mark(title+"_left", btnStyle.Render("<"))
	rightBtn := m.zone.Mark(title+"_right", btnStyle.Render(">"))
	arrows := lipgloss.JoinHorizontal(lipgloss.Top, leftBtn, " ", rightBtn)

	space := contentWidth - lipgloss.Width(titleStr) - lipgloss.Width(arrows)
	if space < 1 {
		space = 1
	}
	row.WriteString(titleStr)
	row.WriteString(strings.Repeat(" ", space))
	row.WriteString(arrows)
	row.WriteString("\n\n")

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

	var blocks []string
	for vi, card := range visibleCards {
		cardIndex := offset + vi
		t := card.Title
		if len(t) > 40 {
			t = t[:37] + "..."
		}
		if card.IsExplicit {
			t += explicitBadge()
		}
		s := homeCardSubtitle(card)
		if len(s) > 42 {
			s = s[:39] + "..."
		}

		artWidthSmall := 16
		artHeightSmall := 8
		art := lipgloss.NewStyle().Width(artWidthSmall).Height(artHeightSmall).Render("")
		if len(card.Thumbnails) > 0 {
			art = m.cachedArtAt(card.Thumbnails[0].URL, artWidthSmall, artHeightSmall)
		}

		titleColor := colorText
		focused := m.focusedHomeCard(index, cardIndex)
		bg := colorBg
		if focused {
			bg = colorFocusBg
			titleColor = colorAccent
		}

		textStyle := lipgloss.NewStyle().PaddingLeft(2).PaddingTop(0)
		textContent := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Render(t),
			lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(s),
		)

		content := lipgloss.JoinHorizontal(lipgloss.Top,
			art, textStyle.Render(textContent),
		)

		style := lipgloss.NewStyle().Padding(1, 2).Width(cardWidth).Background(bg).MarginRight(2)
		styledContent := style.Render(content)
		if zid := entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID); zid != "" {
			styledContent = m.zone.Mark(zid, styledContent)
		}
		blocks = append(blocks, styledContent)
	}

	row.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	return row.String() + "\n\n\n"
}

func (m Model) renderGrid(title string, cards []ytmapi.HomeCarouselItem, mainWidth int) string {
	var grid strings.Builder
	cardWidth := artWidth
	
	if mainWidth < 22 {
		return ""
	}
	
	cols := mainWidth / (cardWidth + 2)
	if cols < 1 {
		cols = 1
	}
	
	var blocks []string
	for i, card := range cards {
		t := card.Title
		if len(t) > 20 {
			t = t[:17] + "..."
		}
		if card.IsExplicit {
			t += explicitBadge()
		}
		s := homeCardSubtitle(card)
		if len(s) > 22 {
			s = s[:19] + "..."
		}

		art := artPlaceholder()
		if len(card.Thumbnails) > 0 {
			art = m.cachedArtAt(card.Thumbnails[0].URL, artWidth, artHeight)
		}

		titleColor := colorText
		focused := false
		if m.activePane == PaneMain && i == m.listCursor {
			focused = true
		}

		if focused {
			titleColor = colorAccent
		}

		content := lipgloss.JoinVertical(lipgloss.Left,
			art, "",
			lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(t),
			lipgloss.NewStyle().Foreground(colorSubtext).Render(s),
		)

		zid := card.ZoneID
		if zid == "" {
			zid = entityZoneID(card.VideoID, card.BrowseID, card.PlaylistID)
		}
		if zid != "" {
			content = m.zone.Mark(zid, content)
		}

		style := lipgloss.NewStyle().Padding(0, 1).Width(cardWidth + 2)
		blocks = append(blocks, style.Render(content))
	}

	for i := 0; i < len(blocks); i += cols {
		end := i + cols
		if end > len(blocks) {
			end = len(blocks)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, blocks[i:end]...)
		grid.WriteString(row)
		grid.WriteString("\n\n")
	}

	return grid.String()
}
