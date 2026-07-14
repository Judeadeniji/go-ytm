package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func buildSuggestionList(msg SearchSuggestionsMsg) []SearchSuggestion {
	var out []SearchSuggestion
	for _, s := range msg.Suggestions {
		typ := SuggestionQuery
		if s.FromHistory {
			typ = SuggestionHistory
		}
		out = append(out, SearchSuggestion{
			Type:        typ,
			Text:        s.Text,
			Runs:        s.Runs,
			FromHistory: s.FromHistory,
		})
	}

	// Prefer YouTube Music "Top result" hits, then the rest (capped).
	ranked := append([]ytmapi.SearchResult(nil), msg.Results...)
	sort.SliceStable(ranked, func(i, j int) bool {
		ti := strings.EqualFold(ranked[i].Category, "Top result")
		tj := strings.EqualFold(ranked[j].Category, "Top result")
		if ti == tj {
			return false
		}
		return ti && !tj
	})
	const maxEntities = 5
	n := 0
	for _, res := range ranked {
		if n >= maxEntities {
			break
		}
		if searchResultZone(res) == "" || strings.TrimSpace(res.Title) == "" {
			continue
		}
		out = append(out, SearchSuggestion{
			Type:       SuggestionEntity,
			Text:       res.Title,
			Subtext:    suggestionEntitySubtext(res),
			ThumbURL:   thumbURL(res.Thumbnails),
			VideoID:    res.VideoID,
			BrowseID:   suggestionBrowseID(res),
			PlaylistID: res.PlaylistID,
			ResultType: res.ResultType,
		})
		n++
	}
	return out
}

func suggestionBrowseID(res ytmapi.SearchResult) string {
	if res.BrowseID != "" {
		return res.BrowseID
	}
	if len(res.Artists) > 0 {
		return res.Artists[0].ID
	}
	return ""
}

func suggestionEntitySubtext(res ytmapi.SearchResult) string {
	var parts []string
	if res.Type != "" {
		parts = append(parts, res.Type)
	} else if res.ResultType != "" {
		parts = append(parts, titleCase(res.ResultType))
	}
	if len(res.Artists) > 0 {
		parts = append(parts, res.Artists[0].Name)
	} else if res.Artist != "" {
		parts = append(parts, res.Artist)
	} else if res.Author != "" {
		parts = append(parts, res.Author)
	}
	if res.Album.Name != "" {
		parts = append(parts, res.Album.Name)
	}
	if res.Year != "" {
		parts = append(parts, res.Year)
	}
	if res.Duration != "" {
		parts = append(parts, res.Duration)
	}
	return strings.Join(parts, " · ")
}

func (m Model) renderSuggestionsModal(width, maxHeight int) string {
	var sb strings.Builder
	inner := width - 4
	if inner < 20 {
		inner = 20
	}

	wroteText := false
	wroteEntities := false
	for i, s := range m.searchSuggestions {
		if s.Type == SuggestionEntity {
			if !wroteEntities {
				if wroteText {
					sb.WriteString("\n")
				}
				sb.WriteString(lipgloss.NewStyle().
					Foreground(colorSubtext).Bold(true).
					Render("Top results"))
				sb.WriteString("\n\n")
				wroteEntities = true
			}
			sb.WriteString(m.renderSuggestionEntity(i, s, inner))
			sb.WriteString("\n")
			continue
		}
		if !wroteText {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(colorSubtext).Bold(true).
				Render("Suggestions"))
			sb.WriteString("\n\n")
			wroteText = true
		}
		sb.WriteString(m.renderSuggestionText(i, s, inner))
		sb.WriteString("\n")
	}

	if len(m.searchSuggestions) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("Type to search…"))
	}

	body := strings.TrimRight(sb.String(), "\n")
	style := lipgloss.NewStyle().
		Background(colorSearchBg).
		Width(width).
		Padding(1, 2).
		// Top border separates the dropdown from the search textbox above.
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorDivider).
		BorderBackground(colorSearchBg)
	if maxHeight > 2 {
		style = style.MaxHeight(maxHeight)
	}
	return style.Render(body)
}

