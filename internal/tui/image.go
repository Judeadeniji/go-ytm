package tui

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nfnt/resize"
)

// RenderLocalImage loads an image and renders it using ANSI half-block characters (▀)
// to simulate pixels in the terminal.
func RenderLocalImage(filepath string, width, height int) string {
	f, err := os.Open(filepath)
	if err != nil {
		// Fallback block
		return lipgloss.NewStyle().Background(lipgloss.Color("#333333")).Width(width).Height(height).Render("")
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return lipgloss.NewStyle().Background(lipgloss.Color("#333333")).Width(width).Height(height).Render("")
	}

	// We scale the image to width x (height * 2) since half-block characters
	// allow two "pixels" vertically per terminal cell.
	resized := resize.Resize(uint(width), uint(height*2), img, resize.Lanczos3)

	var sb strings.Builder
	for y := 0; y < height*2; y += 2 {
		for x := 0; x < width; x++ {
			topColor := resized.At(x, y)
			bottomColor := resized.At(x, y+1)

			r1, g1, b1, _ := topColor.RGBA()
			r2, g2, b2, _ := bottomColor.RGBA()

			hexTop := fmt.Sprintf("#%02x%02x%02x", r1>>8, g1>>8, b1>>8)
			hexBot := fmt.Sprintf("#%02x%02x%02x", r2>>8, g2>>8, b2>>8)

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(hexTop)).
				Background(lipgloss.Color(hexBot))
			sb.WriteString(style.Render("▀"))
		}
		if y < (height*2)-2 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
