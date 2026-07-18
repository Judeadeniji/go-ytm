package apirunner

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	healthTimeout = 45 * time.Second
	healthPoll    = 200 * time.Millisecond
)

// Runner owns an optional child uvicorn process.
type Runner struct {
	Paths     Paths
	Token     string
	cmd       *exec.Cmd
	logFile   *os.File
	startedBy bool
	waitDone  chan error
}

// Start ensures ytm-api is reachable. Reuses an existing healthy socket when possible.
func Start() (*Runner, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, err
	}
	if err := paths.EnsureStateDir(); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	}
	token, err := paths.LoadOrCreateToken()
	if err != nil {
		return nil, fmt.Errorf("api token: %w", err)
	}
	paths.ApplyEnv(token)

	r := &Runner{Paths: paths, Token: token}

	if paths.HealthOK() {
		return r, nil
	}

	if err := CheckPython(); err != nil {
		return nil, err
	}
	if !isAPIHome(paths.APIHome) {
		return nil, fmt.Errorf("ytm-api not found at %s\nInstall with: curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash\nOr set YTM_API_HOME to your ytm-api directory", paths.APIHome)
	}
	if err := ensureVenv(paths); err != nil {
		return nil, err
	}
	if err := r.spawn(); err != nil {
		return nil, err
	}
	if err := r.waitHealthy(); err != nil {
		_ = r.Stop()
		return nil, err
	}
	return r, nil
}

func (r *Runner) spawn() error {
	uvicorn := filepath.Join(r.Paths.VenvDir, "bin", "uvicorn")
	if _, err := os.Stat(uvicorn); err != nil {
		return fmt.Errorf("uvicorn missing in venv (%s): %w", uvicorn, err)
	}

	_ = os.Remove(r.Paths.SockPath)

	logF, err := os.OpenFile(r.Paths.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open api log: %w", err)
	}
	r.logFile = logF

	cmd := exec.Command(uvicorn, "main:app", "--uds", r.Paths.SockPath)
	cmd.Dir = r.Paths.APIHome
	cmd.Env = append(os.Environ(),
		"YTM_API_TOKEN="+r.Token,
		"VIRTUAL_ENV="+r.Paths.VenvDir,
		"PATH="+filepath.Join(r.Paths.VenvDir, "bin")+":"+os.Getenv("PATH"),
	)
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		r.logFile = nil
		return fmt.Errorf("start uvicorn: %w", err)
	}
	r.cmd = cmd
	r.startedBy = true
	r.waitDone = make(chan error, 1)
	go func() { r.waitDone <- cmd.Wait() }()
	return nil
}

func (r *Runner) waitHealthy() error {
	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-r.waitDone:
			if err != nil {
				return fmt.Errorf("ytm-api exited: %v (see %s)", err, r.Paths.LogPath)
			}
			return fmt.Errorf("ytm-api exited early; see %s", r.Paths.LogPath)
		default:
		}
		if r.Paths.HealthOK() {
			return nil
		}
		time.Sleep(healthPoll)
	}
	return fmt.Errorf("timed out waiting for ytm-api; see %s", r.Paths.LogPath)
}

// Stop terminates the child we started (if any).
func (r *Runner) Stop() error {
	if r == nil {
		return nil
	}
	var err error
	if r.startedBy && r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case err = <-r.waitDone:
		case <-time.After(3 * time.Second):
			_ = r.cmd.Process.Kill()
			select {
			case err = <-r.waitDone:
			case <-time.After(2 * time.Second):
			}
		}
		r.cmd = nil
		r.startedBy = false
	}
	if r.logFile != nil {
		_ = r.logFile.Close()
		r.logFile = nil
	}
	return err
}

// HealthOK probes GET /health over the Unix socket.
func (p Paths) HealthOK() bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", p.SockPath)
			},
		},
	}
	resp, err := client.Get("http://localhost/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func ensureVenv(p Paths) error {
	uvicorn := filepath.Join(p.VenvDir, "bin", "uvicorn")
	if _, err := os.Stat(uvicorn); err == nil {
		return nil
	}

	py, err := findPython()
	if err != nil {
		return err
	}
	_ = os.RemoveAll(p.VenvDir)

	cmd := exec.Command(py, "-m", "venv", p.VenvDir)
	cmd.Dir = p.APIHome
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create venv: %w\n%s", err, out)
	}

	pip := filepath.Join(p.VenvDir, "bin", "pip")
	req := filepath.Join(p.APIHome, "requirements.txt")
	cmd = exec.Command(pip, "install", "-q", "-r", req)
	cmd.Dir = p.APIHome
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pip install: %w\n%s", err, out)
	}
	if _, err := os.Stat(uvicorn); err != nil {
		return fmt.Errorf("venv created but uvicorn missing; pip output:\n%s", out)
	}
	return nil
}