func (m Model) renderSuggestionText(i int, s SearchSuggestion, inner int) string {
	focused := m.listCursor == i
	bg := colorSearchBg
	if focused {
		bg = colorFocusBg
	}

	icon := "\ue0e3" // search
	if s.FromHistory || s.Type == SuggestionHistory {
		icon = "\ue292" // history
	}
	prefix := "  "
	if focused {
		prefix = "› "
	}

	var textBuilder strings.Builder
	if len(s.Runs) > 0 {
		for _, run := range s.Runs {
			// detailed_runs: typed match is Bold — emphasize it.
			if run.Bold {
				textBuilder.WriteString(lipgloss.NewStyle().
					Bold(true).Foreground(colorText).Background(bg).Render(run.Text))
			} else {
				textBuilder.WriteString(lipgloss.NewStyle().
					Foreground(colorSubtext).Background(bg).Render(run.Text))
			}
		}
	} else {
		textBuilder.WriteString(lipgloss.NewStyle().
			Foreground(colorText).Background(bg).Render(s.Text))
	}

	iconStyle := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).PaddingRight(1)
	row := lipgloss.JoinHorizontal(lipgloss.Top,
		iconStyle.Render(prefix+icon),
		textBuilder.String(),
	)
	row = lipgloss.NewStyle().Background(bg).Width(inner).MaxWidth(inner).Render(row)
	return m.zone.Mark(fmt.Sprintf("suggestion_%d", i), row)
}

func (m Model) renderSuggestionEntity(i int, s SearchSuggestion, inner int) string {
	focused := m.listCursor == i
	bg := colorSearchBg
	if focused {
		bg = colorFocusBg
	}

	art := m.cachedArtAt(s.ThumbURL, sugArtWidth, sugArtHeight)
	titleColor := colorText
	prefix := "  "
	if focused {
		titleColor = colorAccent
		prefix = "› "
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).
		MaxWidth(inner - sugArtWidth - 4).
		Render(prefix + s.Text)
	sub := lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).
		MaxWidth(inner - sugArtWidth - 4).
		Render(s.Subtext)
	info := lipgloss.JoinVertical(lipgloss.Left, title, sub)
	row := lipgloss.JoinHorizontal(lipgloss.Top, art, "  ", info)
	row = lipgloss.NewStyle().Background(bg).Width(inner).MaxWidth(inner).Padding(0, 0, 1, 0).Render(row)
	return m.zone.Mark(fmt.Sprintf("suggestion_%d", i), row)
}

func (m Model) activateSuggestion() (Model, tea.Cmd) {
	if m.listCursor < 0 || m.listCursor >= len(m.searchSuggestions) {
		query := m.searchInput.Value()
		m.lastSearchQuery = query
		m.statusMsg = "Searching for: " + query
		m.searchInput.Blur()
		m.markSessionDirty()
		return m, doSearchFiltered(m.ytmapiClient, query, m.searchFilter)
	}
	s := m.searchSuggestions[m.listCursor]
	m.searchInput.Blur()

	if s.Type == SuggestionEntity {
		m.statusMsg = s.Text
		m.markSessionDirty()
		zid := entityZoneID(s.VideoID, s.BrowseID, s.PlaylistID)
		if zid == "" {
			return m, nil
		}
		mm, cmd, _ := m.dispatchZone(zid, s.Text, entityArtistHint(s), s.ThumbURL)
		return mm, cmd
	}

	m.searchInput.SetValue(s.Text)
	m.lastSearchQuery = s.Text
	m.statusMsg = "Searching for: " + s.Text
	m.markSessionDirty()
	return m, doSearchFiltered(m.ytmapiClient, s.Text, m.searchFilter)
}

func entityArtistHint(s SearchSuggestion) string {
	// Subtext often starts with type · artist — take last artist-ish chunk loosely.
	parts := strings.Split(s.Subtext, " · ")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func (m Model) enqueueSuggestionImages() tea.Cmd {
	var cmds []tea.Cmd
	seen := map[string]struct{}{}
	for _, s := range m.searchSuggestions {
		if s.Type != SuggestionEntity || s.ThumbURL == "" {
			continue
		}
		key := imageCacheKey(s.ThumbURL, sugArtWidth, sugArtHeight)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if _, ok := m.imageCache[key]; ok {
			continue
		}
		ph := KittyImage{Spacer: sizedPlaceholder(sugArtWidth, sugArtHeight)}
		m.imageCache[key] = &ph
		cmds = append(cmds, fetchImageSized(s.ThumbURL, sugArtWidth, sugArtHeight))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
