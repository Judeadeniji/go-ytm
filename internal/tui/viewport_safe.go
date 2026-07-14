package tui

import "github.com/charmbracelet/bubbles/viewport"

// safeViewportView renders a viewport after clamping scroll offsets so a short
// content replacement can't panic bubbles' visibleLines slice (YOffset past EOF).
func safeViewportView(vp *viewport.Model) string {
	if vp.Height <= 0 || vp.Width < 0 {
		return ""
	}
	vp.SetYOffset(vp.YOffset)
	return vp.View()
}
