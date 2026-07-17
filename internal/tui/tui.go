package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/judeadeniji/go-ytm/internal/library"
	"github.com/judeadeniji/go-ytm/internal/lyrics"
	"github.com/judeadeniji/go-ytm/internal/player"
	"github.com/judeadeniji/go-ytm/internal/search"
	"github.com/judeadeniji/go-ytm/internal/session"
	"github.com/judeadeniji/go-ytm/internal/ytmapi"
	zone "github.com/lrstanley/bubblezone"
)

type Model struct {
	width  int
	height int

	activePane     Pane
	activeCarousel int

	menuItems  []string
	activeMenu string
	playlists  [][2]string

	filters []string

	homeCarousels     []ytmapi.HomeCarousel
	carouselOffsets   map[string]int
	cachedArt         *KittyImage
	imageCache        map[string]*KittyImage
	imageCacheOrder   []string // oldest-first keys for LRU eviction
	mainViewport      viewport.Model
	leftViewport      viewport.Model
	rightViewport     viewport.Model
	lyricsViewport    viewport.Model
	searchInput       textinput.Model
	searchSuggestions []SearchSuggestion
	suggestionGen     int // bumps on each query change; ignores stale fetches
	sugCancel         context.CancelFunc
	zone              *zone.Manager
	
	authState         int // 0: None, 1: Connecting
	oauthState        int // 0: None, 1: Entering Client ID/Path, 2: Entering Client Secret, 3: Waiting
	settingsTab       string
	settingsRow       int // focused row index in the current settings tab
	oauthInput        textinput.Model
	oauthClientID     string
	oauthClientSecret string
	oauthCodeResp     *ytmapi.OAuthCodeResponse

	searchResults   []ytmapi.SearchResult
	searchFilter    string // api filter: "", songs, albums, artists, playlists
	lastSearchQuery string
	ytmapiClient    *ytmapi.Client
	lyricsClient    *lyrics.Client

	player    *player.Player
	extractor *search.Extractor
	statusMsg string

	nowPlayingOpen     bool
	lyricsLoading      bool
	lyricsInstrumental bool
	lyricsErr          string
	lyricsPlain        string
	lyricsLines        []lyrics.Line
	lyricsTrackKey     string
	lyricsGen          int
	lyricsCancel       context.CancelFunc
	lyricsFollow       bool      // auto-center on the singing line
	lyricsCursor       int       // keyboard-focused line (-1 = none)
	lyricsIdleAt       time.Time // last browse/scroll; smart resync after idle
	lyricsFetchDur     float64   // playDuration used for last lyrics fetch

	songDetails        *ytmapi.SongDetails
	songDetailsVideoID string
	songDetailsGen     int
	songDetailsLoading bool
	songDetailsErr     string
	songDetailsCancel  context.CancelFunc

	// Playback state
	queue            Queue
	currentTrack     *Track
	isPlaying        bool
	playPos          float64
	playDuration     float64
	playBuffered     float64 // demuxer cache end (seconds) for buffer overlay
	scrubbing        bool    // true while dragging/clicking the progress bar
	scrubPos         float64 // preview position while scrubbing
	volume           float64 // 0–100 UI/mpv volume
	muted            bool
	normalize        bool      // loudnorm af
	silenceSkip      bool      // silenceremove af
	tempo            float64   // speed multiplier
	pitch            float64   // pitch shift in semitones
	eqPreset         int       // index into eqPresets
	repeatMode       int       // 0=Off, 1=All, 2=One
	shuffle          bool      // shuffle state
	sleepUntil       time.Time // zero = sleep timer off
	sleepMinutes     int       // last set duration in the cycle (0/15/30/45/60)
	crossfade        bool      // gapless append + volume dip (off by default)
	crossfadeSec     int       // fade/handoff window seconds
	armedVideoID     string    // next track appended into mpv playlist
	armInflight      string    // video id currently being appended
	crossfadeFading  bool      // volume dip in progress
	streamCache      map[string]cachedStream
	preloadInflight  map[string]struct{}
	preloadCancel    map[string]context.CancelFunc
	lastStreamCached bool // current load used a prefetched URL (retry-extract on fail)
	queuePanelHidden bool // user dismissed the right rail
	playGen          int  // bumped on each play request; ignores stale extracts
	playCancel       context.CancelFunc
	playCtx          context.Context
	lastRailClockSec int // throttles rail rebuilds to once per displayed second

	// Navigation / detail pages
	stack          ViewStack
	navGen         int // bumped on each page/search fetch; ignores stale replies
	pageLoading    bool
	pageErr        string
	artistPage     *ytmapi.ArtistPage
	albumPage      *ytmapi.AlbumPage
	playlistPage   *ytmapi.PlaylistPage
	trackCursor    int // focus index within playlist/album tracklist
	listCursor     int // search / artist / sidebar / suggestions
	homeCardCursor int // card index within active home carousel
	queueCursor    int // focus in queue panel (separate from playing index)
	railTab        RailTab

	sessionStore      *library.DB
	sessionDirty      bool
	lastSessionPosSec int     // throttle play_pos session dirty to 1Hz
	audioLoaded       bool    // mpv has the current track loaded
	resumeSeek        float64 // seek here after load (session restore)
	resumeSeekTries   int     // remaining progress-tick seek nudges

	navCancel context.CancelFunc
	navCtx    context.Context

	// Explore state
	exploreData       *ytmapi.ExploreData
	exploreLoading    bool
	exploreErr        string
	moodCategories    map[string][]ytmapi.MoodCategory
	moodCatsLoading   bool
	activeMoodSection string
	activeMoodParams  string
	moodPlaylists     []map[string]any
	chartsData        *ytmapi.ChartsData
	chartsLoading     bool
	chartsCountry     string
	exploreSubTab     string // "overview" | "moods" | "charts"

	// imageDirty is true when thumbs arrived and a debounced redraw is pending.
	imageDirty bool
}

