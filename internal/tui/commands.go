package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
)

func fetchStreamURL(ext *search.Extractor, videoID string) tea.Cmd {
	return func() tea.Msg {
		url, err := ext.GetStreamURL(context.Background(), videoID)
		return StreamURLMsg{URL: url, Err: err}
	}
}

func stopPlayback(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		p.Stop()
		return nil
	}
}

func loadAndPlay(p *player.Player, url string) tea.Cmd {
	return func() tea.Msg {
		p.Load(url)
		p.Play()
		return nil
	}
}

type SearchResultsMsg struct {
	Results []ytmapi.SearchResult
	Err     error
}

func doSearch(apiClient *ytmapi.Client, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := apiClient.Search(query)
		return SearchResultsMsg{Results: results, Err: err}
	}
}

type SearchSuggestionsMsg struct {
	Suggestions []ytmapi.SearchSuggestionItem
	Err         error
}

func fetchSuggestions(apiClient *ytmapi.Client, query string) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := apiClient.GetSearchSuggestions(query)
		return SearchSuggestionsMsg{Suggestions: suggestions, Err: err}
	}
}

type HomeMsg struct {
	Carousels []ytmapi.HomeCarousel
	Err       error
}

func fetchHome(apiClient *ytmapi.Client) tea.Cmd {
	return func() tea.Msg {
		carousels, err := apiClient.GetHome()
		return HomeMsg{Carousels: carousels, Err: err}
	}
}

type ImageLoadedMsg struct {
	URL  string
	ANSI string
}

func fetchImage(url string) tea.Cmd {
	return func() tea.Msg {
		// render at 24x10
		ansi := RenderRemoteImage(url, 24, 10)
		return ImageLoadedMsg{URL: url, ANSI: ansi}
	}
}
