package search

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kkdai/youtube/v2"
)

// Extractor manages extraction of stream URLs using kkdai/youtube and yt-dlp fallback
type Extractor struct {
	client *youtube.Client
}

func NewExtractor() *Extractor {
	return &Extractor{
		client: &youtube.Client{},
	}
}

// GetStreamURL attempts to get the stream URL, falling back to yt-dlp if it fails
func (e *Extractor) GetStreamURL(ctx context.Context, videoID string) (string, error) {
	url, err := e.getStreamURLYoutubeClient(videoID)
	if err == nil && url != "" {
		return url, nil
	}

	// Fallback to yt-dlp
	return e.getStreamURLYtDlp(ctx, videoID)
}

func (e *Extractor) getStreamURLYoutubeClient(videoID string) (string, error) {
	video, err := e.client.GetVideo(videoID)
	if err != nil {
		return "", err
	}

	formats := video.Formats.WithAudioChannels()
	if len(formats) == 0 {
		return "", fmt.Errorf("no audio formats found")
	}

	// Sort formats to get the best audio quality
	formats.Sort()

	streamURL, err := e.client.GetStreamURL(video, &formats[0])
	if err != nil {
		return "", err
	}

	return streamURL, nil
}

func (e *Extractor) getStreamURLYtDlp(ctx context.Context, videoID string) (string, error) {
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	cmd := exec.CommandContext(ctx, "yt-dlp", "-g", "-f", "bestaudio", url)
	
	var out bytes.Buffer
	cmd.Stdout = &out
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}

	streamURL := strings.TrimSpace(out.String())
	if streamURL == "" {
		return "", fmt.Errorf("yt-dlp returned empty URL")
	}

	return streamURL, nil
}
