package tui

import "github.com/charmbracelet/lipgloss"

type Pane int

const (
	PaneSidebar Pane = iota
	PaneSearch
	PaneMain
)

type AlbumCard struct {
	Title    string
	Subtitle string
	ArtColor lipgloss.Color
	VideoID  string
}

type SuggestionType int

const (
	SuggestionHistory SuggestionType = iota
	SuggestionQuery
	SuggestionEntity
)

type SearchSuggestion struct {
	Type    SuggestionType
	Text    string
	Subtext string
	Image   string // pre-rendered ANSI image
}

type StreamURLMsg struct {
	URL string
	Err error
}
