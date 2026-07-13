package search

// Search manages queries to YouTube via kkdai/youtube wrapper and yt-dlp fallback
type Search struct {
}

func NewSearch() *Search {
	return &Search{}
}
