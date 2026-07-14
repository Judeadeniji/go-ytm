package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"

	termimg "github.com/blacktop/go-termimg"
	"github.com/charmbracelet/lipgloss"
)

// renderWithTermimg tries to render via go-termimg (auto-detects Kitty/Sixel/iTerm2/Halfblocks).
// Falls back to our own ANSI half-block renderer if termimg fails.
func renderWithTermimg(img image.Image, width, height int) string {
	rendered, err := termimg.New(img).Width(width).Height(height).Scale(termimg.ScaleFit).Render()
	if err == nil && rendered != "" {
		return rendered
	}
	// Fallback: own ANSI half-block renderer
	return ansiHalfblocks(img, width, height)
}

// RenderLocalImage loads a local file and renders it as a terminal image.
func RenderLocalImage(filepath string, width, height, _ int) string {
	f, err := os.Open(filepath)
	if err != nil {
		return fallbackBlock(width, height)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fallbackBlock(width, height)
	}
	return renderWithTermimg(img, width, height)
}

// RenderRemoteImage downloads a URL and renders it as a terminal image.
func RenderRemoteImage(url string, width, height, _ int) string {
	resp, err := http.Get(url)
	if err != nil {
		return fallbackBlock(width, height)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fallbackBlock(width, height)
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(resp.Body); err != nil {
		return fallbackBlock(width, height)
	}

	img, _, err := image.Decode(buf)
	if err != nil {
		return fallbackBlock(width, height)
	}
	return renderWithTermimg(img, width, height)
}

// ansiHalfblocks renders an image.Image using Unicode ▀ half-block characters
// with ANSI true-color escape codes. This is the universal ANSI fallback.
func ansiHalfblocks(img image.Image, width, height int) string {
	// Scale to width×(height*2) so each pair of pixel rows maps to one terminal row.
	scaled := resizeNearest(img, width, height*2)

	var sb strings.Builder
	for y := 0; y < height*2; y += 2 {
		for x := 0; x < width; x++ {
			r1, g1, b1, _ := scaled.At(x, y).RGBA()
			r2, g2, b2, _ := scaled.At(x, y+1).RGBA()
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r1>>8, g1>>8, b1>>8))).
				Background(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r2>>8, g2>>8, b2>>8)))
			sb.WriteString(style.Render("▀"))
		}
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

// fallbackBlock returns a plain colored block when image loading fails.
func fallbackBlock(width, height int) string {
	return lipgloss.NewStyle().Background(lipgloss.Color("#333333")).Width(width).Height(height).Render("")
}
