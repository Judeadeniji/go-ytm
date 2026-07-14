package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

type Pane int

const (
	PaneSidebar Pane = iota
	PaneSearch
	PaneMain
	PaneQueue
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
	Type        SuggestionType
	Text        string
	Runs        []ytmapi.SuggestionRun
	FromHistory bool
	Subtext     string
	ThumbURL    string
	VideoID     string
	BrowseID    string
	PlaylistID  string
	ResultType  string
}

type StreamURLMsg struct {
	URL string
	Err error
}
