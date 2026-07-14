package tui

// ScreenKind identifies center-content screens on the view stack.
type ScreenKind int

const (
	ScreenHome ScreenKind = iota
	ScreenSearch
	ScreenArtist
	ScreenAlbum
	ScreenPlaylist
)

// Screen is one navigable center view.
type Screen struct {
	Kind ScreenKind
	ID   string // browseId / playlistId / channelId
	Title string
}

// ViewStack is a simple LIFO navigation stack. Empty stack means Home.
type ViewStack struct {
	items []Screen
}

func (s *ViewStack) Push(sc Screen) {
	s.items = append(s.items, sc)
}

func (s *ViewStack) Pop() (Screen, bool) {
	if len(s.items) == 0 {
		return Screen{}, false
	}
	last := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return last, true
}

func (s *ViewStack) Current() (Screen, bool) {
	if len(s.items) == 0 {
		return Screen{}, false
	}
	return s.items[len(s.items)-1], true
}

func (s *ViewStack) Clear() {
	s.items = nil
}

func (s *ViewStack) Len() int {
	return len(s.items)
}

func (s *ViewStack) IsHome() bool {
	return len(s.items) == 0
}
