package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	youtube "github.com/kkdai/youtube/v2"
)

// authRoundTripper injects auth headers from ytmusicapi state to bypass 429s.
type authRoundTripper struct {
	transport http.RoundTripper
}

func (a *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	path := os.ExpandEnv("$HOME/.local/state/go-ytm/headers_auth.json")
	if data, err := os.ReadFile(path); err == nil {
		var content map[string]any
		if json.Unmarshal(data, &content) == nil {
			if token, ok := content["access_token"].(string); ok && token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			} else {
				for k, v := range content {
					if strV, ok := v.(string); ok {
						lowerK := strings.ToLower(k)
						if lowerK == "cookie" || lowerK == "user-agent" || lowerK == "authorization" || strings.HasPrefix(lowerK, "x-youtube") || strings.HasPrefix(lowerK, "x-goog") {
							req.Header.Set(k, strV)
						}
					}
				}
			}
		}
	}

	transport := a.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(req)
}

// Extractor manages extraction of stream URLs using kkdai/youtube and yt-dlp fallback
type Extractor struct {
	client *youtube.Client
}

func NewExtractor() *Extractor {
	return &Extractor{
		client: &youtube.Client{
			// Bound orphaned GetVideo work after ctx cancel (library ignores cancel).
			HTTPClient: &http.Client{
				Timeout:   45 * time.Second,
				Transport: &authRoundTripper{},
			},
		},
	}
}

// GetStreamURL attempts to get the stream URL, falling back to yt-dlp if it fails
func (e *Extractor) GetStreamURL(ctx context.Context, videoID string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	url, primaryErr := e.getStreamURLYoutubeClient(ctx, videoID)
	if primaryErr == nil && url != "" {
		return url, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	url, fallbackErr := e.getStreamURLYtDlp(ctx, videoID)
	if fallbackErr == nil && url != "" {
		return url, nil
	}

	if primaryErr == nil {
		primaryErr = errors.New("empty stream URL")
	}
	if fallbackErr == nil {
		fallbackErr = errors.New("empty stream URL")
	}

	if isNotFound(fallbackErr) {
		return "", fmt.Errorf("%w (yt-dlp fallback missing — install yt-dlp or add it to PATH)", primaryErr)
	}
	return "", fmt.Errorf("%w; yt-dlp fallback: %w", primaryErr, fallbackErr)
}

func isNotFound(err error) bool {
	var e *exec.Error
	return errors.As(err, &e) && e.Err == exec.ErrNotFound
}

func (e *Extractor) getStreamURLYoutubeClient(ctx context.Context, videoID string) (string, error) {
	type result struct {
		url string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		video, err := e.client.GetVideo(videoID)
		if err != nil {
			ch <- result{err: err}
			return
		}

		formats := video.Formats.WithAudioChannels()
		if len(formats) == 0 {
			ch <- result{err: fmt.Errorf("no audio formats found")}
			return
		}

		// Sort formats to get the best audio quality
		formats.Sort()

		streamURL, err := e.client.GetStreamURL(video, &formats[0])
		if err != nil {
			ch <- result{err: err}
			return
		}
		ch <- result{url: streamURL}
	}()

	select {
	case <-ctx.Done():
		// Return immediately; HTTPClient timeout bounds the orphaned GetVideo.
		return "", ctx.Err()
	case r := <-ch:
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return r.url, r.err
	}
}

func (e *Extractor) getStreamURLYtDlp(ctx context.Context, videoID string) (string, error) {
	bin, err := lookPathYtDlp()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	
	args := []string{"-g", "-f", "bestaudio/bestaudio*/best", "--no-playlist"}
	
	path := os.ExpandEnv("$HOME/.local/state/go-ytm/headers_auth.json")
	if data, err := os.ReadFile(path); err == nil {
		var content map[string]any
		if json.Unmarshal(data, &content) == nil {
			if token, ok := content["access_token"].(string); ok && token != "" {
				args = append(args, "--add-header", "Authorization: Bearer "+token)
			} else {
				for k, v := range content {
					if strV, ok := v.(string); ok {
						lowerK := strings.ToLower(k)
						if lowerK == "cookie" || lowerK == "user-agent" || lowerK == "authorization" || strings.HasPrefix(lowerK, "x-youtube") || strings.HasPrefix(lowerK, "x-goog") {
							args = append(args, "--add-header", fmt.Sprintf("%s: %s", k, strV))
						}
					}
				}
			}
		}
	}
	
	args = append(args, url)
	cmd := exec.CommandContext(ctx, bin, args...)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("yt-dlp failed: %s", firstLine(msg))
		}
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}

	streamURL := firstLine(out.String())
	if streamURL == "" {
		return "", fmt.Errorf("yt-dlp returned empty URL")
	}

	return streamURL, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// lookPathYtDlp finds yt-dlp on PATH or in common install locations.
// Cursor/AppImage shells often omit ~/.local/bin even when the user has it installed.
func lookPathYtDlp() (string, error) {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "yt-dlp"),
		"/usr/bin/yt-dlp",
		"/usr/local/bin/yt-dlp",
		"/opt/homebrew/bin/yt-dlp",
	}

	// Project-local copy (bin/yt-dlp next to the built TUI)
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), "yt-dlp")}, candidates...)
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return c, nil
		}
	}

	return "", &exec.Error{Name: "yt-dlp", Err: exec.ErrNotFound}
}
