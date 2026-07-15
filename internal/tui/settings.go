package tui

import (
	"fmt"
	"strings"

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

// generateSettingsContent renders the full two-panel settings layout.
func (m Model) generateSettingsContent(mainWidth int) string {
	// ── sidebar ──────────────────────────────────────────────────────────────
	const sidebarW = 22
	sidebar := m.renderSettingsSidebar(sidebarW)

	// ── content panel ────────────────────────────────────────────────────────
	contentW := mainWidth - sidebarW - 3 // 3 = divider + padding
	if contentW < 30 {
		contentW = 30
	}
	panel := m.renderSettingsPanel(contentW)

	// ── divider between sidebar and panel ────────────────────────────────────
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

	for _, tab := range settingsTabs {
		active := m.settingsTab == tab.ID

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

		label := fmt.Sprintf("%s  %s", tab.Icon, tab.Label)
		row := m.zone.Mark("settings_tab_"+tab.ID, rowStyle.Render(label))
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderSettingsPanel renders the right content area for the active tab.
func (m Model) renderSettingsPanel(w int) string {
	switch m.settingsTab {
	case "account":
		return m.renderSettingsAccount(w)
	case "playback":
		return m.renderSettingsPlayback(w)
	case "audio":
		return m.renderSettingsAudio(w)
	case "downloads":
		return m.renderSettingsDownloads(w)
	case "general":
		return m.renderSettingsGeneral(w)
	}
	return ""
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// settingsSectionHeader renders a bolded section heading.
func settingsSectionHeader(title string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Render(title) + "\n\n"
}

// settingsCard wraps a block of rows in a subtle card background.
func settingsCard(w int, content string) string {
	return lipgloss.NewStyle().
		Background(colorSearchBg).
		Width(w).
		Padding(1, 2).
		MarginBottom(1).
		Render(content) + "\n"
}

// settingsToggleRow renders: [Label]  [On/Off toggle]  [hint]
func (m Model) settingsToggleRow(label, desc, zoneID string, on bool) string {
	var toggleStr string
	if on {
		toggleStr = lipgloss.NewStyle().Foreground(colorAccent).Render("● On ")
	} else {
		toggleStr = lipgloss.NewStyle().Foreground(colorSubtext).Render("○ Off")
	}
	toggle := m.zone.Mark(zoneID, lipgloss.NewStyle().
		Background(colorFocusBg).
		Foreground(colorText).
		Padding(0, 1).
		Render(toggleStr))

	labelStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorSubtext)

	left := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(label),
		descStyle.Render(desc),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", 2), toggle) + "\n"
}

// settingsValueRow renders: [Label]  [− value +]
func (m Model) settingsValueRow(label, desc, val, zoneDecID, zoneIncID string) string {
	decBtn := m.zone.Mark(zoneDecID, lipgloss.NewStyle().Foreground(colorSubtext).Background(colorFocusBg).Padding(0, 1).Render("−"))
	incBtn := m.zone.Mark(zoneIncID, lipgloss.NewStyle().Foreground(colorSubtext).Background(colorFocusBg).Padding(0, 1).Render("+"))
	valStr := lipgloss.NewStyle().Foreground(colorText).Bold(true).Width(10).Align(lipgloss.Center).Render(val)
	controls := lipgloss.JoinHorizontal(lipgloss.Center, decBtn, valStr, incBtn)

	labelStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorSubtext)

	left := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(label),
		descStyle.Render(desc),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", 2), controls) + "\n"
}

// settingsCycleRow renders: [Label]  [← value →] (cycle through options)
func (m Model) settingsCycleRow(label, desc, val, zoneID string) string {
	btn := m.zone.Mark(zoneID, lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorFocusBg).
		Padding(0, 1).
		Render("↻  "+val))

	labelStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorSubtext)

	left := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(label),
		descStyle.Render(desc),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", 2), btn) + "\n"
}

// settingsActionRow renders: [Label]  [button text]
func (m Model) settingsActionRow(label, desc, btnText, zoneID string, danger bool) string {
	btnColor := colorAccent
	if danger {
		btnColor = lipgloss.Color("#FF4444")
	}
	btn := m.zone.Mark(zoneID, lipgloss.NewStyle().
		Foreground(btnColor).
		Background(colorFocusBg).
		Padding(0, 2).
		Render(btnText))

	labelStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorSubtext)

	left := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(label),
		descStyle.Render(desc),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", 2), btn) + "\n"
}

func settingsDivider(w int) string {
	return lipgloss.NewStyle().Foreground(colorDivider).Render(strings.Repeat("─", w)) + "\n\n"
}

