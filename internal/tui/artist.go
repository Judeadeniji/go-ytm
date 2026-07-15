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

	title := lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(a.Name)
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
	meta := lipgloss.NewStyle().Foreground(colorSubtext).Render(strings.Join(metaParts, "  ·  "))

	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(meta)
	sb.WriteString("\n\n")

	if a.Description != "" {
		desc := a.Description
		if len(desc) > 280 {
			desc = desc[:277] + "..."
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Width(mainWidth - 4).Render(desc))
		sb.WriteString("\n\n")
	}

	writeSection := func(label string, results []map[string]any, kind string) {
		if len(results) == 0 {
			return
		}
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(label))
		sb.WriteString("\n\n")
		for i, item := range results {
			if i >= 12 {
				break
			}
			rowTitle := mapStr(item, "title")
			if rowTitle == "" {
				rowTitle = mapStr(item, "artist")
			}
			sub := ""
			switch kind {
			case "song":
				sub = artistRefName(item["album"])
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
			case "album":
				sub = mapStr(item, "year")
				t := mapStr(item, "type")
				if t != "" {
					if sub != "" {
						sub = t + " · " + sub
					} else {
						sub = t
					}
				}
			case "video":
				sub = ytmapi.FormatCount(mapStr(item, "views"))
				if sub == "" {
					sub = "Video"
				} else {
					sub += " plays"
				}
			case "related":
				sub = mapStr(item, "subscribers")
				if sub == "" {
					sub = "Artist"
				}
			}

			zoneID := artistItemZone(kind, item)
			focused := m.focusedArtistZone(zoneID)
			bg := colorBg
			titleColor := colorText
			prefix := "  "
			if focused {
				bg = colorFocusBg
				titleColor = colorAccent
				prefix = "› "
			}

			line := lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Bold(true).Foreground(titleColor).Background(bg).Width(mainWidth/2).Render(prefix+rowTitle),
				lipgloss.NewStyle().Foreground(colorSubtext).Background(bg).Render(sub),
			)
			line = lipgloss.NewStyle().Background(bg).Width(mainWidth - 4).Render(line)

			if zoneID != "" {
				line = m.zone.Mark(zoneID, line)
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if a.Songs != nil {
		writeSection("Songs", a.Songs.Results, "song")
	}
	if a.Albums != nil {
		writeSection("Albums", a.Albums.Results, "album")
	}
	if a.Singles != nil {
		writeSection("Singles & EPs", a.Singles.Results, "album")
	}
	if a.Videos != nil {
		writeSection("Videos", a.Videos.Results, "video")
	}
	if a.Related != nil {
		writeSection("Fans also like", a.Related.Results, "related")
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
