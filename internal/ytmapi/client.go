package ytmapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is a thin HTTP client for the Python ytm-api
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL: "http://127.0.0.1:8000",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SearchResult represents a generic item from the ytmusicapi search
type SearchResult struct {
	Category   string `json:"category"`
	ResultType string `json:"resultType"`
	Title      string `json:"title"`
	VideoID    string `json:"videoId,omitempty"`
	BrowseID   string `json:"browseId,omitempty"`
	Artists    []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"artists,omitempty"`
	Artist     string `json:"artist,omitempty"`
	Author     string `json:"author,omitempty"`
	Duration   string `json:"duration,omitempty"`
	Views      string `json:"views,omitempty"`
	Year       string `json:"year,omitempty"`
	ItemCount  string `json:"itemCount,omitempty"`
	Album      struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"album,omitempty"`
	VideoType  string `json:"videoType,omitempty"`
	Thumbnails []struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"thumbnails,omitempty"`
}

type searchResponse struct {
	Results []SearchResult `json:"results"`
}

func (c *Client) Search(query string) ([]SearchResult, error) {
	u := fmt.Sprintf("%s/search?q=%s", c.baseURL, url.QueryEscape(query))
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to call ytm-api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ytm-api returned status %d", resp.StatusCode)
	}

	var data searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode ytm-api response: %w", err)
	}

	return data.Results, nil
}

type SuggestionRun struct {
	Text string `json:"text"`
	Bold bool   `json:"bold,omitempty"`
}

type SearchSuggestionItem struct {
	Text        string          `json:"text"`
	Runs        []SuggestionRun `json:"runs,omitempty"`
	FromHistory bool            `json:"fromHistory,omitempty"`
}

type suggestionsResponse struct {
	Suggestions []SearchSuggestionItem `json:"suggestions"`
}

func (c *Client) GetSearchSuggestions(query string) ([]SearchSuggestionItem, error) {
	if query == "" {
		return []SearchSuggestionItem{}, nil
	}
	u := fmt.Sprintf("%s/suggestions?q=%s", c.baseURL, url.QueryEscape(query))
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to call ytm-api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ytm-api returned status %d", resp.StatusCode)
	}

	var data suggestionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode ytm-api response: %w", err)
	}

	return data.Suggestions, nil
}

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type HomeCarouselItem struct {
	Title       string      `json:"title"`
	VideoID     string      `json:"videoId,omitempty"`
	PlaylistID  string      `json:"playlistId,omitempty"`
	BrowseID    string      `json:"browseId,omitempty"`
	Description string      `json:"description,omitempty"`
	Thumbnails  []Thumbnail `json:"thumbnails,omitempty"`
}

type HomeCarousel struct {
	Title    string             `json:"title"`
	Contents []HomeCarouselItem `json:"contents"`
}

type homeResponse struct {
	Carousels []HomeCarousel `json:"carousels"`
}

func (c *Client) GetHome() ([]HomeCarousel, error) {
	u := fmt.Sprintf("%s/home?limit=20", c.baseURL)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to call ytm-api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ytm-api returned status %d", resp.StatusCode)
	}

	var data homeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode ytm-api response: %w", err)
	}

	return data.Carousels, nil
}
