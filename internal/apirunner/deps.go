package apirunner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CheckMPV returns an error if mpv is not on PATH.
func CheckMPV() error {
	if _, err := exec.LookPath("mpv"); err != nil {
		return fmt.Errorf("mpv not found on PATH\nInstall: pacman -S mpv  |  apt install mpv  |  dnf install mpv")
	}
	return nil
}

// CheckPython returns an error if python3 is not available.
func CheckPython() error {
	_, err := findPython()
	return err
}

func findPython() (string, error) {
	for _, c := range []string{"/usr/bin/python3", "/usr/bin/python3.14", "/usr/bin/python3.13", "/usr/bin/python3.12", "/usr/bin/python3.11", "/usr/bin/python3.10"} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("python3 not found\nInstall: pacman -S python  |  apt install python3  |  dnf install python3")
}

// HasYtDLP reports whether yt-dlp is available.
func HasYtDLP() bool {
	_, err := exec.LookPath("yt-dlp")
	return err == nil
}

// DepHints returns human-readable install tips for the current package manager guess.
func DepHints() string {
	var b strings.Builder
	b.WriteString("Required: mpv, python3\n")
	b.WriteString("Recommended: yt-dlp (fallback stream extraction)\n")
	b.WriteString("  pacman -S mpv python yt-dlp\n")
	b.WriteString("  sudo apt install mpv python3 yt-dlp\n")
	b.WriteString("  sudo dnf install mpv python3 yt-dlp\n")
	return b.String()
}
