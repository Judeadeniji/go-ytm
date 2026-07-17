package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsTabs defines the ordered list of tab IDs and their display names.
var settingsTabs = []struct {
	ID    string
	Label string
	Icon  string
}{
	{"account", "Account", "◉"},
	{"playback", "Playback", "♫"},
	{"audio", "Audio", "◈"},
	{"downloads", "Downloads", "↓"},
	{"general", "General", "⚙"},
}

// settingsItemKind describes what kind of interaction a settings row supports.
type settingsItemKind int

const (
	kindToggle settingsItemKind = iota // enter/space flips on/off
	kindCycle                          // enter/space advances; left/right also work
	kindValue                          // left/right or −/+ adjust; enter resets
	kindAction                         // enter fires the action
	kindInfo                           // non-interactive label
)

// settingsItem is one navigable row in the settings panel.
type settingsItem struct {
	Kind   settingsItemKind
	Label  string
	Desc   string
	ZoneID string // for mouse compatibility
	TabID  string // which tab this belongs to
}

// settingsItems returns the ordered list of interactive items for the given tab.
// This is the single source of truth for keyboard navigation order.
func (m Model) settingsItemsForTab(tabID string) []settingsItem {
	switch tabID {
	case "account":
		l1, l2 := "Sign in (OAuth)", "Sign in (Browser Headers)"
		d1, d2 := "Connect using Google client_secret.json", "Connect by pasting request headers"
		if m.isAuthenticated {
			l1, l2 = "Reconnect (OAuth)", "Reconnect (Browser Headers)"
			d1 = "You are currently authenticated. Press Enter to re-authenticate."
			d2 = "You are currently authenticated. Press Enter to re-authenticate."
		}
		return []settingsItem{
			{kindAction, l1, d1, "settings_oauth", "account"},
			{kindAction, l2, d2, "settings_auth_headers", "account"},
			{kindToggle, "Send listening history", "Allow YouTube Music to personalise recommendations", "settings_history", "account"},
		}
	case "playback":
		items := []settingsItem{
			{kindCycle, "Repeat Mode", "Off → All → One", "settings_repeat", "playback"},
			{kindToggle, "Shuffle", "Play tracks in random order", "settings_shuffle", "playback"},
			{kindToggle, "Crossfade", "Smoothly blend between tracks", "settings_crossfade", "playback"},
		}
		if m.crossfade {
			items = append(items,
				settingsItem{kindValue, "Crossfade Duration", "Overlap length in seconds", "settings_crossfade_val", "playback"},
			)
		}
		items = append(items,
			settingsItem{kindCycle, "Sleep Timer", "Auto-pause after a set time", "settings_sleep", "playback"},
		)
		return items
	case "audio":
		return []settingsItem{
			{kindToggle, "Loudness Normalization", "Level out volume differences between tracks", "settings_normalize", "audio"},
			{kindToggle, "Silence Skip", "Jump over silent passages automatically", "settings_silence", "audio"},
			{kindValue, "Playback Speed", "Adjust tempo (0.25× – 4.00×)  ·  left/right or ← →", "settings_tempo_val", "audio"},
			{kindValue, "Pitch Shift", "Shift pitch in semitones (−12 to +12)", "settings_pitch_val", "audio"},
			{kindAction, "Reset Speed & Pitch", "Restore to 1.00× speed and 0 semitones", "settings_tempo_reset", "audio"},
			{kindCycle, "EQ Preset", "Apply an audio filter to the output", "settings_eq", "audio"},
		}
	case "downloads":
		return []settingsItem{
			{kindAction, "Open Download Folder", "Browse cached audio files  (./downloads/)", "settings_open_downloads", "downloads"},
			{kindAction, "Calculate Cache Size", "Show total space used by offline files", "settings_cache_size", "downloads"},
			{kindAction, "Clear Cache", "Delete all downloaded audio files", "settings_clear_cache", "downloads"},
			{kindCycle, "Download Quality", "Format preference for offline downloads", "settings_dl_quality", "downloads"},
		}
	case "general":
		return []settingsItem{
			{kindToggle, "Show Queue Panel", "Display the queue / lyrics sidebar", "settings_toggle_queue", "general"},
			{kindToggle, "Remember Position", "Resume from where you left off on restart", "settings_remember_pos", "general"},
			{kindAction, "Clear Session", "Reset the queue, history and resume position", "settings_clear_session", "general"},
			{kindInfo, "go-ytm  ·  Terminal YouTube Music client", "Built with mpv · ytmusicapi · bubbletea", "", "general"},
		}
	}
	return nil
}

