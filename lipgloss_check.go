package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	topBarHeight := 3
	topBarStyle := lipgloss.NewStyle().
		Height(topBarHeight).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		Padding(0, 2)

	topBar := topBarStyle.Render("content")
	fmt.Printf("TopBar total height: %d\n", lipgloss.Height(topBar))
	
	leftSidebarStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		Padding(1, 2)
	leftSidebar := leftSidebarStyle.Width(30).Render("content")
	fmt.Printf("LeftSidebar total width: %d\n", lipgloss.Width(leftSidebar))
}