// ── Account tab ──────────────────────────────────────────────────────────────

func (m Model) renderSettingsAccount(w int) string {
	var sb strings.Builder

	sb.WriteString(settingsSectionHeader("Account"))

	// Auth state card
	var authContent string
	switch {
	case m.oauthState == 0:
		statusLine := lipgloss.NewStyle().Foreground(colorSubtext).Render("Not signed in")
		btn := m.zone.Mark("settings_oauth", lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Background(colorFocusBg).
			Padding(0, 3).
			Render("Sign in to YouTube Music"))
		authContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("YouTube Music Account"),
			statusLine, "",
			btn,
		)
	case m.oauthState == 1:
		authContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Step 1 of 2 — Google Cloud Client ID"),
			lipgloss.NewStyle().Foreground(colorSubtext).Render("Create an OAuth app at console.cloud.google.com"),
			"",
			m.oauthInput.View(),
			"",
			lipgloss.NewStyle().Foreground(colorDivider).Render("Press Enter to continue  ·  Esc to cancel"),
		)
	case m.oauthState == 2:
		authContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Step 2 of 2 — Client Secret"),
			lipgloss.NewStyle().Foreground(colorSubtext).Render("Found in the same OAuth credentials screen"),
			"",
			m.oauthInput.View(),
			"",
			lipgloss.NewStyle().Foreground(colorDivider).Render("Press Enter to continue  ·  Esc to cancel"),
		)
	case m.oauthState == 3 && m.oauthCodeResp != nil:
		codeStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorFocusBg).Padding(0, 2)
		authContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Open this link in your browser:"),
			lipgloss.NewStyle().Foreground(colorSubtext).Render(m.oauthCodeResp.VerificationURL),
			"",
			lipgloss.NewStyle().Foreground(colorSubtext).Render("Then enter this code:"),
			codeStyle.Render("  "+m.oauthCodeResp.UserCode+"  "),
			"",
			lipgloss.NewStyle().Foreground(colorDivider).Render("Waiting for you to complete sign-in in the browser…"),
		)
	default:
		authContent = lipgloss.NewStyle().Foreground(colorSubtext).Render("Authorizing…")
	}

	sb.WriteString(settingsCard(w, authContent))

	sb.WriteString(settingsSectionHeader("Privacy"))
	privacyContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsToggleRow(
			"Send listening history",
			"Allows YouTube Music to use your listening history for recommendations",
			"settings_history", false,
		),
	)
	sb.WriteString(settingsCard(w, privacyContent))

	return sb.String()
}

// ── Playback tab ─────────────────────────────────────────────────────────────

func (m Model) renderSettingsPlayback(w int) string {
	var sb strings.Builder
	repeatLabels := []string{"Off", "All", "One"}

	sb.WriteString(settingsSectionHeader("Queue & Navigation"))
	queueContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsCycleRow(
			"Repeat Mode",
			"Loop the queue, or loop the current track",
			repeatLabels[m.repeatMode], "settings_repeat",
		),
		settingsDivider(w-8),
		m.settingsToggleRow(
			"Shuffle",
			"Play tracks in random order",
			"settings_shuffle", m.shuffle,
		),
	)
	sb.WriteString(settingsCard(w, queueContent))

	sb.WriteString(settingsSectionHeader("Crossfade"))
	crossfadeContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsToggleRow(
			"Enable Crossfade",
			"Smoothly blend between tracks at the end of each song",
			"settings_crossfade", m.crossfade,
		),
	)
	if m.crossfade {
		crossfadeContent = lipgloss.JoinVertical(lipgloss.Left,
			crossfadeContent,
			settingsDivider(w-8),
			m.settingsValueRow(
				"Duration",
				"How many seconds the fade overlap lasts",
				m.crossfadeSecLabel(),
				"settings_crossfade_dec", "settings_crossfade_inc",
			),
		)
	}
	sb.WriteString(settingsCard(w, crossfadeContent))

	sb.WriteString(settingsSectionHeader("Sleep Timer"))
	sleepLabel := "Off"
	if m.sleepMinutes > 0 {
		sleepLabel = fmt.Sprintf("%d min", m.sleepMinutes)
	}
	sleepContent := m.settingsCycleRow(
		"Sleep Timer",
		"Automatically pause after the selected time",
		sleepLabel, "settings_sleep",
	)
	sb.WriteString(settingsCard(w, sleepContent))

	return sb.String()
}

// ── Audio tab ────────────────────────────────────────────────────────────────

