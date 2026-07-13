package library

// Library manages sqlite-backed local playlists, queue, download cache index
type Library struct {
}

func NewLibrary() *Library {
	return &Library{}
}
