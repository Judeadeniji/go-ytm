package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// KittyImage holds terminal image data. Spacer is embedded in the Bubble Tea
// layout; UploadSeq is written directly to /dev/tty when using Kitty Graphics
// (Bubble Tea strips APC escape sequences from View output).
type KittyImage struct {
	UploadSeq string
	PlaceSeq  string
	Spacer    string
}

// WriteToTTY writes the Kitty Graphics upload payload directly to /dev/tty.
// It is a no-op when UploadSeq is empty (e.g. ANSI half-block fallback).
func (k *KittyImage) WriteToTTY() error {
	if k == nil || k.UploadSeq == "" {
		return nil
	}
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, k.UploadSeq)
	return err
}

// Art cell sizes. Card art stays large; track rows use compact thumbs.
const (
	artWidth  = 24
	artHeight = 10

	coverWidth  = 18
	coverHeight = 8

	sugArtWidth  = 8
	sugArtHeight = 3
)

func imageCacheKey(url string, width, height int) string {
	return fmt.Sprintf("%s@%dx%d", url, width, height)
}

const maxImageBytes = 5 << 20 // 5 MiB

var imageHTTPClient = &http.Client{
	Timeout: 8 * time.Second,
}

// artPlaceholder returns a fixed-size grey box used while thumbs load / on error.
func artPlaceholder() string {
	return sizedPlaceholder(artWidth, artHeight)
}

func sizedPlaceholder(width, height int) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#333333")).
		Width(width).
		Height(height).
		Render("")
}

// fitArtBox forces art into a fixed Width×Height cell so layout never shifts.
func fitArtBox(s string, width, height int) string {
	if s == "" {
		return sizedPlaceholder(width, height)
	}
	s = strings.TrimRight(s, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxWidth(width).
		MaxHeight(height).
		Render(s)
}



// mosaic treats Width/Height as pixel samples stepped 2×2 per cell, so we
// request 2× the desired character-cell size. ANSI halfblocks is fallback only.
func renderWithTermimg(img image.Image, width, height int) string {
	// Bypass termimg which generates unoptimized, heavy ANSI with redundant sequences and resets.
	// We use our heavily optimized ansiHalfblocks instead.
	return ansiHalfblocks(img, width, height)
}

func wrapRendered(rendered string, width, height int) KittyImage {
	return KittyImage{Spacer: fitArtBox(rendered, width, height)}
}

// RenderLocalImage loads a local file and renders it as a terminal image.
func RenderLocalImage(filepath string, width, height, _ int) KittyImage {
	f, err := os.Open(filepath)
	if err != nil {
		return fallbackKitty(width, height)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fallbackKitty(width, height)
	}
	return wrapRendered(renderWithTermimg(img, width, height), width, height)
}

// RenderRemoteImage downloads a URL and renders it as a terminal image.
func RenderRemoteImage(url string, width, height, _ int) KittyImage {
	resp, err := imageHTTPClient.Get(url)
	if err != nil {
		return fallbackKitty(width, height)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fallbackKitty(width, height)
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(io.LimitReader(resp.Body, maxImageBytes+1)); err != nil {
		return fallbackKitty(width, height)
	}
	if buf.Len() > maxImageBytes {
		return fallbackKitty(width, height)
	}

	img, _, err := image.Decode(buf)
	if err != nil {
		return fallbackKitty(width, height)
	}
	return wrapRendered(renderWithTermimg(img, width, height), width, height)
}

// ansiHalfblocks renders an image.Image using Unicode ▀ half-block characters
// with ANSI true-color escape codes. This is the universal ANSI fallback.
func ansiHalfblocks(img image.Image, width, height int) string {
	scaled := resizeNearest(img, width, height*2)

	var sb strings.Builder
	// Pre-allocate to reduce allocations
	sb.Grow(width * height * 20)

	for y := 0; y < height*2; y += 2 {
		var lastFg, lastBg uint32 = 0xFFFFFFFF, 0xFFFFFFFF // Invalid initial colors
		
		for x := 0; x < width; x++ {
			r1, g1, b1, _ := scaled.At(x, y).RGBA()
			r2, g2, b2, _ := scaled.At(x, y+1).RGBA()
			
			// Extract 8-bit RGB
			r1, g1, b1 = r1>>8, g1>>8, b1>>8
			r2, g2, b2 = r2>>8, g2>>8, b2>>8
			
			fg := (r1 << 16) | (g1 << 8) | b1
			bg := (r2 << 16) | (g2 << 8) | b2
			
			if fg != lastFg {
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r1, g1, b1))
				lastFg = fg
			}
			if bg != lastBg {
				sb.WriteString(fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r2, g2, b2))
				lastBg = bg
			}
			sb.WriteString("▀")
		}
		sb.WriteString("\x1b[0m") // Reset at the end of each line
		if y < (height*2)-2 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// resizeNearest is a simple nearest-neighbour resize — no cgo, no external deps.
func resizeNearest(src image.Image, w, h int) image.Image {
	srcB := src.Bounds()
	srcW, srcH := srcB.Dx(), srcB.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := srcB.Min.X + x*srcW/w
			sy := srcB.Min.Y + y*srcH/h
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func fallbackKitty(width, height int) KittyImage {
	return KittyImage{Spacer: sizedPlaceholder(width, height)}
}