// settingsTabIndex returns the index of the current tab in settingsTabs.
func settingsTabIndex(id string) int {
	for i, t := range settingsTabs {
		if t.ID == id {
			return i
		}
	}
	return 0
}

// settingsClampRow clamps settingsRow to valid range for current tab.
func (m Model) settingsClampRow() Model {
	items := m.settingsItemsForTab(m.settingsTab)
	// skip info-only rows
	max := 0
	for i, it := range items {
		if it.Kind != kindInfo {
			max = i
		}
	}
	if m.settingsRow < 0 {
		m.settingsRow = 0
	}
	if m.settingsRow > max {
		m.settingsRow = max
	}
	return m
}

// HandleSettingsKey is the keyboard handler when activeMenu == "Settings".
// Returns (Model, tea.Cmd, handled bool).
func (m Model) HandleSettingsKey(key string) (Model, tea.Cmd, bool) {
	// Tab digit shortcuts 1–5
	switch key {
	case "1", "2", "3", "4", "5":
		idx := int(key[0] - '1')
		if idx < len(settingsTabs) {
			m.settingsTab = settingsTabs[idx].ID
			m.settingsRow = 0
			m.setMainContent()
			return m, nil, true
		}
	}

	items := m.settingsItemsForTab(m.settingsTab)
	if len(items) == 0 {
		return m, nil, false
	}

	// safe current item
	row := m.settingsRow
	if row < 0 {
		row = 0
	}
	if row >= len(items) {
		row = len(items) - 1
	}
	cur := items[row]

	switch key {
	// ── Navigation ──────────────────────────────────────────────────────────
	case "up", "k":
		m.settingsRow--
		// skip info rows
		items2 := m.settingsItemsForTab(m.settingsTab)
		for m.settingsRow > 0 && items2[m.settingsRow].Kind == kindInfo {
			m.settingsRow--
		}
		m = m.settingsClampRow()
		m.setMainContent()
		return m, nil, true

	case "down", "j":
		m.settingsRow++
		items2 := m.settingsItemsForTab(m.settingsTab)
		for m.settingsRow < len(items2)-1 && items2[m.settingsRow].Kind == kindInfo {
			m.settingsRow++
		}
		m = m.settingsClampRow()
		m.setMainContent()
		return m, nil, true

	// ── Activate / toggle ────────────────────────────────────────────────────
	case "enter", " ":
		mm, cmd := m.settingsActivate(cur)
		mm.setMainContent()
		return mm, cmd, true

	// ── Value adjustment (left/right) ────────────────────────────────────────
	case "left", "h":
		if cur.Kind == kindValue {
			mm, cmd := m.settingsDec(cur)
			mm.setMainContent()
			return mm, cmd, true
		}
		// fall through: switch tab left
		tabIdx := settingsTabIndex(m.settingsTab)
		tabIdx--
		if tabIdx < 0 {
			tabIdx = len(settingsTabs) - 1
		}
		m.settingsTab = settingsTabs[tabIdx].ID
		m.settingsRow = 0
		m.setMainContent()
		return m, nil, true

	case "right", "l":
		if cur.Kind == kindValue {
			mm, cmd := m.settingsInc(cur)
			mm.setMainContent()
			return mm, cmd, true
		}
		tabIdx := settingsTabIndex(m.settingsTab)
		tabIdx++
		if tabIdx >= len(settingsTabs) {
			tabIdx = 0
		}
		m.settingsTab = settingsTabs[tabIdx].ID
		m.settingsRow = 0
		m.setMainContent()
		return m, nil, true
	}

	return m, nil, false
}

