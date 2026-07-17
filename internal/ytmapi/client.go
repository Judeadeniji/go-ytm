package ytmapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const maxAPIBody = 8 << 20 // 8 MiB

// Client is a thin HTTP client for the Python ytm-api.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func NewClient() *Client {
	socketPath := os.Getenv("YTM_API_SOCK")
	if socketPath == "" {
		socketPath = os.ExpandEnv("${HOME}/.local/state/go-ytm/ytm-api.sock")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	return &Client{
		baseURL: "http://localhost",
		httpClient: &http.Client{
			Timeout:   45 * time.Second,
			Transport: transport,
		},
		token: os.Getenv("YTM_API_TOKEN"),
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
	if c.token != "" {
		req.Header.Set("X-API-Token", c.token)
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

func (c *Client) GetExplore(ctx context.Context) (*ExploreData, error) {
	var data ExploreData
	if err := c.getJSON(ctx, "/explore", &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) GetMoodCategories(ctx context.Context) (map[string][]MoodCategory, error) {
	var data struct {
		MoodCategories map[string][]MoodCategory `json:"moodCategories"`
	}
	if err := c.getJSON(ctx, "/explore/moods", &data); err != nil {
		return nil, err
	}
	return data.MoodCategories, nil
}

func (c *Client) GetMoodPlaylists(ctx context.Context, params string) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("params", params)
	var data struct {
		Playlists []map[string]any `json:"playlists"`
	}
	if err := c.getJSON(ctx, "/explore/moods/playlists?"+q.Encode(), &data); err != nil {
		return nil, err
	}
	return data.Playlists, nil
}

func (c *Client) GetCharts(ctx context.Context, country string) (*ChartsData, error) {
	q := url.Values{}
	if country != "" {
		q.Set("country", country)
	}
	var data ChartsData
	if err := c.getJSON(ctx, "/explore/charts?"+q.Encode(), &data); err != nil {
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

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	u := c.baseURL + path
	
	importBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("ytm-api marshal: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(importBytes)))
	if err != nil {
		return fmt.Errorf("ytm-api request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-API-Token", c.token)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ytm-api unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIBody+1))
	if err != nil {
		return fmt.Errorf("ytm-api read: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Detail string `json:"detail"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errBody)
		msg := errBody.Detail
		if msg == "" { msg = errBody.Error }
		if msg == "" { msg = string(respBody) }
		if len(msg) > 200 { msg = msg[:200] }
		return fmt.Errorf("ytm-api %d: %s", resp.StatusCode, msg)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("ytm-api decode: %w", err)
		}
	}
	return nil
}

type authRequest struct {
	HeadersRaw string `json:"headers_raw"`
}

func (c *Client) AuthSetup(ctx context.Context, headersRaw string) error {
	return c.postJSON(ctx, "/auth/setup", authRequest{HeadersRaw: headersRaw}, nil)
}

type libraryPlaylistsResponse struct {
	Playlists []map[string]any `json:"playlists"`
}

func (c *Client) GetLibraryPlaylists(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 { limit = 50 }
	var data libraryPlaylistsResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/library/playlists?limit=%d", limit), &data); err != nil {
		return nil, err
	}
	return data.Playlists, nil
}

type librarySongsResponse struct {
	Songs []map[string]any `json:"songs"`
}

func (c *Client) GetLibrarySongs(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 { limit = 100 }
	var data librarySongsResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/library/songs?limit=%d", limit), &data); err != nil {
		return nil, err
	}
	return data.Songs, nil
}

type libraryAlbumsResponse struct {
	Albums []map[string]any `json:"albums"`
}

func (c *Client) GetLibraryAlbums(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 { limit = 100 }
	var data libraryAlbumsResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/library/albums?limit=%d", limit), &data); err != nil {
		return nil, err
	}
	return data.Albums, nil
}

type libraryArtistsResponse struct {
	Artists []map[string]any `json:"artists"`
}

func (c *Client) GetLibraryArtists(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 { limit = 100 }
	var data libraryArtistsResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/library/artists?limit=%d", limit), &data); err != nil {
		return nil, err
	}
	return data.Artists, nil
}

type OAuthCodeRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type OAuthCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (c *Client) OAuthCode(ctx context.Context, clientID, clientSecret string) (*OAuthCodeResponse, error) {
	req := OAuthCodeRequest{ClientID: clientID, ClientSecret: clientSecret}
	var data OAuthCodeResponse
	if err := c.postJSON(ctx, "/auth/oauth/code", req, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

type OAuthTokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	DeviceCode   string `json:"device_code"`
}

type OAuthTokenResponse struct {
	Status string `json:"status"` // "pending" or "ok"
}

func (c *Client) OAuthToken(ctx context.Context, clientID, clientSecret, deviceCode string) (*OAuthTokenResponse, error) {
	req := OAuthTokenRequest{ClientID: clientID, ClientSecret: clientSecret, DeviceCode: deviceCode}
	var data OAuthTokenResponse
	if err := c.postJSON(ctx, "/auth/oauth/token", req, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) GetProfile(ctx context.Context) (*UserProfile, error) {
	var data UserProfile
	if err := c.getJSON(ctx, "/auth/profile", &data); err != nil {
		return nil, err
	}
	return &data, nil
}
