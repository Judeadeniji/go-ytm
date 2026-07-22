package apirunner

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
)

// Paths holds resolved locations for the API and runtime state.
type Paths struct {
	StateDir  string // ~/.local/state/go-ytm
	APIHome   string // directory containing main.py + requirements.txt
	VenvDir   string
	SockPath  string
	TokenPath string
	LogPath   string
	AuthFile  string
}

// ResolvePaths picks API/state locations from env and XDG conventions.
// API home order: YTM_API_HOME → (YTM_DEV=1) ./ytm-api → XDG_DATA_HOME/go-ytm/ytm-api → ~/.local/share/go-ytm/ytm-api.
// In dev mode (YTM_DEV=1) state files (socket, log, token) are written to
// ./tmp/ so they never conflict with an installed production instance.
func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}

	dev := os.Getenv("YTM_DEV") == "1"

	var stateDir string
	if dev {
		// Use a local tmp/ dir so dev and prod never share a socket or log.
		cwd, _ := os.Getwd()
		stateDir = filepath.Join(cwd, "tmp")
	} else {
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			state = filepath.Join(home, ".local", "state")
		}
		stateDir = filepath.Join(state, "go-ytm")
	}

	apiHome := os.Getenv("YTM_API_HOME")
	if apiHome == "" && dev {
		if cwd, err := os.Getwd(); err == nil {
			cand := filepath.Join(cwd, "ytm-api")
			if isAPIHome(cand) {
				apiHome = cand
			}
		}
	}
	if apiHome == "" {
		data := os.Getenv("XDG_DATA_HOME")
		if data == "" {
			data = filepath.Join(home, ".local", "share")
		}
		apiHome = filepath.Join(data, "go-ytm", "ytm-api")
	}

	// Auth credentials always live in the real state dir so dev runs don't
	// require re-authentication when the installed app is already signed in.
	prodStateDir := stateDir
	if dev {
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			state = filepath.Join(home, ".local", "state")
		}
		prodStateDir = filepath.Join(state, "go-ytm")
	}

	p := Paths{
		StateDir:  stateDir,
		APIHome:   apiHome,
		VenvDir:   filepath.Join(apiHome, "venv"),
		SockPath:  filepath.Join(stateDir, "ytm-api.sock"),
		TokenPath: filepath.Join(stateDir, "api.token"),
		LogPath:   filepath.Join(stateDir, "ytm-api.log"),
		AuthFile:  filepath.Join(prodStateDir, "headers_auth.json"),
	}
	if s := os.Getenv("YTM_API_SOCK"); s != "" {
		p.SockPath = s
	}
	return p, nil
}

func isAPIHome(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "main.py"))
	return err == nil
}

// EnsureStateDir creates the state directory with private perms.
func (p Paths) EnsureStateDir() error {
	return os.MkdirAll(p.StateDir, 0o700)
}

// LoadOrCreateToken returns a stable API token, creating one if needed.
func (p Paths) LoadOrCreateToken() (string, error) {
	if t := os.Getenv("YTM_API_TOKEN"); t != "" {
		return t, nil
	}
	if err := p.EnsureStateDir(); err != nil {
		return "", err
	}
	b, err := os.ReadFile(p.TokenPath)
	if err == nil {
		tok := strings.TrimSpace(string(b))
		if tok != "" {
			return tok, nil
		}
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tok := base64.RawURLEncoding.EncodeToString(raw)
	if err := os.WriteFile(p.TokenPath, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

// ApplyEnv sets process env so ytmapi.NewClient and the child share sock/token.
func (p Paths) ApplyEnv(token string) {
	_ = os.Setenv("YTM_API_SOCK", p.SockPath)
	_ = os.Setenv("YTM_API_TOKEN", token)
}
