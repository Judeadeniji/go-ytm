package tui

import (
	"errors"
	"os/exec"
	"strings"
)

func shortStreamErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "yt-dlp fallback missing"), strings.Contains(lower, "executable file not found"):
		var execErr *exec.Error
		if errors.As(err, &execErr) || strings.Contains(lower, "yt-dlp") {
			return "need yt-dlp in PATH"
		}
	case strings.Contains(lower, "login required"), strings.Contains(lower, "confirm your age"):
		return "age-restricted (need yt-dlp)"
	case strings.Contains(lower, "video is unavailable"):
		return "video unavailable"
	case strings.Contains(lower, "no audio formats"):
		return "no audio stream"
	}

	if len(msg) > 40 {
		return "Error: " + msg[:37] + "…"
	}
	return "Error: " + msg
}
