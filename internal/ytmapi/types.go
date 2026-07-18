package ytmapi

import "strings"

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

type UserProfile struct {
	Name  string `json:"name"`
	Photo string `json:"photo"`
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
	IsExplicit bool       `json:"isExplicit,omitempty"`
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
	ZoneID      string      `json:"-"`
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
	IsExplicit  bool        `json:"isExplicit,omitempty"`
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
	Views       any         `json:"views,omitempty"` // int or null from ytmusicapi
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

// CreditSection is one credits block (Performed by / Written by / …).
type CreditSection struct {
	LocalizedTitle string   `json:"localized_title"`
	Data           []string `json:"data"`
}

// SongCredits is get_song_credits output (names only).
type SongCredits struct {
	PerformedBy             *CreditSection  `json:"performed_by,omitempty"`
	WrittenBy               *CreditSection  `json:"written_by,omitempty"`
	ProducedBy              *CreditSection  `json:"produced_by,omitempty"`
	MusicMetadataProvidedBy *CreditSection  `json:"music_metadata_provided_by,omitempty"`
	OtherSections           []CreditSection `json:"other_sections,omitempty"`
}

// SongDetails is catalog-style metadata for the currently playing song.
// Artists/album are resolved names; IDs are for navigation only.
type SongDetails struct {
	VideoID         string       `json:"videoId"`
	Title           string       `json:"title"`
	Artists         []NamedRef   `json:"artists,omitempty"`
	Album           *NamedRef    `json:"album,omitempty"`
	Year            string       `json:"year,omitempty"`
	Duration        string       `json:"duration,omitempty"`
	DurationSeconds int          `json:"durationSeconds,omitempty"`
	IsExplicit      bool         `json:"isExplicit,omitempty"`
	TrackNumber     *int         `json:"trackNumber,omitempty"`
	AlbumType       string       `json:"albumType,omitempty"`
	AlbumTrackCount int          `json:"albumTrackCount,omitempty"`
	LikeStatus      string       `json:"likeStatus,omitempty"`
	VideoType       string       `json:"videoType,omitempty"`
	Thumbnails      []Thumbnail  `json:"thumbnails,omitempty"`
	Credits         *SongCredits `json:"credits,omitempty"`
}

// ArtistNames joins artist display names.
func (s *SongDetails) ArtistNames() string {
	if s == nil || len(s.Artists) == 0 {
		return ""
	}
	names := make([]string, 0, len(s.Artists))
	for _, a := range s.Artists {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return strings.Join(names, ", ")
}

// ── Explore Types ────────────────────────────────────────────────────────────

// MoodCategory represents a single mood or genre tile.
type MoodCategory struct {
	Title  string `json:"title"`
	Params string `json:"params"`
}

// ChartArtist represents an artist in the charts list.
type ChartArtist struct {
	Title       string      `json:"title"`
	BrowseID    string      `json:"browseId,omitempty"`
	Subscribers string      `json:"subscribers,omitempty"`
	Rank        string      `json:"rank,omitempty"`
	Trend       string      `json:"trend,omitempty"`
	Thumbnails  []Thumbnail `json:"thumbnails,omitempty"`
}

// ChartPlaylist holds a list of items for a specific chart section.
type ChartPlaylist struct {
	Playlist string             `json:"playlist,omitempty"`
	Items    []HomeCarouselItem `json:"items,omitempty"`
}

// ExploreData is the full response from get_explore.
type ExploreData struct {
	NewReleases    []ArtistAlbum      `json:"new_releases,omitempty"`
	TopSongs       *ChartPlaylist     `json:"top_songs,omitempty"`
	MoodsAndGenres []MoodCategory     `json:"moods_and_genres,omitempty"`
	TopEpisodes    []any              `json:"top_episodes,omitempty"` // skipping for now
	Trending       *ChartPlaylist     `json:"trending,omitempty"`
	NewVideos      []HomeCarouselItem `json:"new_videos,omitempty"`
}

type ChartsCountries struct {
	Selected map[string]string `json:"selected,omitempty"`
	Options  []string          `json:"options,omitempty"`
}

// ChartsData is the full response from get_charts.
type ChartsData struct {
	Countries ChartsCountries    `json:"countries,omitempty"`
	Videos    []HomeCarouselItem `json:"videos,omitempty"`
	Artists   []ChartArtist      `json:"artists,omitempty"`
	Genres    []HomeCarouselItem `json:"genres,omitempty"`
	Daily     []HomeCarouselItem `json:"daily,omitempty"`
	Weekly    []HomeCarouselItem `json:"weekly,omitempty"`
}
