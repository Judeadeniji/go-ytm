package tui

const (
	leftSidebarWidth  = 24
	rightSidebarWidth = 32
	// Terminal must leave room for left + right + a usable main pane.
	minWidthForQueuePanel = leftSidebarWidth + rightSidebarWidth + 40
	headerHeight          = 4

	queueArtWidth  = 26
	queueArtHeight = 12
)

// layoutWidths returns left, main, and right column widths for the current state.
// right is 0 when the queue panel is hidden.
func (m Model) layoutWidths() (left, main, right int) {
	left = leftSidebarWidth
	right = 0
	if m.showQueuePanel() {
		right = rightSidebarWidth
	}
	main = m.width - left - right
	if main < 0 {
		main = 0
	}
	return left, main, right
}

// showQueuePanel reports whether the right rail should be visible.
// It appears while something is queued/playing, when the terminal is wide
// enough, and unless the user has dismissed it.
func (m Model) showQueuePanel() bool {
	if m.queuePanelHidden {
		return false
	}
	if m.currentTrack == nil && m.queue.IsEmpty() {
		return false
	}
	if m.width < minWidthForQueuePanel {
		return false
	}
	return true
}

func (m Model) contentHeight() int {
	h := m.height - playerBarHeight
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) mainPaneHeight() int {
	h := m.contentHeight() - headerHeight
	if h < 1 {
		return 1
	}
	return h
}