// settingsActivate fires the primary action for a settings item.
func (m Model) settingsActivate(it settingsItem) (Model, tea.Cmd) {
	switch it.ZoneID {
	// account
	case "settings_oauth":
		m.oauthState = 1
		m.oauthInput.Placeholder = "Path to client_secret.json (or raw Client ID)"
		m.oauthInput.Reset()
		m.oauthInput.Focus()
		return m, textinput.Blink
	case "settings_auth_headers":
		m.authState = 1
		return m, m.openEditorForHeadersCmd()
	case "settings_history":
		m.statusMsg = "Listening history toggle (coming soon)"
		return m, nil

	// playback
	case "settings_repeat":
		return m.cycleRepeatMode()
	case "settings_shuffle":
		return m.toggleShuffle()
	case "settings_crossfade":
		return m.toggleCrossfade()
	case "settings_crossfade_val":
		return m.cycleCrossfadeSec()
	case "settings_sleep":
		return m.cycleSleepTimer()

	// audio
	case "settings_normalize":
		return m.toggleNormalize()
	case "settings_silence":
		return m.toggleSilenceSkip()
	case "settings_tempo_val":
		return m.resetTempo()
	case "settings_pitch_val":
		return m.resetPitch()
	case "settings_tempo_reset":
		mm, c1 := m.resetTempo()
		mmm, c2 := mm.resetPitch()
		return mmm, tea.Batch(c1, c2)
	case "settings_eq":
		return m.cycleEQPreset()

	// downloads
	case "settings_clear_cache":
		m.statusMsg = "Cache cleared (no files downloaded yet)"
		return m, nil
	case "settings_cache_size":
		m.statusMsg = "Calculating…"
		return m, nil
	case "settings_open_downloads":
		m.statusMsg = "Download folder: ./downloads/"
		return m, nil
	case "settings_dl_quality":
		m.statusMsg = "Download quality: Best Available"
		return m, nil

	// general
	case "settings_toggle_queue":
		m.queuePanelHidden = !m.queuePanelHidden
		m.markSessionDirty()
		return m, nil
	case "settings_remember_pos":
		m.statusMsg = "Remember position always enabled"
		return m, nil
	case "settings_clear_session":
		m.statusMsg = "Session cleared"
		return m, nil
	}
	return m, nil
}

// settingsInc increments a value item.
func (m Model) settingsInc(it settingsItem) (Model, tea.Cmd) {
	switch it.ZoneID {
	case "settings_tempo_val":
		return m.adjustTempo(0.05)
	case "settings_pitch_val":
		return m.adjustPitch(1)
	case "settings_crossfade_val":
		return m.stepCrossfadeSec(1)
	}
	return m, nil
}

// settingsDec decrements a value item.
func (m Model) settingsDec(it settingsItem) (Model, tea.Cmd) {
	switch it.ZoneID {
	case "settings_tempo_val":
		return m.adjustTempo(-0.05)
	case "settings_pitch_val":
		return m.adjustPitch(-1)
	case "settings_crossfade_val":
		return m.stepCrossfadeSec(-1)
	}
	return m, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

// generateSettingsContent renders the full two-panel settings layout.
func (m Model) generateSettingsContent(mainWidth int) string {
	const sidebarW = 22
	sidebar := m.renderSettingsSidebar(sidebarW)

	contentW := mainWidth - sidebarW - 3
	if contentW < 30 {
		contentW = 30
	}
	panel := m.renderSettingsPanel(contentW)

	sidebarH := strings.Count(sidebar, "\n") + 1
	panelH := strings.Count(panel, "\n") + 1
	divH := sidebarH
	if panelH > divH {
		divH = panelH
	}
	divLines := make([]string, divH)
	divStyle := lipgloss.NewStyle().Foreground(colorDivider).Background(colorBg)
	for i := range divLines {
		divLines[i] = divStyle.Render("│")
	}
	divider := strings.Join(divLines, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", divider, " ", panel)
}

// renderSettingsSidebar renders the left vertical tab list.
func (m Model) renderSettingsSidebar(w int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Width(w).
		Padding(0, 1).
		Background(colorBg)

	sb.WriteString(titleStyle.Render("Settings"))
	sb.WriteString("\n\n")

	for i, tab := range settingsTabs {
		active := m.settingsTab == tab.ID

		numHint := lipgloss.NewStyle().Foreground(colorDivider).Render(fmt.Sprintf("%d", i+1))
		var rowStyle lipgloss.Style
		if active {
			rowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent).
				Background(colorSearchBg).
				Width(w).
				Padding(0, 1)
		} else {
			rowStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				Background(colorBg).
				Width(w).
				Padding(0, 1)
		}

		label := fmt.Sprintf("%s  %s  %s", tab.Icon, tab.Label, numHint)
		row := m.zone.Mark("settings_tab_"+tab.ID, rowStyle.Render(label))
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	hintStyle := lipgloss.NewStyle().Foreground(colorDivider).Width(w).Padding(0, 1)
	sb.WriteString(hintStyle.Render("↑/↓   navigate rows"))
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("←/→   switch tabs"))
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Enter  activate"))
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("1–5    jump to tab"))
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Tab    switch pane"))

	return sb.String()
}

