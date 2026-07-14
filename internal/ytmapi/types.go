package ytmapi

// Thumbnail is a shared image size entry from ytmusicapi responses.
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// NamedRef is an artist/album/author reference with optional browse/channel id.
type NamedRef struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

// SearchResult represents a generic item from ytmusicapi search.
type SearchResult struct {
	Category   string `json:"category"`
	ResultType string `json:"resultType"`
	Title      string `json:"title"`
	VideoID    string `json:"videoId,omitempty"`
	BrowseID   string `json:"browseId,omitempty"`
	PlaylistID string `json:"playlistId,omitempty"`
	// Type holds Album/Single/EP for album-like results (resultType is still "album").
	Type    string     `json:"type,omitempty"`
	Artists []NamedRef `json:"artists,omitempty"`
	Artist  string     `json:"artist,omitempty"`
	Author  string     `json:"author,omitempty"`
	Duration  string      `json:"duration,omitempty"`
	Views     string      `json:"views,omitempty"`
	Year      string      `json:"year,omitempty"`
	ItemCount string      `json:"itemCount,omitempty"`
	Album     NamedRef    `json:"album,omitempty"`
	VideoType string      `json:"videoType,omitempty"`
	Thumbnails []Thumbnail `json:"thumbnails,omitempty"`
}

type SearchSuggestionItem struct {
	Text          string          `json:"text"`
	Runs          []SuggestionRun `json:"runs,omitempty"`
	FromHistory   bool            `json:"fromHistory,omitempty"`
	FeedbackToken *string         `json:"feedbackToken,omitempty"` // null from ytmusicapi
}

type SuggestionRun struct {
	Text string `json:"text"`
	Bold bool   `json:"bold,omitempty"`
}

type HomeCarouselItem struct {
	Title       string      `json:"title"`
	VideoID     string      `json:"videoId,omitempty"`
	PlaylistID  string      `json:"playlistId,omitempty"`
	BrowseID    string      `json:"browseId,omitempty"`
	Description string      `json:"description,omitempty"`
	Artists     []NamedRef  `json:"artists,omitempty"`
	Album       any         `json:"album,omitempty"` // string or {name,id}
	Year        string      `json:"year,omitempty"`
	Views       string      `json:"views,omitempty"`
	Subscribers string      `json:"subscribers,omitempty"`
	Count       string      `json:"count,omitempty"`
	Author      any         `json:"author,omitempty"` // string or {name,id}
	Thumbnails  []Thumbnail `json:"thumbnails,omitempty"`
}

type HomeCarousel struct {
	Title    string             `json:"title"`
	Contents []HomeCarouselItem `json:"contents"`
}

// TrackItem is a playable row shared across album/playlist/watch/artist songs.
type TrackItem struct {
	VideoID   string      `json:"videoId,omitempty"`
	Title     string      `json:"title"`
	Artists   []NamedRef  `json:"artists,omitempty"`
	Artist    string      `json:"artist,omitempty"` // artist songs section often uses flat string
	Album     any         `json:"album,omitempty"` // string or NamedRef
	Duration  string      `json:"duration,omitempty"`
	Length    string      `json:"length,omitempty"` // watch playlist
	Views     string      `json:"views,omitempty"`
	VideoType string      `json:"videoType,omitempty"`
	Year      string      `json:"year,omitempty"`
	IsExplicit bool       `json:"isExplicit,omitempty"`
	Thumbnails []Thumbnail `json:"thumbnails,omitempty"`
	Thumbnail  []Thumbnail `json:"thumbnail,omitempty"` // watch uses singular key
	PlaylistID string      `json:"playlistId,omitempty"`
	BrowseID   string      `json:"browseId,omitempty"`
}

// ArtistSection is one content block on an artist page (songs, albums, …).
type ArtistSection struct {
	BrowseID string           `json:"browseId,omitempty"`
	Params   string           `json:"params,omitempty"`
	Results  []map[string]any `json:"results,omitempty"`
}

// ArtistPage is the get_artist response.
type ArtistPage struct {
	Name             string         `json:"name"`
	ChannelID        string         `json:"channelId,omitempty"`
	Description      string         `json:"description,omitempty"`
	Views            string         `json:"views,omitempty"`
	Subscribers      string         `json:"subscribers,omitempty"`
	MonthlyListeners string         `json:"monthlyListeners,omitempty"`
	Thumbnails       []Thumbnail    `json:"thumbnails,omitempty"`
	ShuffleID        string         `json:"shuffleId,omitempty"`
	RadioID          string         `json:"radioId,omitempty"`
	Songs            *ArtistSection `json:"songs,omitempty"`
	Albums           *ArtistSection `json:"albums,omitempty"`
	Singles          *ArtistSection `json:"singles,omitempty"`
	Videos           *ArtistSection `json:"videos,omitempty"`
	Related          *ArtistSection `json:"related,omitempty"`
}

// ArtistAlbum is an entry from get_artist_albums / artist albums section.
type ArtistAlbum struct {
	Title      string      `json:"title"`
	BrowseID   string      `json:"browseId,omitempty"`
	PlaylistID string      `json:"playlistId,omitempty"`
	Type       string      `json:"type,omitempty"`
	Year       string      `json:"year,omitempty"`
	Thumbnails []Thumbnail `json:"thumbnails,omitempty"`
}

// AlbumPage is the get_album response (also used for Single/EP).
type AlbumPage struct {
	Title           string      `json:"title"`
	Type            string      `json:"type,omitempty"`
	Year            string      `json:"year,omitempty"`
	TrackCount      int         `json:"trackCount,omitempty"`
	Duration        string      `json:"duration,omitempty"`
	Description     string      `json:"description,omitempty"`
	AudioPlaylistID string      `json:"audioPlaylistId,omitempty"`
	Artists         []NamedRef  `json:"artists,omitempty"`
	Thumbnails      []Thumbnail `json:"thumbnails,omitempty"`
	Tracks          []TrackItem `json:"tracks,omitempty"`
}

// PlaylistPage is the get_playlist response.
type PlaylistPage struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Year        string      `json:"year,omitempty"`
	Duration    string      `json:"duration,omitempty"`
	TrackCount  int         `json:"trackCount,omitempty"`
	Privacy     string      `json:"privacy,omitempty"`
	Author      any         `json:"author,omitempty"`
	Thumbnails  []Thumbnail `json:"thumbnails,omitempty"`
	Tracks      []TrackItem `json:"tracks,omitempty"`
}

// WatchPlaylist is the get_watch_playlist response.
type WatchPlaylist struct {
	Tracks     []TrackItem `json:"tracks"`
	PlaylistID string      `json:"playlistId,omitempty"`
	Lyrics     string      `json:"lyrics,omitempty"`
	Related    string      `json:"related,omitempty"`
}
