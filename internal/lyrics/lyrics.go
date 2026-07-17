package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL       = "https://lrclib.net"
	userAgent     = "go-ytm/0.1 (https://github.com/Judeadeniji/go-ytm)"
	maxLyricsBody = 2 << 20 // 2 MiB
)

// Line is one timed lyric line (seconds from track start).
type Line struct {
	Time time.Duration
	Text string
}

// Result is one LRCLIB search/get hit.
type Result struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// Lyrics is a resolved lyrics payload for the TUI.
type Lyrics struct {
	Instrumental bool
	Plain        string
	RawSynced    string
	Lines        []Line
	SourceID     int
}

// Client talks to the LRCLIB REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    baseURL,
	}
}

// Search finds lyrics candidates by track/artist/album keywords.
func (c *Client) Search(ctx context.Context, track, artist, album string) ([]Result, error) {
	q := url.Values{}
	if track != "" {
		q.Set("track_name", track)
	}
	if artist != "" {
		q.Set("artist_name", artist)
	}
	if album != "" {
		q.Set("album_name", album)
	}
	if track == "" && artist == "" {
		return nil, fmt.Errorf("lyrics search needs track or artist")
	}
	// LRCLIB requires q OR track_name; ensure at least one.
	if q.Get("track_name") == "" {
		q.Set("q", artist)
	}

	var out []Result
	if err := c.getJSON(ctx, "/api/search?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FetchForTrack searches and picks the best match for the playing track.
// durationSec may be 0 when unknown; when >0 prefer ±2s duration matches.
func (c *Client) FetchForTrack(ctx context.Context, track, artist, album string, durationSec float64) (*Lyrics, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	results, err := c.Search(ctx, track, artist, album)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		// Broader keyword search as fallback.
		q := url.Values{}
		q.Set("q", strings.TrimSpace(track+" "+artist))
		var broad []Result
		if err := c.getJSON(ctx, "/api/search?"+q.Encode(), &broad); err == nil {
			results = broad
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no lyrics found")
	}

	best := pickBest(results, durationSec)
	return resultToLyrics(best), nil
}

func pickBest(results []Result, durationSec float64) Result {
	if durationSec > 0 {
		var matched []Result
		for _, r := range results {
			if math.Abs(r.Duration-durationSec) <= 2 {
				matched = append(matched, r)
			}
		}
		if len(matched) > 0 {
			results = matched
		}
	}
	for _, r := range results {
		if strings.TrimSpace(r.SyncedLyrics) != "" {
			return r
		}
	}
	for _, r := range results {
		if r.Instrumental || strings.TrimSpace(r.PlainLyrics) != "" {
			return r
		}
	}
	return results[0]
}

func resultToLyrics(r Result) *Lyrics {
	out := &Lyrics{
		Instrumental: r.Instrumental,
		Plain:        strings.TrimSpace(r.PlainLyrics),
		SourceID:     r.ID,
	}
	if synced := strings.TrimSpace(r.SyncedLyrics); synced != "" {
		out.RawSynced = synced
		out.Lines = ParseLRC(synced)
	}
	if out.Plain == "" && len(out.Lines) > 0 {
		var b strings.Builder
		for i, ln := range out.Lines {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(ln.Text)
		}
		out.Plain = b.String()
	}
	return out
}

var lrcStampRe = regexp.MustCompile(`\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]`)

// ParseLRC parses synchronized LRC text into timed lines (sorted by Time).
func ParseLRC(s string) []Line {
	var lines []Line
	for _, raw := range strings.Split(s, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		matches := lrcStampRe.FindAllStringSubmatchIndex(raw, -1)
		if len(matches) == 0 {
			continue
		}
		// Text is everything after the last timestamp tag.
		last := matches[len(matches)-1]
		text := strings.TrimSpace(raw[last[1]:])
		for _, loc := range matches {
			min, _ := strconv.Atoi(raw[loc[2]:loc[3]])
			sec, _ := strconv.Atoi(raw[loc[4]:loc[5]])
			ms := 0
			if loc[6] >= 0 {
				frac := raw[loc[6]:loc[7]]
				for len(frac) < 3 {
					frac += "0"
				}
				if len(frac) > 3 {
					frac = frac[:3]
				}
				ms, _ = strconv.Atoi(frac)
			}
			d := time.Duration(min)*time.Minute +
				time.Duration(sec)*time.Second +
				time.Duration(ms)*time.Millisecond
			lines = append(lines, Line{Time: d, Text: text})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].Time < lines[j].Time
	})
	return lines
}

// ActiveLineIndex returns the index of the last line with Time <= pos.
// Returns -1 when pos is before the first line or lines is empty.
// lines must be sorted by Time ascending.
func ActiveLineIndex(lines []Line, pos time.Duration) int {
	if len(lines) == 0 {
		return -1
	}
	lo, hi, ans := 0, len(lines)-1, -1
	for lo <= hi {
		mid := (lo + hi) / 2
		if lines[mid].Time <= pos {
			ans = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return ans
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("lrclib unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLyricsBody+1))
	if err != nil {
		return fmt.Errorf("lrclib read: %w", err)
	}
	if len(body) > maxLyricsBody {
		return fmt.Errorf("lrclib response too large")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no lyrics found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lrclib %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("lrclib decode: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