// renderSettingsPanel renders the right content area for the active tab.
func (m Model) renderSettingsPanel(w int) string {
	items := m.settingsItemsForTab(m.settingsTab)
	switch m.settingsTab {
	case "account":
		return m.renderSettingsAccount(w, items)
	case "playback":
		return m.renderSettingsPlayback(w, items)
	case "audio":
		return m.renderSettingsAudio(w, items)
	case "downloads":
		return m.renderSettingsDownloads(w, items)
	case "general":
		return m.renderSettingsGeneral(w, items)
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────────────────────────────────────

func settingsSectionHeader(title string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Render(title) + "\n\n"
}

func settingsCard(w int, content string) string {
	return lipgloss.NewStyle().
		Background(colorSearchBg).
		Width(w).
		Padding(1, 2).
		MarginBottom(1).
		Render(content) + "\n"
}

// settingsRow renders one settings row with cursor-aware focus highlight.
// rowIdx is its position in the settingsItemsForTab list.
func (m Model) renderSettingsRow(it settingsItem, rowIdx int, valStr string, w int) string {
	focused := m.activeMenu == "Settings" && m.settingsRow == rowIdx && it.Kind != kindInfo

	// Left column: cursor + label + description
	cursor := "  "
	labelColor := colorText
	descColor := colorSubtext
	bg := colorSearchBg // inside card already

	if focused {
		cursor = lipgloss.NewStyle().Foreground(colorAccent).Render("› ")
		labelColor = colorAccent
		_ = descColor
		_ = bg
	}
	if it.Kind == kindInfo {
		labelColor = colorSubtext
	}

	labelStyle := lipgloss.NewStyle().Foreground(labelColor).Bold(it.Kind != kindInfo)
	descStyle := lipgloss.NewStyle().Foreground(colorSubtext)

	left := lipgloss.JoinVertical(lipgloss.Left,
		cursor+labelStyle.Render(it.Label),
		"  "+descStyle.Render(it.Desc),
	)

	if it.Kind == kindInfo || valStr == "" {
		return left
	}

	// Right column: control widget
	ctrl := m.renderSettingsControl(it, valStr, focused)
	leftW := w - lipgloss.Width(ctrl) - 4
	if leftW < 20 {
		leftW = 20
	}
	leftFixed := lipgloss.NewStyle().Width(leftW).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Center, leftFixed, ctrl)
}

// renderSettingsControl renders the right-hand control widget for a row.
func (m Model) renderSettingsControl(it settingsItem, val string, focused bool) string {
	accentOrSub := colorSubtext
	if focused {
		accentOrSub = colorAccent
	}
	pillBg := colorFocusBg
	if focused {
		pillBg = colorSearchBg
	}

	switch it.Kind {
	case kindToggle:
		on := val == "On"
		var s string
		if on {
			s = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("● On ")
		} else {
			s = lipgloss.NewStyle().Foreground(colorSubtext).Render("○ Off")
		}
		return m.zone.Mark(it.ZoneID,
			lipgloss.NewStyle().Background(pillBg).Padding(0, 1).Render(s))

	case kindCycle:
		label := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(val)
		arrow := lipgloss.NewStyle().Foreground(accentOrSub).Render(" ↻")
		return m.zone.Mark(it.ZoneID,
			lipgloss.NewStyle().Background(pillBg).Padding(0, 1).Render(label+arrow))

	case kindValue:
		decBtn := m.zone.Mark(it.ZoneID+"_dec",
			lipgloss.NewStyle().Foreground(accentOrSub).Background(colorFocusBg).Padding(0, 1).Render("−"))
		valLabel := lipgloss.NewStyle().Foreground(colorText).Bold(true).Width(9).Align(lipgloss.Center).Render(val)
		incBtn := m.zone.Mark(it.ZoneID+"_inc",
			lipgloss.NewStyle().Foreground(accentOrSub).Background(colorFocusBg).Padding(0, 1).Render("+"))
		return lipgloss.JoinHorizontal(lipgloss.Center, decBtn, valLabel, incBtn)

	case kindAction:
		danger := it.ZoneID == "settings_clear_cache" || it.ZoneID == "settings_clear_session"
		btnColor := colorAccent
		if danger {
			btnColor = lipgloss.Color("#FF4444")
		}
		return m.zone.Mark(it.ZoneID,
			lipgloss.NewStyle().Foreground(btnColor).Background(pillBg).Padding(0, 2).Render("⏎ Activate"))
	}
	return ""
}

// settingsRowVal returns the current display value for a settings item.
func (m Model) settingsRowVal(it settingsItem) string {
	switch it.ZoneID {
	// toggles
	case "settings_history", "settings_remember_pos":
		return "Off" // placeholders
	case "settings_normalize":
		if m.normalize {
			return "On"
		}
		return "Off"
	case "settings_silence":
		if m.silenceSkip {
			return "On"
		}
		return "Off"
	case "settings_shuffle":
		if m.shuffle {
			return "On"
		}
		return "Off"
	case "settings_crossfade":
		if m.crossfade {
			return "On"
		}
		return "Off"
	case "settings_toggle_queue":
		if !m.queuePanelHidden {
			return "On"
		}
		return "Off"

	// cycles
	case "settings_repeat":
		return []string{"Off", "All", "One"}[m.repeatMode]
	case "settings_eq":
		return eqPresets[m.eqPreset].Name
	case "settings_crossfade_val":
		return m.crossfadeSecLabel()
	case "settings_sleep":
		if m.sleepMinutes == 0 {
			return "Off"
		}
		return fmt.Sprintf("%d min", m.sleepMinutes)
	case "settings_dl_quality":
		return "Best Available"

	// values
	case "settings_tempo_val":
		return fmt.Sprintf("%.2f×", m.tempo)
	case "settings_pitch_val":
		return fmt.Sprintf("%+.1f st", m.pitch)

	// actions
	case "settings_auth_headers":
		return ""
	case "settings_tempo_reset", "settings_open_downloads",
		"settings_cache_size", "settings_clear_cache", "settings_clear_session":
		return ""
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Tab content renderers
// ─────────────────────────────────────────────────────────────────────────────

func (m Model) renderSettingsAccount(w int, items []settingsItem) string {
	var sb strings.Builder
	sb.WriteString(settingsSectionHeader("Account"))

	if m.authState > 0 {
		sb.WriteString(settingsCard(w, m.renderAuthFlow()))
	} else if m.oauthState > 0 {
		sb.WriteString(settingsCard(w, m.renderOAuthFlow()))
	} else {
		rows := m.renderItemGroup(items, []int{0, 1}, w)
		sb.WriteString(settingsCard(w, rows))
	}

	sb.WriteString(settingsSectionHeader("Privacy"))
	rows := m.renderItemGroup(items, []int{2}, w)
	sb.WriteString(settingsCard(w, rows))

	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).
		Render("Provide client_secret.json path, or use browser headers via your editor."))
	return sb.String()
}

func (m Model) renderOAuthFlow() string {
	var sb strings.Builder
	switch m.oauthState {
	case 1:
		sb.WriteString(lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Enter path to client_secret.json (or raw Client ID):"))
		sb.WriteString("\n\n")
		sb.WriteString(m.oauthInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("Press Enter to continue  ·  Esc to cancel"))
	case 2:
		sb.WriteString(lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Enter Client Secret:"))
		sb.WriteString("\n\n")
		sb.WriteString(m.oauthInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("Press Enter to continue  ·  Esc to cancel"))
	case 3:
		sb.WriteString(lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Waiting for authorization..."))
		sb.WriteString("\n\n")
		if m.oauthCodeResp != nil {
			sb.WriteString(fmt.Sprintf("1. Open: %s\n", lipgloss.NewStyle().Foreground(colorAccent).Render(m.oauthCodeResp.VerificationURL)))
			sb.WriteString(fmt.Sprintf("2. Enter code: %s", lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render(m.oauthCodeResp.UserCode)))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("Generating code..."))
		}
	}
	return sb.String()
}

func (m Model) renderAuthFlow() string {
	if m.authState == 1 {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Connecting to YouTube Music…"),
			lipgloss.NewStyle().Foreground(colorSubtext).Render("Please complete sign-in in your text editor."),
		)
	}
	return lipgloss.NewStyle().Foreground(colorSubtext).Render("Authorising…")
}

func (m Model) renderSettingsPlayback(w int, items []settingsItem) string {
	var sb strings.Builder
	sb.WriteString(settingsSectionHeader("Queue & Navigation"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{0, 1}, w)))

	sb.WriteString(settingsSectionHeader("Crossfade"))
	crossfadeIdxs := []int{2}
	for i, it := range items {
		if it.ZoneID == "settings_crossfade_val" {
			crossfadeIdxs = append(crossfadeIdxs, i)
			break
		}
	}
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, crossfadeIdxs, w)))

	// sleep timer is always last
	lastIdx := len(items) - 1
	sb.WriteString(settingsSectionHeader("Sleep Timer"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{lastIdx}, w)))
	return sb.String()
}