func NewModel(p *player.Player, ext *search.Extractor, apiClient *ytmapi.Client, lyricsClient *lyrics.Client) Model {
	// Pre-render the image once at startup!
	artStr := RenderLocalImage(".build_assets/2026-07-14_05-43.png", artWidth, artHeight, hashString(".build_assets/2026-07-14_05-43.png"))

	// Initialize interactive search input
	ti := textinput.New()
	ti.Placeholder = "Search songs, albums, artists, podcasts"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSubtext)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorText)
	ti.CharLimit = 156
	ti.Width = 56 // Leave room for padding

	oti := textinput.New()
	oti.Placeholder = "Path to client_secret.json (or raw Client ID)"
	oti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSubtext)
	oti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	oti.Cursor.Style = lipgloss.NewStyle().Foreground(colorText)
	oti.CharLimit = 200
	oti.Width = 60

	store, err := library.Open()
	status := "Ready"
	if err != nil {
		status = "Session store unavailable"
		store = nil
	}

	return Model{
		activePane:     PaneMain,
		activeCarousel: 0,
		menuItems:      []string{"Home", "Explore", "Library", "Settings"},
		activeMenu:     "Home",
		playlists: [][2]string{
			{"Liked Music", "📌 Auto playlist"},
			{"TikTok Songs", "Oluwaferanmi A.J"},
			{"Elite Raps..", "Misfit"},
			{"2022 Dump", "Oluwaferanmi A.J"},
			{"2025 Recap", "Made for Oluwaferanmi A.J"},
			{"This ain't Odumodu Blvck", "Oluwaferanmi A.J"},
			{"Violin Classics", "Oluwaferanmi A.J"},
		},
		filters:           []string{"All", "Songs", "Albums", "Artists", "Playlists"},
		searchFilter:      "",
		homeCarousels:     nil,
		searchSuggestions: []SearchSuggestion{},
		carouselOffsets:   make(map[string]int),
		cachedArt:         &artStr,
		imageCache:        make(map[string]*KittyImage),
		mainViewport:      viewport.New(0, 0),
		leftViewport:      viewport.New(0, 0),
		rightViewport:     viewport.New(0, 0),
		lyricsViewport:    viewport.New(0, 0),
		searchInput:       ti,
		oauthInput:        oti,
		settingsTab:       "account",
		zone:              zone.New(),
		searchResults:     nil,
		ytmapiClient:      apiClient,
		lyricsClient:      lyricsClient,
		player:            p,
		extractor:         ext,
		statusMsg:         status,
		queue:             Queue{current: -1},
		volume:            100,
		tempo:             1.0,
		pitch:             0.0,
		crossfadeSec:      session.DefaultCrossfadeSec,
		sessionStore:      store,
		lyricsFollow:      true,
		lyricsCursor:      -1,
		streamCache:       make(map[string]cachedStream),
		preloadInflight:   make(map[string]struct{}),
		preloadCancel:     make(map[string]context.CancelFunc),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadSession(m.sessionStore),
		fetchHome(m.ytmapiClient),
		tickPlayProgress(),
		listenTrackEnded(m.player),
		tickSessionPersist(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, m.enqueueVisibleImages(m.mainWidth())

	case tea.KeyMsg:
		if m.oauthState == 1 || m.oauthState == 2 {
			switch msg.String() {
			case "esc":
				m.oauthState = 0
				m.oauthInput.Blur()
				m.setMainContent()
				return m, nil
			case "enter":
				val := strings.TrimSpace(m.oauthInput.Value())
				if m.oauthState == 1 {
					if val != "" {
						if strings.HasSuffix(val, ".json") {
							// Simple heuristic: if it ends with .json, parse it.
							// For real we'd os.ReadFile and json.Unmarshal. Let's do that.
							if _, err := os.Stat(os.ExpandEnv(val)); err == nil {
								// Parse {"installed": {"client_id": "...", "client_secret": "..."}}
								// To avoid heavy imports, let's just do regex or basic struct.
								// Actually better: just send a tea.Cmd that reads the file.
								return m, m.parseClientSecretCmd(val)
							}
						}
						m.oauthClientID = val
					}
					m.oauthState = 2
					m.oauthInput.Reset()
					m.oauthInput.Placeholder = "Client Secret"
					m.setMainContent()
					return m, nil
				} else if m.oauthState == 2 {
					m.oauthClientSecret = val
					m.oauthState = 3
					m.oauthInput.Blur()
					m.statusMsg = "Requesting OAuth code..."
					m.setMainContent()
					return m, m.fetchOAuthCodeCmd()
				}
			}
			var cmd tea.Cmd
			m.oauthInput, cmd = m.oauthInput.Update(msg)
			m.setMainContent()
			return m, cmd
		}

		// If the search bar is focused, hijack keyboard events
		if m.searchInput.Focused() {
			switch msg.String() {
			case "enter":
				mm, cmd := m.activateSuggestion()
				return mm, cmd
			case "esc":
				m.searchInput.Blur()
				m.searchSuggestions = nil
				m.setMainContent()
				return m, nil
			case "down":
				m = m.moveSuggestionFocus(1)
				return m, nil
			case "up":
				m = m.moveSuggestionFocus(-1)
				return m, nil
			}

			var cmd tea.Cmd
			oldVal := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(msg)
			newVal := m.searchInput.Value()

			if newVal != oldVal {
				m.listCursor = 0
				m.suggestionGen++
				m.cancelSuggestions()
				if strings.TrimSpace(newVal) == "" {
					m.searchSuggestions = nil
					return m, cmd
				}
				return m, tea.Batch(cmd, debounceSuggestions(newVal, m.suggestionGen))
			}
			m.setMainContent()
			return m, cmd
		}

		// Settings page keyboard nav (intercept before global keys)
		if m.activeMenu == "Settings" && !m.nowPlayingOpen {
			if mm, cmd, handled := m.HandleSettingsKey(msg.String()); handled {
				return mm, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.cancelPlayExtract()
			m.cancelAllPrefetches()
			m.cancelSuggestions()
			m.cancelLyrics()
			m.cancelSongDetails()
			// Sync the latest mpv position into the snapshot before we exit.
			if m.audioLoaded && m.player != nil {
				if pos, err := m.player.PositionSeconds(); err == nil && pos >= 0 {
					m.playPos = pos
				}
				if dur, err := m.player.DurationSeconds(); err == nil && dur > 0 {
					m.playDuration = dur
				}
			}
			m.markSessionDirty()
			return m, tea.Sequence(
				saveSession(m.sessionStore, m.snapshot()),
				closeSession(m.sessionStore),
				tea.Quit,
			)
		case "esc", "backspace":
			if m.nowPlayingOpen {
				return m.closeNowPlaying(), nil
			}
			mm, cmd := m.popNav()
			mm.markSessionDirty()
			return mm, cmd
		case "f":
			if m.currentTrack == nil && !m.nowPlayingOpen {
				return m, nil
			}
			return m.toggleNowPlaying()
		case "tab":
			if m.nowPlayingOpen {
				// Cycle between NP body and queue rail only.
				if m.activePane == PaneQueue {
					m.activePane = PaneMain
				} else if m.showQueuePanel() {
					m.activePane = PaneQueue
					m.queueCursor = m.queue.CurrentIndex()
					if m.queueCursor < 0 {
						m.queueCursor = 0
					}
					m.setQueuePanelContent()
				}
				return m, nil
			}
			m.activePane = m.nextPane()
			if m.activePane == PaneSidebar {
				m.listCursor = 0
				for i, item := range m.menuItems {
					if item == m.activeMenu {
						m.listCursor = i
						break
					}
				}
				m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
			}
			if m.activePane == PaneQueue {
				m.queueCursor = m.queue.CurrentIndex()
				if m.queueCursor < 0 {
					m.queueCursor = 0
				}
				m.setQueuePanelContent()
			}
			return m, nil
		case "shift+tab":
			if m.nowPlayingOpen {
				if m.activePane == PaneMain && m.showQueuePanel() {
					m.activePane = PaneQueue
					m.queueCursor = m.queue.CurrentIndex()
					if m.queueCursor < 0 {
						m.queueCursor = 0
					}
					m.setQueuePanelContent()
				} else {
					m.activePane = PaneMain
				}
				return m, nil
			}
			m.activePane = m.prevPane()
			if m.activePane == PaneSidebar {
				m.listCursor = 0
				for i, item := range m.menuItems {
					if item == m.activeMenu {
						m.listCursor = i
						break
					}
				}
				m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
			}
			if m.activePane == PaneQueue {
				m.queueCursor = m.queue.CurrentIndex()
				if m.queueCursor < 0 {
					m.queueCursor = 0
				}
				m.setQueuePanelContent()
			}
			return m, nil
		case "\\":
			m.queuePanelHidden = !m.queuePanelHidden
			m.markSessionDirty()
			m.applyLayout()
			return m, m.enqueueVisibleImages(m.mainWidth())
		case "]":
			if !m.showQueuePanel() {
				return m, nil
			}
			var cmd tea.Cmd
			m, cmd = m.cycleRailTab(1)
			m.activePane = PaneQueue
			return m, cmd
		case "[":
			if !m.showQueuePanel() {
				return m, nil
			}
			var cmd tea.Cmd
			m, cmd = m.cycleRailTab(-1)
			m.activePane = PaneQueue
			return m, cmd
		case "/":
			if m.nowPlayingOpen {
				m = m.closeNowPlaying()
			}
			m.listCursor = 0
			m.searchInput.Focus()
			m.suggestionGen++
			q := m.searchInput.Value()
			cmds := []tea.Cmd{textinput.Blink}
			if strings.TrimSpace(q) != "" {
				cmds = append(cmds, debounceSuggestions(q, m.suggestionGen))
			} else {
				m.searchSuggestions = nil
			}
			return m, tea.Batch(cmds...)
		case "p":
			return m.togglePlayPause()
		case " ":
			if m.currentTrack != nil {
				return m.togglePlayPause()
			}
			return m.activateFocused()
		case "s":
			m.playGen++
			m.cancelPlayExtract()
			m.cancelAllPrefetches()
			m.lastStreamCached = false
			m.statusMsg = "Stopped playback"
			m.isPlaying = false
			m.audioLoaded = false
			m.playPos = 0
			m.playDuration = 0
			m.playBuffered = 0
			m.clearResumeSeek()
			m.markSessionDirty()
			m.setQueuePanelContent()
			return m, stopPlayback(m.player)
		case "a":
			return m.goToPlayingAlbum()
		case "n":
			return m.playNext()
		case "b":
			return m.playPrev()
		case "enter":
			if m.nowPlayingOpen {
				if m.activePane == PaneQueue {
					return m.activateFocused()
				}
				if len(m.lyricsLines) > 0 {
					cur := m.lyricsCursor
					if cur < 0 || m.lyricsFollow {
						pos := time.Duration(m.playPos * float64(time.Second))
						cur = lyrics.ActiveLineIndex(m.lyricsLines, pos)
					}
					return m.seekToLyricsLine(cur)
				}
				return m, nil
			}
			return m.activateFocused()
		case "c", "r":
			if m.nowPlayingOpen && m.activePane != PaneQueue {
				m.resyncLyricsFollow()
				m.syncLyricsFollowOffset()
				m.statusMsg = "Lyrics following"
				return m, nil
			}
			return m, nil
		case "-":
			return m.adjustVolume(-5)
		case "=", "+":
			return m.adjustVolume(5)
		case "m":
			return m.toggleMute()
		case "o":
			return m.toggleNormalize()
		case "v":
			return m.toggleSilenceSkip()
		case "t":
			return m.cycleSleepTimer()
		case "x":
			return m.toggleCrossfade()
		case "X":
			return m.cycleCrossfadeSec()
		case "<":
			if m.activePane == PaneMain && m.currentScreen() == screenArtist {
				if mm, handled := m.moveArtistCarousel(-1); handled {
					return mm, mm.enqueueVisibleImages(mm.mainWidth())
				}
			}
			return m.adjustTempo(-0.05)
		case ">":
			if m.activePane == PaneMain && m.currentScreen() == screenArtist {
				if mm, handled := m.moveArtistCarousel(1); handled {
					return mm, mm.enqueueVisibleImages(mm.mainWidth())
				}
			}
			return m.adjustTempo(0.05)
		case "{":
			return m.adjustPitch(-0.5)
		case "}":
			return m.adjustPitch(0.5)
		case "E":
			return m.cycleEQPreset()
		case "R":
			return m.cycleRepeatMode()
		case "S":
			return m.toggleShuffle()
		case ",":
			if m.activePane == PaneMain && m.currentScreen() == screenArtist {
				if mm, handled := m.moveArtistCarousel(-1); handled {
					return mm, mm.enqueueVisibleImages(mm.mainWidth())
				}
			}
			if m.currentTrack != nil {
				m.clearResumeSeek()
				return m, tea.Batch(seekCmd(m.player, -5), fetchPlayProgress(m.player))
			}
			return m, nil
		case ".":
			if m.activePane == PaneMain && m.currentScreen() == screenArtist {
				if mm, handled := m.moveArtistCarousel(1); handled {
					return mm, mm.enqueueVisibleImages(mm.mainWidth())
				}
			}
			if m.currentTrack != nil {
				m.clearResumeSeek()
				return m, tea.Batch(seekCmd(m.player, 5), fetchPlayProgress(m.player))
			}
			return m, nil
		case "right", "l":
			if m.nowPlayingOpen {
				if m.currentTrack != nil {
					m.clearResumeSeek()
					return m, tea.Batch(seekCmd(m.player, 5), fetchPlayProgress(m.player))
				}
				return m, nil
			}
			if m.activePane == PaneMain {
				if m.onHomeScreen() || (m.currentScreen() == screenExplore && m.exploreSubTab == "overview") {
					return m.moveHomeCard(1)
				}
				if m.currentScreen() == screenArtist {
					if mm, handled := m.moveArtistCarousel(1); handled {
						return mm, mm.enqueueVisibleImages(mm.mainWidth())
					}
				}
			}
			if m.currentTrack != nil {
				m.clearResumeSeek()
				return m, tea.Batch(seekCmd(m.player, 5), fetchPlayProgress(m.player))
			}
			return m, nil
		case "left", "h":
			if m.nowPlayingOpen {
				if m.currentTrack != nil {
					m.clearResumeSeek()
					return m, tea.Batch(seekCmd(m.player, -5), fetchPlayProgress(m.player))
				}
				return m, nil
			}
			if m.activePane == PaneMain {
				if m.onHomeScreen() || (m.currentScreen() == screenExplore && m.exploreSubTab == "overview") {
					return m.moveHomeCard(-1)
				}
				if m.currentScreen() == screenArtist {
					if mm, handled := m.moveArtistCarousel(-1); handled {
						return mm, mm.enqueueVisibleImages(mm.mainWidth())
					}
				}
			}
			if m.currentTrack != nil {
				m.clearResumeSeek()
				return m, tea.Batch(seekCmd(m.player, -5), fetchPlayProgress(m.player))
			}
			return m, nil
		case "up", "k":
			if m.nowPlayingOpen && m.activePane != PaneQueue {
				m.moveLyricsCursor(-1)
				return m, nil
			}
			if mm, handled := m.moveListFocus(-1); handled {
				return mm, mm.enqueueVisibleImages(mm.mainWidth())
			}
			return m, nil
		case "down", "j":
			if m.nowPlayingOpen && m.activePane != PaneQueue {
				m.moveLyricsCursor(1)
				return m, nil
			}
			if mm, handled := m.moveListFocus(1); handled {
				return mm, mm.enqueueVisibleImages(mm.mainWidth())
			}
			return m, nil
		case "pgup", "ctrl+u":
			if m.nowPlayingOpen && m.activePane != PaneQueue {
				m.hydrateLyricsViewport()
				m.pauseLyricsFollow()
				m.lyricsViewport.ViewUp()
				if len(m.lyricsLines) > 0 {
					m.lyricsCursor = clampIndex(m.lyricsViewport.YOffset, len(m.lyricsLines))
				}
				m.syncLyricsFollowOffset()
				return m, nil
			}
			switch m.activePane {
			case PaneSidebar:
				m.leftViewport.ViewUp()
			case PaneQueue:
				m.rightViewport.ViewUp()
			default:
				m.mainViewport.ViewUp()
			}
			return m, nil
		case "pgdown", "ctrl+d":
			if m.nowPlayingOpen && m.activePane != PaneQueue {
				m.hydrateLyricsViewport()
				m.pauseLyricsFollow()
				m.lyricsViewport.ViewDown()
				if len(m.lyricsLines) > 0 {
					m.lyricsCursor = clampIndex(m.lyricsViewport.YOffset+m.lyricsViewport.Height-1, len(m.lyricsLines))
				}
				m.syncLyricsFollowOffset()
				return m, nil
			}
			switch m.activePane {
			case PaneSidebar:
				m.leftViewport.ViewDown()
			case PaneQueue:
				m.rightViewport.ViewDown()
			default:
				m.mainViewport.ViewDown()
			}
			return m, nil
		}
		return m, nil
	case TrackStartedMsg:
		if msg.Gen != 0 && msg.Gen != m.playGen {
			return m, nil // superseded by a newer skip
		}
		if msg.Err != nil && !msg.SeekOnlyErr {
			if m.lastStreamCached && msg.Track.VideoID != "" {
				// Prefetched URL went stale — drop it and re-extract once.
				m.invalidateStreamCache(msg.Track.VideoID)
				m.lastStreamCached = false
				m.statusMsg = "Retrying: " + msg.Track.Title
				return m, tea.Sequence(
					stopPlayback(m.player),
					playTrack(m.extractor, msg.Track, m.playGen, m.playCtx),
				)
			}
			m.isPlaying = false
			m.audioLoaded = false
			m.statusMsg = shortStreamErr(msg.Err)
			return m, nil
		}
		m.lastStreamCached = false
		if msg.URL != "" && msg.Track.VideoID != "" {
			m.putStreamCache(msg.Track.VideoID, msg.URL)
		}
		m.currentTrack = &msg.Track
		m.isPlaying = true
		m.audioLoaded = true
		if m.resumeSeek >= 0.5 {
			m.playPos = m.resumeSeek
			// Keep resumeSeek until progress confirms we're near the target.
		} else if m.playPos < 0.5 {
			m.playPos = 0
		}
		// Keep restored/session duration until mpv reports a real one.
		if m.resumeSeek < 0.5 {
			m.playDuration = 0
			m.playBuffered = 0
		} else if m.playBuffered < m.playPos {
			m.playBuffered = m.playPos
		}
		if msg.SeekOnlyErr {
			m.statusMsg = "Playing (seek failed): " + msg.Track.Title
			if m.resumeSeek >= 0.5 {
				m.playPos = m.resumeSeek
			}
		} else {
			m.statusMsg = "Playing: " + msg.Track.Title
		}
		m.markSessionDirty()
		if m.onTracklistScreen() {
			m = m.syncTrackCursorToPlaying()
			m.ensureTrackCursorInView(10, 1)
			m.setMainContent()
		}
		showing := m.showQueuePanel()
		m.applyLayout()
		if showing {
			m.queueCursor = m.queue.CurrentIndex()
			if m.queueCursor < 0 {
				m.queueCursor = 0
			}
		}
		m.setQueuePanelContent()
		cmds := []tea.Cmd{fetchPlayProgress(m.player), m.enqueueVisibleImages(m.mainWidth())}
		if cmd := m.ensureSongDetailsFetched(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.nowPlayingOpen || (m.showQueuePanel() && m.railTab == RailDetails) {
			if cmd := m.ensureLyricsFetched(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if m.nowPlayingOpen {
			m.ensureNowPlayingLayout()
			cmds = append(cmds, m.enqueueNowPlayingImage())
		}
		if cmd := m.ensureUpcomingPreloaded(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.ensureUpcomingArmed(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	case SongDetailsMsg:
		if msg.Gen != 0 && msg.Gen != m.songDetailsGen {
			return m, nil
		}
		if m.currentTrack == nil || msg.VideoID != m.currentTrack.VideoID {
			return m, nil
		}
		m.songDetailsLoading = false
		if msg.Err != nil {
			if errors.Is(msg.Err, context.Canceled) {
				return m, nil
			}
			m.songDetailsErr = "Metadata unavailable"
			m.songDetails = nil
			m.setQueuePanelContent()
			return m, nil
		}
		m.songDetailsErr = ""
		m.songDetails = msg.Song
		hadThumb := m.currentTrack != nil && m.currentTrack.ThumbnailURL != ""
		m.applySongDetails(msg.Song)
		m.setQueuePanelContent()
		var cmds []tea.Cmd
		if m.nowPlayingOpen {
			m.setMainContent()
			if !hadThumb && m.currentTrack != nil && m.currentTrack.ThumbnailURL != "" {
				cmds = append(cmds, m.enqueueNowPlayingImage())
			}
		}
		return m, tea.Batch(cmds...)
	case LyricsMsg:
		if msg.Gen != 0 && msg.Gen != m.lyricsGen {
			return m, nil
		}
		if msg.TrackKey != "" && msg.TrackKey != m.lyricsTrackKey {
			return m, nil
		}
		m.lyricsLoading = false
		if msg.Err != nil {
			if errors.Is(msg.Err, context.Canceled) {
				return m, nil
			}
			m.lyricsErr = "No lyrics found"
			m.lyricsLines = nil
			m.lyricsPlain = ""
			m.lyricsInstrumental = false
			if m.showQueuePanel() {
				m.setQueuePanelContent()
			}
			return m, nil
		}
		m.lyricsErr = ""
		m.lyricsInstrumental = msg.Instrumental
		m.lyricsPlain = msg.Plain
		keepBrowse := !m.lyricsFollow
		m.lyricsLines = msg.Lines
		if keepBrowse {
			// Duration-refine / same-track refresh must not yank user scroll.
		} else {
			m.lyricsFollow = true
			m.lyricsIdleAt = time.Time{}
		}
		if len(m.lyricsLines) > 0 {
			pos := time.Duration(m.playPos * float64(time.Second))
			if m.lyricsFollow || m.lyricsCursor < 0 {
				m.lyricsCursor = lyrics.ActiveLineIndex(m.lyricsLines, pos)
			}
		} else {
			m.lyricsCursor = -1
		}
		if m.showQueuePanel() {
			m.setQueuePanelContent()
		}
		if m.nowPlayingOpen {
			m.syncLyricsFollowOffset()
		}
		return m, nil
	case playerErrMsg:
		if msg.Err != nil {
			if msg.Op == "pause" {
				m.isPlaying = !m.isPlaying
				if m.onTracklistScreen() {
					m.setMainContent()
				}
			}
			if msg.Op == "mute" {
				m.muted = !m.muted
			}
			if msg.Op == "volume" {
				// Best-effort: re-read is hard; surface error only.
			}
			op := msg.Op
			if op == "" {
				op = "player"
			}
			m.statusMsg = fmt.Sprintf("%s failed: %v", op, msg.Err)
		}
		return m, nil
	case streamReadyMsg:
		if msg.Gen != m.playGen {
			return m, nil // user already skipped ahead
		}
		if msg.Err != nil {
			if errors.Is(msg.Err, context.Canceled) {
				return m, nil
			}
			m.lastStreamCached = false
			m.statusMsg = shortStreamErr(msg.Err)
			m.isPlaying = false
			return m, nil
		}
		m.lastStreamCached = msg.Cached
		// Don't warm-cache until Load/Play succeeds (see TrackStartedMsg).
		m.statusMsg = "Starting: " + msg.Track.Title
		seekTo := m.resumeSeek
		return m, loadTrack(m.player, msg.Track, msg.URL, msg.Gen, seekTo, m.playCtx)
	case streamPreloadMsg:
		delete(m.preloadInflight, msg.VideoID)
		delete(m.preloadCancel, msg.VideoID)
		if msg.Err != nil {
			if errors.Is(msg.Err, context.Canceled) {
				return m, nil
			}
			return m, nil
		}
		if msg.URL == "" || !m.queueContainsVideo(msg.VideoID) {
			return m, nil
		}
		m.putStreamCache(msg.VideoID, msg.URL)
		return m, nil
	case sleepTickMsg:
		return m.handleSleepTick()
	case sessionLoadedMsg:
		if msg.Err != nil {
			m.statusMsg = "Session load failed"
			return m, nil
		}
		cmd := m.applySnapshot(msg.Snap)
		m.applyLayout()
		m.setQueuePanelContent()
		if m.leftViewport.Width > 0 {
			m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
		}
		var cmds []tea.Cmd
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.nowPlayingOpen && m.currentTrack != nil {
			m.lyricsFollow = true
			m.lyricsIdleAt = time.Time{}
			m.ensureNowPlayingLayout()
			m.setMainContent()
			cmds = append(cmds, m.enqueueNowPlayingImage())
			if c := m.ensureLyricsFetched(); c != nil {
				cmds = append(cmds, c)
			}
			if c := m.ensureSongDetailsFetched(); c != nil {
				cmds = append(cmds, c)
			}
		}
		return m, tea.Batch(cmds...)
	case sessionPersistTickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, tickSessionPersist())
		if m.sessionDirty {
			snap := m.snapshot()
			m.sessionDirty = false
			cmds = append(cmds, saveSession(m.sessionStore, snap))
		}
		return m, tea.Batch(cmds...)
	case sessionSavedMsg:
		if msg.Err != nil {
			m.sessionDirty = true
			m.statusMsg = "Session save failed"
		}
		return m, nil
	case trackEndedMsg:
		if msg.Closed {
			m.statusMsg = "Playback engine disconnected"
			m.isPlaying = false
			return m, nil
		}
		// Always re-arm the listener; only natural EOF advances the queue.
		// loadfile/stop emit reason "stop" (or similar) — ignore those.
		rearm := listenTrackEnded(m.player)
		if msg.Reason != "eof" {
			return m, rearm
		}
		// Gapless: mpv already advanced to the appended next track.
		if m.crossfade && m.armedVideoID != "" {
			nextIdx := m.queue.CurrentIndex() + 1
			if next, ok := m.queue.At(nextIdx); ok && next.VideoID == m.armedVideoID {
				mm, cmd := m.promoteArmedTrack()
				return mm, tea.Batch(cmd, rearm)
			}
		}
		m.clearCrossfadeArmState()
		mm, cmd := m.playNextWithManual(false)
		if cmd == nil {
			// End of queue
			mm.isPlaying = false
			mm.statusMsg = "End of queue"
			return mm, rearm
		}
		return mm, tea.Batch(cmd, rearm)
	case streamArmMsg:
		if msg.VideoID == "" {
			return m, nil
		}
		if m.armInflight == msg.VideoID {
			m.armInflight = ""
		}
		if msg.Err != nil {
			m.invalidateStreamCache(msg.VideoID)
			if m.armedVideoID == msg.VideoID {
				m.armedVideoID = ""
			}
			return m, nil
		}
		// Only keep the arm if this is still the next queue item.
		cur := m.queue.CurrentIndex()
		next, ok := m.queue.At(cur + 1)
		if !ok || next.VideoID != msg.VideoID {
			return m, dropArmedPlaylistEntry(m.player)
		}
		m.armedVideoID = msg.VideoID
		return m, nil
	case playProgressTickMsg:
		if m.currentTrack == nil {
			return m, tickPlayProgress()
		}
		if !m.audioLoaded {
			// Keep restored playPos intact; tick faster while showing load pulse.
			if m.audioLoading() {
				return m, tickAudioLoading()
			}
			return m, tickPlayProgress()
		}
		return m, tea.Batch(fetchPlayProgress(m.player), tickPlayProgress())
	case playProgressMsg:
		if msg.Err == nil && m.audioLoaded {
			if msg.Duration > 0 {
				m.playDuration = msg.Duration
			}
			m.playBuffered = msg.Buffered
			if m.playBuffered < m.playPos {
				m.playBuffered = m.playPos
			}
			var cmds []tea.Cmd
			// Don't fight the mouse while scrubbing the progress bar.
			if !m.scrubbing {
				if m.resumeSeek >= 0.5 {
					if msg.Pos+2.5 >= m.resumeSeek {
						m.playPos = msg.Pos
						m.clearResumeSeek()
						m.markPlayPosDirty()
					} else {
						// Keep restored position in UI/session until seek sticks.
						m.playPos = m.resumeSeek
						if m.resumeSeekTries > 0 {
							m.resumeSeekTries--
							cmds = append(cmds, seekAbsoluteCmd(m.player, m.resumeSeek))
						} else {
							// Give up nudging; follow real position so we don't soft-lock.
							m.playPos = msg.Pos
							m.clearResumeSeek()
							m.markPlayPosDirty()
						}
					}
				} else {
					m.playPos = msg.Pos
					m.markPlayPosDirty()
				}
			}
			// Refetch lyrics once duration becomes known after a duration=0 fetch.
			if msg.Duration >= 0.5 && m.lyricsFetchDur < 0.5 && m.lyricsTrackKey != "" && !m.lyricsLoading {
				if cmd := m.ensureLyricsFetched(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			// Focus switching on Details and NP rail — throttle to once per displayed second.
			pos := m.playPos
			if m.scrubbing {
				pos = m.scrubPos
			}
			wantRail := m.showQueuePanel() && (m.nowPlayingOpen || m.railTab == RailDetails)
			if wantRail {
				sec := int(pos)
				if sec != m.lastRailClockSec {
					m.lastRailClockSec = sec
					m.setQueuePanelContent()
				}
			}
			if m.nowPlayingOpen {
				m.syncLyricsFollowOffset()
			}
			// Warm next stream URL(s) — ramps ahead when near track end.
			if cmd := m.ensureUpcomingPreloaded(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if cmd := m.ensureUpcomingArmed(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if cmd := m.applyCrossfadeVolume(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil
	case StreamURLMsg:
		if msg.Err != nil {
			m.statusMsg = shortStreamErr(msg.Err)
			return m, nil
		}

		m.statusMsg = "Playing audio!"
		return m, loadAndPlay(m.player, msg.URL)
	case SearchResultsMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Search error: %v", msg.Err)
			m.pageLoading = false
			return m, nil
		}
		m.searchResults = msg.Results
		if m.listCursor < 0 || m.listCursor >= len(m.searchResults) {
			m.listCursor = 0
		}
		m.stack.Clear()
		m.artistPage = nil
		m.albumPage = nil
		m.playlistPage = nil
		m.pageLoading = false
		m.pageErr = ""
		m.statusMsg = fmt.Sprintf("Found %d results", len(msg.Results))
		m.markSessionDirty()
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		return m, m.enqueueVisibleImages(m.mainWidth())
	case ArtistMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.pageLoading = false
		if msg.Err != nil || msg.Page == nil {
			m.pageErr = fmtErr(msg.Err)
			if msg.Page == nil && msg.Err == nil {
				m.pageErr = "empty artist response"
			}
			m.statusMsg = "Artist unavailable"
			m.setMainContent()
			return m, nil
		}
		m.artistPage = msg.Page
		m.listCursor = 0
		m.pageErr = ""
		id := msg.RequestID
		if id == "" {
			id = msg.Page.ChannelID
		}
		m.stack.ReplaceOrPush(Screen{Kind: ScreenArtist, ID: id, Title: msg.Page.Name})
		m.statusMsg = msg.Page.Name
		m.markSessionDirty()
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		return m, m.enqueueVisibleImages(m.mainWidth())
	case resolveAlbumMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.pageLoading = false
		if msg.BrowseID != "" {
			if m.currentTrack != nil {
				m.currentTrack.AlbumID = msg.BrowseID
				if msg.Name != "" {
					m.currentTrack.Album = msg.Name
				}
			}
			return m.openAlbum(msg.BrowseID)
		}
		if msg.AudioID != "" {
			if m.currentTrack != nil {
				m.currentTrack.AlbumID = msg.AudioID
				if msg.Name != "" {
					m.currentTrack.Album = msg.Name
				}
			}
			return m.openOlak(msg.AudioID)
		}
		m.statusMsg = "View Album unavailable"
		m.setMainContent()
		return m, nil
	case AlbumMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.pageLoading = false
		if msg.Err != nil || msg.Page == nil {
			m.pageErr = fmtErr(msg.Err)
			if msg.Page == nil && msg.Err == nil {
				m.pageErr = "empty album response"
			}
			m.statusMsg = "Album unavailable"
			m.setMainContent()
			return m, nil
		}
		m.albumPage = msg.Page
		m.pageErr = ""
		n := len(msg.Page.Tracks)
		if m.trackCursor < 0 || m.trackCursor >= n {
			m.trackCursor = 0
		}
		m.stack.ReplaceOrPush(Screen{Kind: ScreenAlbum, ID: msg.BrowseID, Title: msg.Page.Title})
		m.statusMsg = msg.Page.Title
		m = m.syncTrackCursorToPlaying()
		m.markSessionDirty()
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		m.ensureTrackCursorInView(10, 1)
		return m, m.enqueueVisibleImages(m.mainWidth())
	case PlaylistMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.pageLoading = false
		if msg.Err != nil || msg.Page == nil {
			m.pageErr = fmtErr(msg.Err)
			if msg.Page == nil && msg.Err == nil {
				m.pageErr = "empty playlist response"
			}
			m.statusMsg = "Playlist unavailable"
			m.setMainContent()
			return m, nil
		}
		m.playlistPage = msg.Page
		m.pageErr = ""
		n := len(msg.Page.Tracks)
		if m.trackCursor < 0 || m.trackCursor >= n {
			m.trackCursor = 0
		}
		m.stack.ReplaceOrPush(Screen{Kind: ScreenPlaylist, ID: msg.Page.ID, Title: msg.Page.Title})
		m.statusMsg = msg.Page.Title
		m = m.syncTrackCursorToPlaying()
		m.markSessionDirty()
		m.mainViewport.SetContent(m.generateMainContent(m.mainWidth()))
		m.mainViewport.YOffset = 0
		m.ensureTrackCursorInView(10, 1)
		return m, m.enqueueVisibleImages(m.mainWidth())
	case WatchMsg:
		if msg.Gen != 0 && msg.Gen != m.playGen {
			return m, nil
		}
		if msg.SeedVideoID != "" && (m.currentTrack == nil || m.currentTrack.VideoID != msg.SeedVideoID) {
			return m, nil
		}
		if msg.Err != nil || msg.Watch == nil {
			if msg.Err != nil {
				m.statusMsg = "Radio unavailable"
			}
			return m, nil
		}
		// Replace upcoming with watch tracks that come *after* the current
		// video — never wrap to earlier playlist positions.
		m.queue.TruncateAfterCurrent()
		start := 0
		if m.currentTrack != nil {
			for i, tr := range msg.Watch.Tracks {
				if tr.VideoID == m.currentTrack.VideoID {
					start = i + 1
					break
				}
			}
		}
		seen := map[string]struct{}{"": {}}
		if m.currentTrack != nil {
			seen[m.currentTrack.VideoID] = struct{}{}
		}
		for _, tr := range msg.Watch.Tracks[start:] {
			if tr.VideoID == "" {
				continue
			}
			if _, dup := seen[tr.VideoID]; dup {
				continue
			}
			seen[tr.VideoID] = struct{}{}
			m.queue.Add(trackFromAPI(tr))
		}
		m.queue.CapHistory(maxQueueHistory)
		m.markSessionDirty()
		m.applyLayout()
		m.setQueuePanelContent()
		mm, disarm := m.invalidateArmedIfNextChanged()
		return mm, tea.Batch(
			mm.enqueueVisibleImages(mm.mainWidth()),
			mm.ensureUpcomingPreloaded(),
			mm.ensureUpcomingArmed(),
			disarm,
		)
	case HomeMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Home error: %v", msg.Err)
			return m, nil
		}
		m.homeCarousels = msg.Carousels
		m.setMainContent()
		return m, m.enqueueVisibleImages(m.mainWidth())
	case ExploreMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.exploreLoading = false
		if msg.Err != nil {
			m.exploreErr = fmtErr(msg.Err)
			m.statusMsg = "Explore unavailable"
			m.setMainContent()
			return m, nil
		}
		m.exploreData = msg.Data
		m.exploreErr = ""
		m.setMainContent()
		return m, m.enqueueVisibleImages(m.mainWidth())
	case MoodCategoriesMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.moodCatsLoading = false
		if msg.Err != nil {
			m.exploreErr = fmtErr(msg.Err)
			m.statusMsg = "Moods unavailable"
			m.setMainContent()
			return m, nil
		}
		m.moodCategories = msg.Categories
		m.exploreErr = ""
		m.setMainContent()
		return m, m.enqueueVisibleImages(m.mainWidth())
	case MoodPlaylistsMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.exploreLoading = false
		if msg.Err != nil {
			m.exploreErr = fmtErr(msg.Err)
			m.statusMsg = "Mood playlists unavailable"
			m.setMainContent()
			return m, nil
		}
		m.moodPlaylists = msg.Playlists
		m.exploreErr = ""
		m.setMainContent()
		return m, m.enqueueVisibleImages(m.mainWidth())
	case ChartsMsg:
		if msg.Gen != 0 && msg.Gen != m.navGen {
			return m, nil
		}
		m.chartsLoading = false
		if msg.Err != nil {
			m.exploreErr = fmtErr(msg.Err)
			m.statusMsg = "Charts unavailable"
			m.setMainContent()
			return m, nil
		}
		m.chartsData = msg.Data
		m.exploreErr = ""
		m.setMainContent()
		return m, m.enqueueVisibleImages(m.mainWidth())
	case ImageLoadedMsg:
		if msg.Kitty == nil {
			msg.Kitty = &KittyImage{Spacer: sizedPlaceholder(msg.Width, msg.Height)}
		}
		w, h := msg.Width, msg.Height
		if w <= 0 {
			w = artWidth
		}
		if h <= 0 {
			h = artHeight
		}
		m.putImageCache(imageCacheKey(msg.URL, w, h), msg.Kitty)
		// Debounce grid rebuild so a burst of finishes doesn't reshape N times.
		if !m.imageDirty {
			m.imageDirty = true
			return m, debounceImagesRedraw()
		}
		return m, nil

	case authHeadersReadyMsg:
		m.statusMsg = "Authenticating with headers..."
		m.setMainContent()
		return m, func() tea.Msg {
			err := m.ytmapiClient.AuthSetup(context.Background(), msg.headers)
			if err != nil {
				return authErrorMsg{err: err}
			}
			return authSuccessMsg{}
		}

	case authErrorMsg:
		m.authState = 0
		m.statusMsg = fmt.Sprintf("Auth error: %v", msg.err)
		m.setMainContent()
		return m, nil

	case authSuccessMsg:
		m.authState = 0
		m.statusMsg = "Successfully authenticated!"
		m.setMainContent()
		return m, nil

	case clientSecretParsedMsg:
		if msg.err != nil {
			m.oauthState = 0
			m.statusMsg = fmt.Sprintf("Error parsing client secret: %v", msg.err)
			m.setMainContent()
			return m, nil
		}
		m.oauthClientID = msg.clientID
		m.oauthClientSecret = msg.clientSecret
		m.oauthState = 3
		m.oauthInput.Blur()
		m.statusMsg = "Requesting OAuth code..."
		m.setMainContent()
		return m, m.fetchOAuthCodeCmd()

	case oauthCodeMsg:
		if msg.err != nil {
			m.oauthState = 0
			m.statusMsg = fmt.Sprintf("OAuth error: %v", msg.err)
			m.setMainContent()
			return m, nil
		}
		m.oauthCodeResp = msg.resp
		m.statusMsg = "Waiting for browser authorization..."
		m.setMainContent()
		return m, m.pollOAuthTokenCmd(msg.resp.Interval)

	case oauthTokenMsg:
		if msg.err != nil {
			m.oauthState = 0
			m.statusMsg = fmt.Sprintf("OAuth token error: %v", msg.err)
			m.setMainContent()
			return m, nil
		}
		if msg.resp.Status == "pending" {
			return m, m.pollOAuthTokenCmd(m.oauthCodeResp.Interval)
		}
		// Success!
		m.oauthState = 0
		m.statusMsg = "Successfully authenticated!"
		m.oauthCodeResp = nil
		m.setMainContent()
		return m, nil

	case imagesRedrawMsg:
		m.imageDirty = false
		m.setMainContent()
		m.setQueuePanelContent()
		// Kick off any newly-visible thumbs after layout settled.
		return m, m.enqueueVisibleImages(m.mainWidth())
	case suggestionsDebounceMsg:
		if msg.Gen != m.suggestionGen || msg.Query != m.searchInput.Value() {
			return m, nil
		}
		m.cancelSuggestions()
		ctx, cancel := context.WithCancel(context.Background())
		m.sugCancel = cancel
		return m, fetchSuggestions(m.ytmapiClient, msg.Query, msg.Gen, ctx)
	case SearchSuggestionsMsg:
		// Ignore stale responses from older keystrokes.
		if msg.Gen != 0 && msg.Gen != m.suggestionGen {
			return m, nil
		}
		if msg.Query != "" && msg.Query != m.searchInput.Value() {
			return m, nil
		}
		if msg.Err != nil {
			if errors.Is(msg.Err, context.Canceled) {
				return m, nil
			}
			m.statusMsg = "Suggestions unavailable"
			return m, nil
		}
		m.searchSuggestions = buildSuggestionList(msg)
		if m.listCursor >= len(m.searchSuggestions) {
			m.listCursor = 0
		}
		return m, m.enqueueSuggestionImages()
	}

	// Pass other events to viewport (e.g. mouse wheel/clicks)
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if mm, cmd, handled := m.handleProgressScrub(mouseMsg); handled {
			return mm, cmd
		}
		if mm, cmd, handled := m.handleLyricsWheel(mouseMsg); handled {
			return mm, cmd
		}

		if m.searchInput.Focused() && mouseMsg.Button == tea.MouseButtonLeft && mouseMsg.Action == tea.MouseActionPress {
			for i := range m.searchSuggestions {
				if m.zone.Get(fmt.Sprintf("suggestion_%d", i)).InBounds(mouseMsg) {
					m.listCursor = i
					return m.activateSuggestion()
				}
			}
		}

		if mouseMsg.Button == tea.MouseButtonLeft && mouseMsg.Action == tea.MouseActionPress {
			if mm, cmd, handled := m.handleLyricsClick(mouseMsg); handled {
				return mm, cmd
			}
			if m.zone.Get("player_play").InBounds(mouseMsg) {
				return m.togglePlayPause()
			}
			if m.zone.Get("player_next").InBounds(mouseMsg) {
				return m.playNext()
			}
			if m.zone.Get("player_prev").InBounds(mouseMsg) {
				return m.playPrev()
			}
			if m.zone.Get("player_vol_down").InBounds(mouseMsg) {
				return m.adjustVolume(-5)
			}
			if m.zone.Get("player_vol_up").InBounds(mouseMsg) {
				return m.adjustVolume(5)
			}
			if m.zone.Get("player_mute").InBounds(mouseMsg) {
				return m.toggleMute()
			}
			if m.zone.Get("player_nowplaying").InBounds(mouseMsg) {
				return m.openNowPlaying()
			}
			if m.zone.Get("np_album").InBounds(mouseMsg) || m.zone.Get("np_view_album").InBounds(mouseMsg) {
				return m.goToPlayingAlbum()
			}
			if mm, cmd, handled := m.handleRailPanelClick(mouseMsg); handled {
				return mm, cmd
			}

			if m.activeMenu == "Explore" && !m.nowPlayingOpen {
				tabs := []string{"overview", "moods", "charts"}
				for _, tab := range tabs {
					if m.zone.Get("explore_tab_" + tab).InBounds(mouseMsg) {
						if m.exploreSubTab == tab {
							continue
						}
						m.exploreSubTab = tab
						m.activeMoodParams = "" // Reset mood playlists view when changing tabs
						m.setMainContent()
						var cmd tea.Cmd
						if tab == "overview" && m.exploreData == nil {
							m.exploreLoading = true
							cmd = fetchExplore(m.ytmapiClient, m.navGen, m.navCtx)
						} else if tab == "moods" && m.moodCategories == nil {
							m.moodCatsLoading = true
							cmd = fetchMoodCategories(m.ytmapiClient, m.navGen, m.navCtx)
						} else if tab == "charts" && m.chartsData == nil {
							m.chartsLoading = true
							cmd = fetchCharts(m.ytmapiClient, "", m.navGen, m.navCtx)
						}
						if cmd == nil {
							cmd = m.enqueueVisibleImages(m.mainWidth())
						} else {
							cmd = tea.Batch(cmd, m.enqueueVisibleImages(m.mainWidth()))
						}
						return m, cmd
					}
				}
				if m.exploreSubTab == "moods" {
					if m.zone.Get("mood_back").InBounds(mouseMsg) {
						m.activeMoodParams = ""
						m.moodPlaylists = nil
						m.setMainContent()
						return m, m.enqueueVisibleImages(m.mainWidth())
					}
					if m.moodCategories != nil && m.activeMoodParams == "" {
						for _, categories := range m.moodCategories {
							for _, cat := range categories {
								if m.zone.Get("mood_"+cat.Params).InBounds(mouseMsg) {
									m.activeMoodParams = cat.Params
									m.moodPlaylists = nil
									m.exploreLoading = true
									m.setMainContent()
									return m, fetchMoodPlaylists(m.ytmapiClient, cat.Params, m.navGen, m.navCtx)
								}
							}
						}
					}
				}
			}

			if m.activeMenu == "Settings" && !m.nowPlayingOpen {
				// ── Tab switching ───────────────────────────────────────────
				for _, tab := range settingsTabs {
					if m.zone.Get("settings_tab_" + tab.ID).InBounds(mouseMsg) {
						m.settingsTab = tab.ID
						m.setMainContent()
						return m, nil
					}
				}

				// ── Account tab ─────────────────────────────────────────────
				if m.zone.Get("settings_oauth").InBounds(mouseMsg) {
					m.oauthState = 1
					m.oauthInput.Placeholder = "Path to client_secret.json (or raw Client ID)"
					m.oauthInput.Reset()
					m.oauthInput.Focus()
					m.setMainContent()
					return m, textinput.Blink
				}
				if m.zone.Get("settings_auth_headers").InBounds(mouseMsg) {
					m.authState = 1
					m.setMainContent()
					return m, m.openEditorForHeadersCmd()
				}
				if m.zone.Get("settings_toggle_queue").InBounds(mouseMsg) {
					m.queuePanelHidden = !m.queuePanelHidden
					m.markSessionDirty()
					m.setMainContent()
					return m, nil
				}

				// ── Playback tab ────────────────────────────────────────────
				if m.zone.Get("settings_repeat").InBounds(mouseMsg) {
					mm, cmd := m.cycleRepeatMode(); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_shuffle").InBounds(mouseMsg) {
					mm, cmd := m.toggleShuffle(); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_crossfade").InBounds(mouseMsg) {
					mm, cmd := m.toggleCrossfade(); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_crossfade_dec").InBounds(mouseMsg) ||
					m.zone.Get("settings_crossfade_val_dec").InBounds(mouseMsg) {
					return m.stepCrossfadeSec(-1)
				}
				if m.zone.Get("settings_crossfade_inc").InBounds(mouseMsg) ||
					m.zone.Get("settings_crossfade_val_inc").InBounds(mouseMsg) {
					return m.stepCrossfadeSec(1)
				}
				if m.zone.Get("settings_sleep").InBounds(mouseMsg) {
					mm, cmd := m.cycleSleepTimer(); mm.setMainContent(); return mm, cmd
				}

				// ── Audio tab ───────────────────────────────────────────────
				if m.zone.Get("settings_normalize").InBounds(mouseMsg) {
					mm, cmd := m.toggleNormalize(); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_silence").InBounds(mouseMsg) {
					mm, cmd := m.toggleSilenceSkip(); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_tempo_dec").InBounds(mouseMsg) {
					mm, cmd := m.adjustTempo(-0.05); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_tempo_inc").InBounds(mouseMsg) {
					mm, cmd := m.adjustTempo(0.05); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_pitch_dec").InBounds(mouseMsg) {
					mm, cmd := m.adjustPitch(-1); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_pitch_inc").InBounds(mouseMsg) {
					mm, cmd := m.adjustPitch(1); mm.setMainContent(); return mm, cmd
				}
				if m.zone.Get("settings_tempo_reset").InBounds(mouseMsg) {
					mm, c1 := m.resetTempo()
					mmm, c2 := mm.resetPitch()
					mmm.setMainContent()
					return mmm, tea.Batch(c1, c2)
				}
				if m.zone.Get("settings_eq").InBounds(mouseMsg) {
					mm, cmd := m.cycleEQPreset(); mm.setMainContent(); return mm, cmd
				}

				// ── General tab ─────────────────────────────────────────────
				if m.zone.Get("settings_clear_session").InBounds(mouseMsg) {
					m.statusMsg = "Session cleared"
					return m, nil
				}
			}

			// Queue panel track jumps
			if m.showQueuePanel() {
				for i := 0; i < m.queue.Len(); i++ {
					if m.zone.Get(fmt.Sprintf("queue_track_%d", i)).InBounds(mouseMsg) {
						return m.playQueueIndex(i)
					}
				}
			}

			// Sidebar menu items
			for _, item := range m.menuItems {
				if m.zone.Get("menu_" + item).InBounds(mouseMsg) {
					if item == "Home" {
						m = m.goHome()
						m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
						return m, m.enqueueVisibleImages(m.mainWidth())
					}
					if item == "Explore" {
						mm, cmd := m.openExplore()
						mm.leftViewport.SetContent(mm.generateSidebarContent(leftSidebarWidth))
						return mm, tea.Batch(cmd, mm.enqueueVisibleImages(mm.mainWidth()))
					}
					m.activeMenu = item
					m = m.leaveDetailPages()
					m.markSessionDirty()
					m.leftViewport.SetContent(m.generateSidebarContent(leftSidebarWidth))
					m.setMainContent()
					m.mainViewport.YOffset = 0
					return m, nil
				}
			}

			// Entity / filter / play zones
			if mm, zcmd, handled := m.handleZoneClick(mouseMsg); handled {
				return mm, zcmd
			}

			for i, carousel := range m.homeCarousels {
				title := carousel.Title
				if m.zone.Get(title + "_left").InBounds(mouseMsg) {
					m.activeCarousel = i
					m.activePane = PaneMain
					if m.carouselOffsets[title] > 0 {
						m.carouselOffsets[title]--
					}
					m.setMainContent()
					return m, m.enqueueVisibleImages(m.mainWidth())
				}

				if m.zone.Get(title + "_right").InBounds(mouseMsg) {
					m.activeCarousel = i
					m.activePane = PaneMain
					maxLen := len(carousel.Contents)
					if m.carouselOffsets[title] < maxLen-1 {
						m.carouselOffsets[title]++
					}
					m.setMainContent()
					return m, m.enqueueVisibleImages(m.mainWidth())
				}
			}

			if m.activeMenu == "Explore" {
				checkExploreCarousel := func(title string, maxLen int) bool {
					if m.zone.Get(title + "_left").InBounds(mouseMsg) {
						if m.carouselOffsets[title] > 0 { m.carouselOffsets[title]-- }
						m.setMainContent()
						return true
					}
					if m.zone.Get(title + "_right").InBounds(mouseMsg) {
						if m.carouselOffsets[title] < maxLen-1 { m.carouselOffsets[title]++ }
						m.setMainContent()
						return true
					}
					return false
				}
				
				if m.exploreSubTab == "overview" && m.exploreData != nil {
					if len(m.exploreData.NewReleases) > 0 && checkExploreCarousel("New Releases", len(m.exploreData.NewReleases)) {
						return m, m.enqueueVisibleImages(m.mainWidth())
					}
					if m.exploreData.Trending != nil && len(m.exploreData.Trending.Items) > 0 && checkExploreCarousel("Trending", len(m.exploreData.Trending.Items)) {
						return m, m.enqueueVisibleImages(m.mainWidth())
					}
					if len(m.exploreData.NewVideos) > 0 && checkExploreCarousel("New Music Videos", len(m.exploreData.NewVideos)) {
						return m, m.enqueueVisibleImages(m.mainWidth())
					}
				}
			}

			if m.artistPage != nil && m.currentScreen() == screenArtist {
				checkArtistCarousel := func(title string, maxLen int) bool {
					if m.zone.Get(title + "_left").InBounds(mouseMsg) {
						if m.carouselOffsets[title] > 0 {
							m.carouselOffsets[title]--
						}
						m.setMainContent()
						return true
					}
					if m.zone.Get(title + "_right").InBounds(mouseMsg) {
						if m.carouselOffsets[title] < maxLen-1 {
							m.carouselOffsets[title]++
						}
						m.setMainContent()
						return true
					}
					return false
				}
				a := m.artistPage
				if a.Albums != nil && checkArtistCarousel("Albums", len(a.Albums.Results)) {
					return m, m.enqueueVisibleImages(m.mainWidth())
				}
				if a.Singles != nil && checkArtistCarousel("Singles & EPs", len(a.Singles.Results)) {
					return m, m.enqueueVisibleImages(m.mainWidth())
				}
				if a.Related != nil && checkArtistCarousel("Fans Also Like", len(a.Related.Results)) {
					return m, m.enqueueVisibleImages(m.mainWidth())
				}
			}
		}

		left, main, _ := m.layoutWidths()
		switch {
		case mouseMsg.X < left:
			m.leftViewport, cmd = m.leftViewport.Update(msg)
		case mouseMsg.X >= left+main && m.showQueuePanel():
			m.activePane = PaneQueue
			m.rightViewport, cmd = m.rightViewport.Update(msg)
		default:
			m.mainViewport, cmd = m.mainViewport.Update(msg)
		}
	}

	return m, cmd
}

func (m Model) cycleRepeatMode() (Model, tea.Cmd) {
	m.repeatMode = (m.repeatMode + 1) % 3
	m.markSessionDirty()
	labels := []string{"Off", "All", "One"}
	m.statusMsg = "Repeat: " + labels[m.repeatMode]
	m.setQueuePanelContent()
	return m, nil
}

func (m Model) toggleShuffle() (Model, tea.Cmd) {
	m.shuffle = !m.shuffle
	m.markSessionDirty()
	if m.shuffle {
		m.statusMsg = "Shuffle: On"
	} else {
		m.statusMsg = "Shuffle: Off"
	}
	m.setQueuePanelContent()
	return m, nil
}

// playNext advances the queue and starts the next track.
func (m Model) playNext() (Model, tea.Cmd) {
	return m.playNextWithManual(true)
}

func (m Model) playNextWithManual(manual bool) (Model, tea.Cmd) {
	if !manual && m.repeatMode == 2 && m.queue.CurrentIndex() >= 0 { // RepeatOne
		return m.playQueueIndex(m.queue.CurrentIndex())
	}

	t, ok := m.queue.Next(m.shuffle, m.repeatMode == 1) // true if RepeatAll
	if !ok {
		m.statusMsg = "End of queue"
		m.isPlaying = false
		return m, nil
	}
	return m.startQueuedTrack(t)
}

// playPrev moves back in the queue and starts that track.
func (m Model) playPrev() (Model, tea.Cmd) {
	t, ok := m.queue.Prev()
	if !ok {
		m.statusMsg = "Start of queue"
		return m, nil
	}
	return m.startQueuedTrack(t)
}

// playQueueIndex jumps to a queue slot and plays it.
func (m Model) playQueueIndex(i int) (Model, tea.Cmd) {
	if !m.queue.JumpTo(i) {
		return m, nil
	}
	t, ok := m.queue.Current()
	if !ok {
		return m, nil
	}
	return m.startQueuedTrack(t)
}

func (m Model) startQueuedTrack(t Track) (Model, tea.Cmd) {
	m.clearCrossfadeArmState()
	m.currentTrack = &t
	m.isPlaying = true
	m.audioLoaded = false
	m.resumeSeek = 0
	m.resumeSeekTries = 0
	m.playPos = 0
	m.playDuration = 0
	m.playBuffered = 0
	m.playGen++
	gen := m.playGen
	m.cancelPlayExtract()
	ctx, cancel := context.WithCancel(context.Background())
	m.playCancel = cancel
	m.playCtx = ctx
	sideCmd := m.onTrackChanged()
	m.queueCursor = m.queue.CurrentIndex()
	m.statusMsg = "Loading: " + t.Title
	if m.onTracklistScreen() {
		m = m.syncTrackCursorToPlaying()
		m.ensureTrackCursorInView(10, 1)
		m.setMainContent()
	}
	m.applyLayout()
	m.setQueuePanelContent()
	m.markSessionDirty()
	cachedURL, _ := m.peekStreamCache(t.VideoID)
	batch := []tea.Cmd{
		playTrackResolved(m.extractor, t, gen, ctx, cachedURL),
		m.enqueueVisibleImages(m.mainWidth()),
		sideCmd,
	}
	if warm := m.ensureUpcomingPreloaded(); warm != nil {
		batch = append(batch, warm)
	}
	// Stop must finish before extract/load so a concurrent Stop can't kill the new track.
	return m, tea.Sequence(stopPlayback(m.player), tea.Batch(batch...))
}

func (m Model) nextPane() Pane {
	switch m.activePane {
	case PaneSidebar:
		return PaneMain
	case PaneMain:
		if m.showQueuePanel() {
			return PaneQueue
		}
		return PaneSidebar
	case PaneQueue:
		return PaneSidebar
	default:
		return PaneMain
	}
}

func (m Model) prevPane() Pane {
	switch m.activePane {
	case PaneSidebar:
		if m.showQueuePanel() {
			return PaneQueue
		}
		return PaneMain
	case PaneMain:
		return PaneSidebar
	case PaneQueue:
		return PaneMain
	default:
		return PaneMain
	}
}

// enqueueVisibleImages fetches thumbs for the active view (home cards, search,
// playlist/album cover + track thumbs), keyed by size so layout stays stable.
func (m *Model) enqueueVisibleImages(mainWidth int) tea.Cmd {
	const maxSearchThumbs = 8

	var cmds []tea.Cmd
	seen := make(map[string]struct{})

	queue := func(url string, width, height int) {
		if url == "" {
			return
		}
		key := imageCacheKey(url, width, height)
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		if _, ok := m.imageCache[key]; ok {
			return
		}
		ph := KittyImage{Spacer: sizedPlaceholder(width, height)}
		m.putImageCache(key, &ph)
		cmds = append(cmds, fetchImageSized(url, width, height))
	}

	if sc, ok := m.stack.Current(); ok {
		switch sc.Kind {
		case ScreenPlaylist:
			if m.playlistPage != nil {
				queue(firstThumbURL(m.playlistPage.Thumbnails), coverWidth, coverHeight)
			}
		case ScreenAlbum:
			if m.albumPage != nil {
				queue(firstThumbURL(m.albumPage.Thumbnails), coverWidth, coverHeight)
			}
		case ScreenArtist:
			if m.artistPage != nil {
				a := m.artistPage
				if len(a.Thumbnails) > 0 {
					queue(a.Thumbnails[len(a.Thumbnails)-1].URL, artWidth, artHeight)
				}

				getThumb := func(item map[string]any) string {
					if tList, ok := item["thumbnails"].([]any); ok && len(tList) > 0 {
						if t, ok := tList[0].(map[string]any); ok {
							if url, ok := t["url"].(string); ok {
								return url
							}
						}
					}
					return ""
				}

				if a.Songs != nil {
					for i, item := range a.Songs.Results {
						if i >= 10 {
							break
						}
						if url := getThumb(item); url != "" {
							queue(url, sugArtWidth, sugArtHeight)
						}
					}
				}

				contentWidth := mainWidth - 2
				maxVis := contentWidth / 28
				if maxVis < 1 {
					maxVis = 1
				}

				queueCarousel := func(items []map[string]any) {
					for i, item := range items {
						if i >= maxVis {
							break
						}
						if url := getThumb(item); url != "" {
							queue(url, artWidth, artHeight)
						}
					}
				}

				if a.Albums != nil {
					queueCarousel(a.Albums.Results)
				}
				if a.Singles != nil {
					queueCarousel(a.Singles.Results)
				}
				if a.Related != nil {
					queueCarousel(a.Related.Results)
				}
			}
		}
	} else if len(m.searchResults) > 0 {
		n := 0
		for _, res := range m.searchResults {
			if n >= maxSearchThumbs {
				break
			}
			if len(res.Thumbnails) == 0 {
				continue
			}
			if res.Category == "Top result" || n == 0 {
				queue(res.Thumbnails[0].URL, artWidth, artHeight)
				n++
			}
		}
	} else if m.activeMenu == "Explore" {
		contentWidth := mainWidth - 2
		cardWidth := 28
		maxVisible := contentWidth / cardWidth
		if maxVisible < 1 { maxVisible = 1 }
		maxVisible++

		if m.exploreSubTab == "overview" && m.exploreData != nil {
			var carousels [][]ytmapi.HomeCarouselItem
			
			if len(m.exploreData.NewReleases) > 0 {
				items := make([]ytmapi.HomeCarouselItem, len(m.exploreData.NewReleases))
				for i, r := range m.exploreData.NewReleases {
					items[i] = ytmapi.HomeCarouselItem{
						Title: r.Title, BrowseID: r.BrowseID, PlaylistID: r.PlaylistID, Year: r.Year, Thumbnails: r.Thumbnails,
					}
				}
				carousels = append(carousels, items)
			}
			if m.exploreData.Trending != nil && len(m.exploreData.Trending.Items) > 0 {
				carousels = append(carousels, m.exploreData.Trending.Items)
			}
			if len(m.exploreData.NewVideos) > 0 {
				carousels = append(carousels, m.exploreData.NewVideos)
			}

			// We use arbitrary titles for offsets in explore Overview in renderExploreOverview!
			// Actually renderExploreOverview passes "New Releases", "Trending", "New Music Videos" as titles.
			titles := []string{"New Releases", "Trending", "New Music Videos"}
			cIdx := 0
			for _, items := range carousels {
				if cIdx >= len(titles) { break }
				offset := m.carouselOffsets[titles[cIdx]]
				if offset < 0 { offset = 0 }
				end := offset + maxVisible
				if end > len(items) { end = len(items) }
				if offset <= end {
					for _, card := range items[offset:end] {
						if len(card.Thumbnails) > 0 {
							queue(card.Thumbnails[0].URL, artWidth, artHeight)
						}
					}
				}
				cIdx++
			}
		} else if m.exploreSubTab == "moods" && m.activeMoodParams != "" && m.moodPlaylists != nil {
			const thumbW, thumbH = 8, 4
			for i, p := range m.moodPlaylists {
				if i >= 30 {
					break
				}
				if thumbs, ok := p["thumbnails"].([]any); ok && len(thumbs) > 0 {
					if thumbMap, ok := thumbs[0].(map[string]any); ok {
						if url, ok := thumbMap["url"].(string); ok && url != "" {
							queue(url, thumbW, thumbH)
						}
					}
				}
			}
		} else if m.exploreSubTab == "charts" && m.chartsData != nil {
			for i, artist := range m.chartsData.Artists {
				if i >= 15 { break }
				if len(artist.Thumbnails) > 0 {
					queue(artist.Thumbnails[0].URL, 8, 4)
				}
			}
			
			cardW := ((mainWidth - 2) * 6 / 10 - 4) / 2
			for i, v := range m.chartsData.Videos {
				if i >= 6 { break }
				if len(v.Thumbnails) > 0 {
					queue(v.Thumbnails[0].URL, cardW-2, 6)
				}
			}
			for i, v := range m.chartsData.Daily {
				if i >= 10 { break }
				if len(v.Thumbnails) > 0 {
					queue(v.Thumbnails[0].URL, 8, 4)
				}
			}
			for i, v := range m.chartsData.Weekly {
				if i >= 10 { break }
				if len(v.Thumbnails) > 0 {
					queue(v.Thumbnails[0].URL, 8, 4)
				}
			}
		}
	} else {
		contentWidth := mainWidth - 2
		cardWidth := 28
		maxVisible := contentWidth / cardWidth
		if maxVisible < 1 {
			maxVisible = 1
		}
		maxVisible++

		for _, carousel := range m.homeCarousels {
			offset := m.carouselOffsets[carousel.Title]
			if offset < 0 {
				offset = 0
			}
			end := offset + maxVisible
			if end > len(carousel.Contents) {
				end = len(carousel.Contents)
			}
			if offset > end {
				continue
			}
			for _, card := range carousel.Contents[offset:end] {
				if len(card.Thumbnails) > 0 {
					queue(card.Thumbnails[0].URL, artWidth, artHeight)
				}
			}
		}
	}

	// Browse rail needs the large cover; NP mode rail does not (art is on stage).
	if m.showQueuePanel() && !m.nowPlayingOpen && m.currentTrack != nil && m.currentTrack.ThumbnailURL != "" {
		aw, ah := m.queueArtDims()
		queue(m.currentTrack.ThumbnailURL, aw, ah)
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

const (
	imageCacheMax   = 128
	maxQueueHistory = 50
)

func (m *Model) cancelPlayExtract() {
	if m.playCancel != nil {
		m.playCancel()
		m.playCancel = nil
	}
	m.playCtx = nil
}

func (m *Model) cancelSuggestions() {
	if m.sugCancel != nil {
		m.sugCancel()
		m.sugCancel = nil
	}
}

func (m *Model) cancelLyrics() {
	if m.lyricsCancel != nil {
		m.lyricsCancel()
		m.lyricsCancel = nil
	}
}

func (m *Model) cancelSongDetails() {
	if m.songDetailsCancel != nil {
		m.songDetailsCancel()
		m.songDetailsCancel = nil
	}
}

// resetTrackSideState clears lyrics/song metadata when switching tracks.
func (m *Model) resetTrackSideState() {
	m.cancelLyrics()
	m.cancelSongDetails()
	m.lyricsTrackKey = ""
	m.lyricsLines = nil
	m.lyricsPlain = ""
	m.lyricsErr = ""
	m.lyricsInstrumental = false
	m.lyricsLoading = false
	m.lyricsFetchDur = 0
	m.lyricsFollow = true
	m.lyricsCursor = -1
	m.lyricsIdleAt = time.Time{}
	m.songDetails = nil
	m.songDetailsVideoID = ""
	m.songDetailsLoading = false
	m.songDetailsErr = ""
}

// onTrackChanged clears side state and fetches lyrics/meta when the stage/details need them.
func (m *Model) onTrackChanged() tea.Cmd {
	m.resetTrackSideState()
	var cmds []tea.Cmd
	if m.nowPlayingOpen {
		cmds = append(cmds, m.enqueueNowPlayingImage())
	}
	if m.nowPlayingOpen || (m.showQueuePanel() && m.railTab == RailDetails) {
		if cmd := m.ensureLyricsFetched(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.showQueuePanel() && m.railTab == RailDetails {
		if cmd := m.ensureSongDetailsFetched(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) putImageCache(key string, img *KittyImage) {
	if m.imageCache == nil {
		m.imageCache = make(map[string]*KittyImage)
	}
	if _, exists := m.imageCache[key]; exists {
		// Refresh LRU position.
		for i, k := range m.imageCacheOrder {
			if k == key {
				m.imageCacheOrder = append(m.imageCacheOrder[:i], m.imageCacheOrder[i+1:]...)
				break
			}
		}
		m.imageCacheOrder = append(m.imageCacheOrder, key)
		m.imageCache[key] = img
		return
	}
	for len(m.imageCache) >= imageCacheMax && len(m.imageCacheOrder) > 0 {
		oldest := m.imageCacheOrder[0]
		m.imageCacheOrder = m.imageCacheOrder[1:]
		delete(m.imageCache, oldest)
	}
	m.imageCache[key] = img
	m.imageCacheOrder = append(m.imageCacheOrder, key)
}

type authHeadersReadyMsg struct {
	headers string
}

type authSuccessMsg struct{}
type authErrorMsg struct {
	err error
}

func (m *Model) openEditorForHeadersCmd() tea.Cmd {
	return func() tea.Msg {
		filePath := os.ExpandEnv("$HOME/.local/state/go-ytm/headers.txt")
		os.MkdirAll(filepath.Dir(filePath), 0700)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			os.WriteFile(filePath, []byte("# Paste your request headers from music.youtube.com here\n# (e.g. from Firefox Network tab -> right click a request -> Copy -> Copy Request Headers)\n# Save and exit to continue.\n\n"), 0600)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		cmd := exec.Command(editor, filePath)
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return authErrorMsg{err: err}
			}
			data, err := os.ReadFile(filePath)
			if err != nil {
				return authErrorMsg{err: err}
			}
			os.Remove(filePath)
			return authHeadersReadyMsg{headers: string(data)}
		})()
	}
}

func explicitBadge() string {
	return " 🅴"
}

type clientSecretParsedMsg struct {
	clientID     string
	clientSecret string
	err          error
}

func (m *Model) parseClientSecretCmd(path string) tea.Cmd {
	return func() tea.Msg {
		path = strings.ReplaceAll(path, "~", os.Getenv("HOME"))
		data, err := os.ReadFile(os.ExpandEnv(path))
		if err != nil {
			return clientSecretParsedMsg{err: err}
		}
		var parsed struct {
			Installed struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
			} `json:"installed"`
			Web struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
			} `json:"web"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return clientSecretParsedMsg{err: err}
		}
		
		clientID := parsed.Installed.ClientID
		clientSecret := parsed.Installed.ClientSecret
		if clientID == "" {
			clientID = parsed.Web.ClientID
			clientSecret = parsed.Web.ClientSecret
		}
		if clientID == "" {
			return clientSecretParsedMsg{err: fmt.Errorf("client_id not found in JSON")}
		}
		return clientSecretParsedMsg{clientID: clientID, clientSecret: clientSecret}
	}
}

type oauthCodeMsg struct {
	resp *ytmapi.OAuthCodeResponse
	err  error
}

func (m *Model) fetchOAuthCodeCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.ytmapiClient.OAuthCode(context.Background(), m.oauthClientID, m.oauthClientSecret)
		return oauthCodeMsg{resp: resp, err: err}
	}
}

type oauthTokenMsg struct {
	resp *ytmapi.OAuthTokenResponse
	err  error
}

func (m *Model) pollOAuthTokenCmd(interval int) tea.Cmd {
	return tea.Tick(time.Duration(interval)*time.Second, func(time.Time) tea.Msg {
		resp, err := m.ytmapiClient.OAuthToken(context.Background(), m.oauthClientID, m.oauthClientSecret, m.oauthCodeResp.DeviceCode)
		return oauthTokenMsg{resp: resp, err: err}
	})
}
