package tui

// Track represents a playable audio track.
type Track struct {
	VideoID        string
	Title          string
	Artist         string
	ArtistID       string // channel / browse id when known
	Album          string
	AlbumID        string // album browse id when known
	Duration       string // display duration from API (e.g. "3:07")
	ThumbnailURL   string
	IsExplicit     bool
}

// Queue manages an ordered list of tracks with a current-position pointer.
// current == -1 means nothing is queued / nothing is playing.
type Queue struct {
	tracks  []Track
	current int
}

// Add appends a track to the end of the queue.
func (q *Queue) Add(t Track) {
	q.tracks = append(q.tracks, t)
}

// AddNext inserts a track immediately after the current position.
// If the queue is empty, the track becomes the first element.
func (q *Queue) AddNext(t Track) {
	if len(q.tracks) == 0 || q.current < 0 {
		q.tracks = append([]Track{t}, q.tracks...)
		return
	}
	insertAt := q.current + 1
	q.tracks = append(q.tracks[:insertAt], append([]Track{t}, q.tracks[insertAt:]...)...)
}

// Current returns the track at the current position and whether one exists.
func (q *Queue) Current() (Track, bool) {
	if q.current < 0 || q.current >= len(q.tracks) {
		return Track{}, false
	}
	return q.tracks[q.current], true
}

// Next advances the position by one and returns the new current track.
// Returns (Track{}, false) if already at the end.
func (q *Queue) Next() (Track, bool) {
	if q.current+1 >= len(q.tracks) {
		return Track{}, false
	}
	q.current++
	return q.tracks[q.current], true
}

// Prev moves the position back by one and returns the new current track.
// Returns (Track{}, false) if already at the beginning.
func (q *Queue) Prev() (Track, bool) {
	if q.current <= 0 {
		return Track{}, false
	}
	q.current--
	return q.tracks[q.current], true
}

// JumpTo sets the current index. Returns false if the index is out of range.
func (q *Queue) JumpTo(index int) bool {
	if index < 0 || index >= len(q.tracks) {
		return false
	}
	q.current = index
	return true
}

// AppendAndSelect appends a track and makes it the current item.
func (q *Queue) AppendAndSelect(t Track) {
	q.tracks = append(q.tracks, t)
	q.current = len(q.tracks) - 1
}

// SetPlaying replaces the queue with a single track as current.
func (q *Queue) SetPlaying(t Track) {
	q.tracks = []Track{t}
	q.current = 0
}

// SetFrom replaces the queue with tracks[start:] and selects the first of that slice.
func (q *Queue) SetFrom(tracks []Track, start int) {
	if start < 0 {
		start = 0
	}
	if start >= len(tracks) {
		q.Clear()
		return
	}
	q.tracks = append([]Track(nil), tracks[start:]...)
	q.current = 0
}

// TruncateAfterCurrent drops every track after the current one.
func (q *Queue) TruncateAfterCurrent() {
	if q.current < 0 {
		q.tracks = nil
		return
	}
	if q.current+1 < len(q.tracks) {
		q.tracks = q.tracks[:q.current+1]
	}
}

// CapHistory keeps at most maxPlayed tracks before the current index.
func (q *Queue) CapHistory(maxPlayed int) {
	if maxPlayed < 0 || q.current <= maxPlayed {
		return
	}
	drop := q.current - maxPlayed
	q.tracks = append([]Track(nil), q.tracks[drop:]...)
	q.current -= drop
}

// Clear empties the queue and resets the position.
func (q *Queue) Clear() {
	q.tracks = nil
	q.current = -1
}

// Len returns the number of tracks in the queue.
func (q *Queue) Len() int {
	return len(q.tracks)
}

// IsEmpty reports whether the queue contains no tracks.
func (q *Queue) IsEmpty() bool {
	return len(q.tracks) == 0
}

// Tracks returns a copy of the queued tracks.
func (q Queue) Tracks() []Track {
	if len(q.tracks) == 0 {
		return nil
	}
	out := make([]Track, len(q.tracks))
	copy(out, q.tracks)
	return out
}

// CurrentIndex returns the queue cursor (-1 if none).
func (q Queue) CurrentIndex() int {
	return q.current
}

// At returns the track at index i.
func (q Queue) At(i int) (Track, bool) {
	if i < 0 || i >= len(q.tracks) {
		return Track{}, false
	}
	return q.tracks[i], true
}