func (m Model) renderSettingsAudio(w int, items []settingsItem) string {
	var sb strings.Builder
	sb.WriteString(settingsSectionHeader("Sound Processing"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{0, 1}, w)))

	sb.WriteString(settingsSectionHeader("Tempo & Pitch"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{2, 3, 4}, w)))

	sb.WriteString(settingsSectionHeader("Equalizer"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{5}, w)))

	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).
		Render("Keyboard: o norm  ·  </> tempo  ·  {/} pitch  ·  E EQ  ·  x crossfade"))
	return sb.String()
}

func (m Model) renderSettingsDownloads(w int, items []settingsItem) string {
	var sb strings.Builder
	sb.WriteString(settingsSectionHeader("Offline Storage"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{0, 1, 2}, w)))

	sb.WriteString(settingsSectionHeader("Download Quality"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{3}, w)))

	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).
		Render("Offline download requires a signed-in YouTube Music account."))
	return sb.String()
}

func (m Model) renderSettingsGeneral(w int, items []settingsItem) string {
	var sb strings.Builder
	sb.WriteString(settingsSectionHeader("Appearance"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{0}, w)))

	sb.WriteString(settingsSectionHeader("Session"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{1, 2}, w)))

	sb.WriteString(settingsSectionHeader("About"))
	sb.WriteString(settingsCard(w, m.renderItemGroup(items, []int{3}, w)))
	return sb.String()
}

// renderItemGroup renders a subset of items (by index) separated by dividers.
func (m Model) renderItemGroup(items []settingsItem, idxs []int, w int) string {
	var parts []string
	divStyle := lipgloss.NewStyle().Foreground(colorDivider).Render(strings.Repeat("─", max(10, w-6)))
	for _, idx := range idxs {
		if idx >= len(items) {
			continue
		}
		it := items[idx]
		val := m.settingsRowVal(it)
		parts = append(parts, m.renderSettingsRow(it, idx, val, w-6))
	}
	return strings.Join(parts, "\n"+divStyle+"\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
