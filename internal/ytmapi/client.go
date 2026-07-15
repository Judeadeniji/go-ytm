package ytmapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxAPIBody = 8 << 20 // 8 MiB

// Client is a thin HTTP client for the Python ytm-api.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL: "http://127.0.0.1:8000",
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("ytm-api request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ytm-api unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIBody+1))
	if err != nil {
		return fmt.Errorf("ytm-api read: %w", err)
	}
	if len(body) > maxAPIBody {
		return fmt.Errorf("ytm-api response too large")
	}

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Detail string `json:"detail"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal(body, &errBody)
		msg := errBody.Detail
		if msg == "" {
			msg = errBody.Error
		}
		if msg == "" {
			msg = string(body)
		}
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("ytm-api %d: %s", resp.StatusCode, msg)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("ytm-api decode: %w", err)
	}
	return nil
}

type searchResponse struct {
	Results []SearchResult `json:"results"`
}

func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	return c.SearchFiltered(ctx, query, "", 20)
}

func (c *Client) SearchFiltered(ctx context.Context, query, filter string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	if filter != "" && filter != "all" {
		q.Set("filter", filter)
	}
	var data searchResponse
	if err := c.getJSON(ctx, "/search?"+q.Encode(), &data); err != nil {
		return nil, err
	}
	return data.Results, nil
}

type suggestionsResponse struct {
	Suggestions []SearchSuggestionItem `json:"suggestions"`
}

func (c *Client) GetSearchSuggestions(ctx context.Context, query string) ([]SearchSuggestionItem, error) {
	if query == "" {
		return []SearchSuggestionItem{}, nil
	}
	var data suggestionsResponse
	if err := c.getJSON(ctx, "/suggestions?q="+url.QueryEscape(query), &data); err != nil {
		return nil, err
	}
	return data.Suggestions, nil
}

type homeResponse struct {
	Carousels []HomeCarousel `json:"carousels"`
}

func (c *Client) GetHome(ctx context.Context) ([]HomeCarousel, error) {
	var data homeResponse
	if err := c.getJSON(ctx, "/home?limit=20", &data); err != nil {
		return nil, err
	}
	return data.Carousels, nil
}

func (c *Client) GetArtist(ctx context.Context, channelID string) (*ArtistPage, error) {
	var data ArtistPage
	if err := c.getJSON(ctx, "/artist/"+url.PathEscape(channelID), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

type artistAlbumsResponse struct {
	Albums []ArtistAlbum `json:"albums"`
}

func (c *Client) GetArtistAlbums(ctx context.Context, channelID, params string, limit int) ([]ArtistAlbum, error) {
	if limit <= 0 {
		limit = 100
	}
	q := url.Values{}
	q.Set("params", params)
	q.Set("limit", fmt.Sprintf("%d", limit))
	var data artistAlbumsResponse
	path := "/artist/" + url.PathEscape(channelID) + "/albums?" + q.Encode()
	if err := c.getJSON(ctx, path, &data); err != nil {
		return nil, err
	}
	return data.Albums, nil
}

func (c *Client) GetAlbum(ctx context.Context, browseID string) (*AlbumPage, error) {
	var data AlbumPage
	if err := c.getJSON(ctx, "/album/"+url.PathEscape(browseID), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

type albumBrowseIDResponse struct {
	BrowseID string `json:"browseId"`
}

func (c *Client) GetAlbumBrowseID(ctx context.Context, audioPlaylistID string) (string, error) {
	q := url.Values{}
	q.Set("audioPlaylistId", audioPlaylistID)
	var data albumBrowseIDResponse
	if err := c.getJSON(ctx, "/album/browse-id?"+q.Encode(), &data); err != nil {
		return "", err
	}
	return data.BrowseID, nil
}

func (c *Client) GetPlaylist(ctx context.Context, playlistID string, limit int) (*PlaylistPage, error) {
	if limit <= 0 {
		limit = 100
	}
	path := fmt.Sprintf("/playlist/%s?limit=%d", url.PathEscape(playlistID), limit)
	var data PlaylistPage
	if err := c.getJSON(ctx, path, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) GetSong(ctx context.Context, videoID string) (*SongDetails, error) {
	if videoID == "" {
		return nil, fmt.Errorf("videoId required")
	}
	var data SongDetails
	if err := c.getJSON(ctx, "/song/"+url.PathEscape(videoID), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) GetWatchPlaylist(ctx context.Context, videoID, playlistID string, radio bool, limit int) (*WatchPlaylist, error) {
	if limit <= 0 {
		limit = 25
	}
	q := url.Values{}
	if videoID != "" {
		q.Set("videoId", videoID)
	}
	if playlistID != "" {
		q.Set("playlistId", playlistID)
	}
	if radio {
		q.Set("radio", "true")
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	var data WatchPlaylist
	if err := c.getJSON(ctx, "/watch?"+q.Encode(), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ArtistName returns a display string from artists slice or flat artist field.
func (t TrackItem) ArtistName() string {
	if len(t.Artists) > 0 {
		return t.Artists[0].Name
	}
	return t.Artist
}

// DurationLabel prefers duration, then length (watch).
func (t TrackItem) DurationLabel() string {
	if t.Duration != "" {
		return t.Duration
	}
	return t.Length
}

// ThumbURL returns the first available thumbnail URL.
func (t TrackItem) ThumbURL() string {
	if len(t.Thumbnails) > 0 {
		return t.Thumbnails[0].URL
	}
	if len(t.Thumbnail) > 0 {
		return t.Thumbnail[0].URL
	}
	return ""
}

// AuthorName unwraps playlist author string or object.
func AuthorName(author any) string {
	switch v := author.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return ""
}

// AlbumRef unwraps TrackItem.Album as name + optional browse id.
func AlbumRef(album any) (name, id string) {
	switch v := album.(type) {
	case string:
		return v, ""
	case map[string]any:
		if n, ok := v["name"].(string); ok {
			name = n
		}
		if i, ok := v["id"].(string); ok {
			id = i
		}
		return name, id
	}
	return "", ""
}

// ArtistChannelID returns the first artist browse/channel id when present.
func (t TrackItem) ArtistChannelID() string {
	if len(t.Artists) > 0 {
		return t.Artists[0].ID
	}
	return ""
}