func (m Model) renderSettingsAudio(w int) string {
	var sb strings.Builder

	sb.WriteString(settingsSectionHeader("Sound Processing"))
	soundContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsToggleRow(
			"Loudness Normalization",
			"Level out volume differences between tracks",
			"settings_normalize", m.normalize,
		),
		settingsDivider(w-8),
		m.settingsToggleRow(
			"Silence Skip",
			"Automatically jump over silent passages",
			"settings_silence", m.silenceSkip,
		),
	)
	sb.WriteString(settingsCard(w, soundContent))

	sb.WriteString(settingsSectionHeader("Tempo & Pitch"))
	tpContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsValueRow(
			"Playback Speed",
			"Adjust how fast or slow tracks play (0.25× – 4.00×)",
			fmt.Sprintf("%.2f×", m.tempo),
			"settings_tempo_dec", "settings_tempo_inc",
		),
		settingsDivider(w-8),
		m.settingsValueRow(
			"Pitch Shift",
			"Shift pitch up or down in semitones (−12 to +12)",
			fmt.Sprintf("%+.1f st", m.pitch),
			"settings_pitch_dec", "settings_pitch_inc",
		),
		settingsDivider(w-8),
		m.settingsActionRow(
			"Reset Both",
			"Restore speed to 1.00× and pitch to 0 semitones",
			"Reset to Default", "settings_tempo_reset", false,
		),
	)
	sb.WriteString(settingsCard(w, tpContent))

	sb.WriteString(settingsSectionHeader("Equalizer"))
	eqContent := m.settingsCycleRow(
		"EQ Preset",
		"Applies an audio filter preset to the current and future tracks",
		eqPresets[m.eqPreset].Name, "settings_eq",
	)
	sb.WriteString(settingsCard(w, eqContent))

	sb.WriteString(lipgloss.NewStyle().
		Foreground(colorDivider).
		Render("Keyboard shortcuts:  o norm  ·  </>  tempo  ·  {/}  pitch  ·  E  EQ  ·  x  crossfade"))

	return sb.String()
}

// ── Downloads & Storage tab ──────────────────────────────────────────────────

func (m Model) renderSettingsDownloads(w int) string {
	var sb strings.Builder

	sb.WriteString(settingsSectionHeader("Offline Storage"))

	storageContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsActionRow(
			"Download Location",
			"Where cached audio files are saved  (./downloads/)",
			"Open Folder", "settings_open_downloads", false,
		),
		settingsDivider(w-8),
		m.settingsActionRow(
			"Cache Size",
			"Audio files stored locally for offline playback",
			"Calculate", "settings_cache_size", false,
		),
		settingsDivider(w-8),
		m.settingsActionRow(
			"Clear Cache",
			"Delete all downloaded audio files",
			"Clear All", "settings_clear_cache", true,
		),
	)
	sb.WriteString(settingsCard(w, storageContent))

	sb.WriteString(settingsSectionHeader("Download Quality"))
	qualityContent := m.settingsCycleRow(
		"Audio Quality",
		"Format preference when downloading for offline use",
		"Best Available", "settings_dl_quality",
	)
	sb.WriteString(settingsCard(w, qualityContent))

	sb.WriteString(lipgloss.NewStyle().Foreground(colorDivider).
		Render("Offline download is available once you have signed in to your account."))

	return sb.String()
}

// ── General tab ──────────────────────────────────────────────────────────────

func (m Model) renderSettingsGeneral(w int) string {
	var sb strings.Builder

	sb.WriteString(settingsSectionHeader("Appearance"))
	appearContent := m.settingsToggleRow(
		"Show Queue Panel",
		"Display the queue / lyrics panel in the right sidebar",
		"settings_toggle_queue", !m.queuePanelHidden,
	)
	sb.WriteString(settingsCard(w, appearContent))

	sb.WriteString(settingsSectionHeader("Session"))
	sessionContent := lipgloss.JoinVertical(lipgloss.Left,
		m.settingsToggleRow(
			"Remember Playback Position",
			"Resume from where you left off on restart",
			"settings_remember_pos", true,
		),
		settingsDivider(w-8),
		m.settingsActionRow(
			"Clear Session",
			"Reset the queue, history and resume position",
			"Clear", "settings_clear_session", true,
		),
	)
	sb.WriteString(settingsCard(w, sessionContent))

	sb.WriteString(settingsSectionHeader("About"))
	aboutContent := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("go-ytm"),
		lipgloss.NewStyle().Foreground(colorSubtext).Render("Terminal YouTube Music client"),
		"",
		lipgloss.NewStyle().Foreground(colorDivider).Render("Built with mpv · ytmusicapi · kkdai/youtube · bubbletea"),
	)
	sb.WriteString(settingsCard(w, aboutContent))

	return sb.String()
}
