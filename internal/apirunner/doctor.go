package apirunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Report is a diagnostic snapshot for `ytm doctor`.
type Report struct {
	Version   string
	Paths     Paths
	MPV       bool
	Python    bool
	YtDLP     bool
	APIHomeOK bool
	VenvOK    bool
	HealthOK  bool
	AuthOK    bool
	TokenSet  bool
}

// Doctor gathers status without starting the API (unless already running).
func Doctor(version string) Report {
	paths, err := ResolvePaths()
	if err != nil {
		return Report{Version: version}
	}
	_ = paths.EnsureStateDir()
	r := Report{
		Version:   version,
		Paths:     paths,
		MPV:       CheckMPV() == nil,
		Python:    CheckPython() == nil,
		YtDLP:     HasYtDLP(),
		APIHomeOK: isAPIHome(paths.APIHome),
		VenvOK:    fileExists(filepath.Join(paths.VenvDir, "bin", "uvicorn")),
		HealthOK:  paths.HealthOK(),
		AuthOK:    fileExists(paths.AuthFile),
		TokenSet:  os.Getenv("YTM_API_TOKEN") != "" || fileExists(paths.TokenPath),
	}
	return r
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Format renders a doctor report for the terminal.
func (r Report) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "ytm doctor\n")
	fmt.Fprintf(&b, "  version:    %s\n", orDash(r.Version))
	fmt.Fprintf(&b, "  state dir:  %s\n", r.Paths.StateDir)
	fmt.Fprintf(&b, "  api home:   %s %s\n", r.Paths.APIHome, okMark(r.APIHomeOK))
	fmt.Fprintf(&b, "  socket:     %s %s\n", r.Paths.SockPath, okMark(r.HealthOK))
	fmt.Fprintf(&b, "  venv:       %s %s\n", r.Paths.VenvDir, okMark(r.VenvOK))
	fmt.Fprintf(&b, "  auth file:  %s %s\n", r.Paths.AuthFile, okMark(r.AuthOK))
	fmt.Fprintf(&b, "  mpv:        %s\n", okMark(r.MPV))
	fmt.Fprintf(&b, "  python3:    %s\n", okMark(r.Python))
	fmt.Fprintf(&b, "  yt-dlp:     %s\n", okMark(r.YtDLP))
	fmt.Fprintf(&b, "  api token:  %s\n", okMark(r.TokenSet))
	if !r.MPV || !r.Python {
		b.WriteString("\n")
		b.WriteString(DepHints())
	}
	if !r.APIHomeOK {
		b.WriteString("\nMissing ytm-api. Re-run the installer or set YTM_API_HOME.\n")
	}
	if !r.AuthOK {
		b.WriteString("\nNot authenticated yet — open Settings in the TUI after launch.\n")
	}
	return b.String()
}

func okMark(ok bool) string {
	if ok {
		return "[ok]"
	}
	return "[missing]"
}

func orDash(s string) string {
	if s == "" {
		return "dev"
	}
	return s
}
