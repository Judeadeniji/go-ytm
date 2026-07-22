package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/judeadeniji/go-ytm/internal/library"
	"github.com/judeadeniji/go-ytm/internal/search"
)

// ProgressMsg is sent to the TUI to report download progress.
type ProgressMsg struct {
	VideoID string
	Bytes   int64
	Total   int64
	Done    bool
	Err     error
	Track   library.CachedTrack
}

// Manager handles a background queue of downloads.
type Manager struct {
	extractor *search.Extractor
	db        *library.DB
	dir       string

	mu      sync.Mutex
	active  map[string]context.CancelFunc
	program *tea.Program
}

// NewManager creates a download manager.
func NewManager(extractor *search.Extractor, db *library.DB) (*Manager, error) {
	stateDir := os.ExpandEnv("$HOME/.local/state/go-ytm/downloads")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, err
	}

	m := &Manager{
		extractor: extractor,
		db:        db,
		dir:       stateDir,
		active:    make(map[string]context.CancelFunc),
	}
	return m, nil
}

func (m *Manager) SetProgram(p *tea.Program) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.program = p
}

// Enqueue adds a track to the download queue.
func (m *Manager) Enqueue(t library.CachedTrack) {
	m.mu.Lock()
	if _, ok := m.active[t.VideoID]; ok {
		m.mu.Unlock()
		return
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	m.active[t.VideoID] = cancel
	m.mu.Unlock()

	go m.worker(ctx, t)
}

// Cancel stops a download.
func (m *Manager) Cancel(videoID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.active[videoID]; ok {
		cancel()
		delete(m.active, videoID)
	}
}

// IsDownloading returns true if the track is in progress.
func (m *Manager) IsDownloading(videoID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.active[videoID]
	return ok
}

// ActiveIDs returns video IDs currently being downloaded.
func (m *Manager) ActiveIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.active))
	for id := range m.active {
		ids = append(ids, id)
	}
	return ids
}

type progressWriter struct {
	videoID string
	track   library.CachedTrack
	total   int64
	current int64
	lastPub time.Time
	program *tea.Program
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.current += int64(n)

	if pw.program != nil {
		now := time.Now()
		if now.Sub(pw.lastPub) > 500*time.Millisecond {
			pw.program.Send(ProgressMsg{
				VideoID: pw.videoID,
				Bytes:   pw.current,
				Total:   pw.total,
				Track:   pw.track,
			})
			pw.lastPub = now
		}
	}

	return n, nil
}

func (m *Manager) worker(ctx context.Context, t library.CachedTrack) {
	defer func() {
		m.mu.Lock()
		delete(m.active, t.VideoID)
		m.mu.Unlock()
	}()

	sendProgress := func(done bool, err error, total int64) {
		m.mu.Lock()
		p := m.program
		m.mu.Unlock()
		if p != nil {
			p.Send(ProgressMsg{
				VideoID: t.VideoID,
				Bytes:   total,
				Total:   total,
				Done:    done,
				Err:     err,
				Track:   t,
			})
		}
	}

	url, err := m.extractor.GetStreamURL(ctx, t.VideoID)
	if err != nil {
		sendProgress(true, err, 0)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		sendProgress(true, err, 0)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sendProgress(true, err, 0)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		sendProgress(true, fmt.Errorf("bad status: %s", resp.Status), 0)
		return
	}

	total := resp.ContentLength
	t.Path = filepath.Join(m.dir, t.VideoID+".m4a")

	tmpPath := t.Path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		sendProgress(true, err, 0)
		return
	}

	m.mu.Lock()
	pw := &progressWriter{
		videoID: t.VideoID,
		track:   t,
		total:   total,
		program: m.program,
		lastPub: time.Now(),
	}
	m.mu.Unlock()

	bytesWritten, err := io.Copy(f, io.TeeReader(resp.Body, pw))
	f.Close()

	if err != nil {
		os.Remove(tmpPath)
		sendProgress(true, err, 0)
		return
	}

	if err := os.Rename(tmpPath, t.Path); err != nil {
		sendProgress(true, err, 0)
		return
	}

	t.Bytes = bytesWritten
	if m.db != nil {
		_ = m.db.AddDownload(t)
	}

	sendProgress(true, nil, bytesWritten)
}
