package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/search"
)

const (
	streamCacheTTL    = 25 * time.Minute
	preloadAheadFar   = 1 // keep one URL warm while comfortable
	preloadAheadNear  = 2 // warm two when near the end of the track
	preloadNearEndSec = 90
)

type cachedStream struct {
	URL string
	At  time.Time
}

// streamPreloadMsg is an async prefetch of a future queue item's stream URL.
type streamPreloadMsg struct {
	VideoID string
	URL     string
	Err     error
}

func (m *Model) initStreamCache() {
	if m.streamCache == nil {
		m.streamCache = make(map[string]cachedStream)
	}
	if m.preloadInflight == nil {
		m.preloadInflight = make(map[string]struct{})
	}
	if m.preloadCancel == nil {
		m.preloadCancel = make(map[string]context.CancelFunc)
	}
}

func (m *Model) cancelPrefetch(videoID string) {
	m.initStreamCache()
	if videoID == "" {
		return
	}
	if cancel, ok := m.preloadCancel[videoID]; ok {
		cancel()
		delete(m.preloadCancel, videoID)
	}
	delete(m.preloadInflight, videoID)
}

func (m *Model) cancelAllPrefetches() {
	m.initStreamCache()
	for id, cancel := range m.preloadCancel {
		cancel()
		delete(m.preloadCancel, id)
	}
	clear(m.preloadInflight)
}

func (m *Model) pruneStreamCache() {
	m.initStreamCache()
	now := time.Now()
	live := make(map[string]struct{}, m.queue.Len())
	for _, t := range m.queue.Tracks() {
		if t.VideoID != "" {
			live[t.VideoID] = struct{}{}
		}
	}
	for id, e := range m.streamCache {
		if _, ok := live[id]; !ok || now.Sub(e.At) > streamCacheTTL {
			delete(m.streamCache, id)
		}
	}
	for id := range m.preloadInflight {
		if _, ok := live[id]; !ok {
			m.cancelPrefetch(id)
		}
	}
}

func (m *Model) peekStreamCache(videoID string) (string, bool) {
	m.initStreamCache()
	if videoID == "" {
		return "", false
	}
	e, ok := m.streamCache[videoID]
	if !ok {
		return "", false
	}
	if time.Since(e.At) > streamCacheTTL || e.URL == "" {
		delete(m.streamCache, videoID)
		return "", false
	}
	return e.URL, true
}

func (m *Model) putStreamCache(videoID, url string) {
	m.initStreamCache()
	if videoID == "" || url == "" {
		return
	}
	m.streamCache[videoID] = cachedStream{URL: url, At: time.Now()}
}

func (m *Model) invalidateStreamCache(videoID string) {
	m.cancelPrefetch(videoID)
	m.initStreamCache()
	delete(m.streamCache, videoID)
}

// playTrackResolved uses a warm stream URL when available, otherwise extracts.
func playTrackResolved(ext *search.Extractor, t Track, gen int, ctx context.Context, cachedURL string) tea.Cmd {
	if cachedURL != "" {
		return func() tea.Msg {
			return streamReadyMsg{Track: t, URL: cachedURL, Gen: gen, Cached: true}
		}
	}
	return playTrack(ext, t, gen, ctx)
}

func (m *Model) queueContainsVideo(videoID string) bool {
	if videoID == "" {
		return false
	}
	for _, t := range m.queue.Tracks() {
		if t.VideoID == videoID {
			return true
		}
	}
	return false
}

// ensureUpcomingPreloaded kickstarts stream URL (+ art) warm for upcoming queue items.
func (m *Model) ensureUpcomingPreloaded() tea.Cmd {
	m.pruneStreamCache()
	cur := m.queue.CurrentIndex()
	if cur < 0 {
		return nil
	}

	ahead := preloadAheadFar
	// Warm while resolving/playing current — not only after audioLoaded.
	if m.playDuration > 30 {
		left := m.playDuration - m.playPos
		if left > 0 && left <= preloadNearEndSec {
			ahead = preloadAheadNear
		}
	} else if m.isPlaying && !m.audioLoaded {
		// Current track is still extracting — still warm at least the next one.
		ahead = preloadAheadFar
	}

	var cmds []tea.Cmd
	for i := 1; i <= ahead; i++ {
		t, ok := m.queue.At(cur + i)
		if !ok || t.VideoID == "" {
			break
		}
		if _, hit := m.peekStreamCache(t.VideoID); hit {
			continue
		}
		if _, busy := m.preloadInflight[t.VideoID]; busy {
			continue
		}
		m.preloadInflight[t.VideoID] = struct{}{}
		cmds = append(cmds, m.prefetchStreamURLCmd(t.VideoID))
		if t.ThumbnailURL != "" {
			cmds = append(cmds, m.enqueueImageURL(t.ThumbnailURL))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) prefetchStreamURLCmd(videoID string) tea.Cmd {
	m.initStreamCache()
	if cancel, ok := m.preloadCancel[videoID]; ok {
		cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	m.preloadCancel[videoID] = cancel
	ext := m.extractor
	return func() tea.Msg {
		defer cancel()
		if ext == nil || videoID == "" {
			return streamPreloadMsg{VideoID: videoID, Err: context.Canceled}
		}
		url, err := ext.GetStreamURL(ctx, videoID)
		return streamPreloadMsg{VideoID: videoID, URL: url, Err: err}
	}
}

// enqueueImageURL warms the image cache for a URL at queue-rail art size when possible.
func (m Model) enqueueImageURL(url string) tea.Cmd {
	if url == "" {
		return nil
	}
	w, h := m.queueArtDims()
	key := imageCacheKey(url, w, h)
	if _, ok := m.imageCache[key]; ok {
		return nil
	}
	return fetchImageSized(url, w, h)
}
