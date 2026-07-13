package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Player manages the mpv IPC client (connect, load, seek, filters, sleep timer)
type Player struct {
	socketPath string
	cmd        *exec.Cmd
	conn       net.Conn
	mu         sync.Mutex
}

// IPCCommand represents a JSON IPC command for mpv
type IPCCommand struct {
	Command []interface{} `json:"command"`
}

var execCommand = exec.Command

// NewPlayer starts an mpv instance with an IPC server and connects to it.
func NewPlayer() (*Player, error) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("go-ytm-mpv-%d.sock", time.Now().UnixNano()))

	// Ensure any old socket is removed
	os.Remove(socketPath)

	// Start mpv
	cmd := execCommand("mpv", "--idle", "--input-ipc-server="+socketPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	p := &Player{
		socketPath: socketPath,
		cmd:        cmd,
	}

	// Wait for the socket to be created and connect
	err := p.connect(5 * time.Second)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("failed to connect to mpv IPC: %w", err)
	}

	// Start reading events so the socket buffer doesn't fill up
	p.startEventLoop()

	return p, nil
}

// connect attempts to dial the Unix socket with a timeout
func (p *Player) connect(timeout time.Duration) error {
	start := time.Now()
	for {
		conn, err := net.Dial("unix", p.socketPath)
		if err == nil {
			p.conn = conn
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for socket %s", p.socketPath)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// startEventLoop reads from the connection to prevent buffer overflow.
// In the future, this can be expanded to parse events and dispatch them.
func (p *Player) startEventLoop() {
	go func() {
		scanner := bufio.NewScanner(p.conn)
		for scanner.Scan() {
			// discard event string for now
			_ = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			if !isClosedConnectionError(err) {
				fmt.Fprintf(os.Stderr, "mpv ipc scanner error: %v\n", err)
			}
		}
	}()
}

// isClosedConnectionError checks if an error is due to the connection being closed
func isClosedConnectionError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "use of closed network connection" || err == net.ErrClosed || strings.Contains(err.Error(), "use of closed network connection")
}

// Close stops the mpv instance and closes the IPC connection.
func (p *Player) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		// Try to politely ask mpv to quit
		p.sendCommandUnlocked("quit")
		p.conn.Close()
	}

	if p.cmd != nil && p.cmd.Process != nil {
		// Wait briefly for graceful shutdown, then force kill
		done := make(chan error, 1)
		go func() { done <- p.cmd.Wait() }()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			p.cmd.Process.Kill()
		}
	}

	os.Remove(p.socketPath)
	return nil
}

func (p *Player) sendCommandUnlocked(args ...interface{}) error {
	if p.conn == nil {
		return fmt.Errorf("not connected to mpv")
	}
	cmd := IPCCommand{Command: args}
	b, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = p.conn.Write(b)
	return err
}

// sendCommand sends a JSON IPC command to mpv
func (p *Player) sendCommand(args ...interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sendCommandUnlocked(args...)
}

// Load loads a URL or file path into mpv
func (p *Player) Load(url string) error {
	return p.sendCommand("loadfile", url)
}

// Pause pauses playback
func (p *Player) Pause() error {
	return p.sendCommand("set_property", "pause", true)
}

// Play resumes playback
func (p *Player) Play() error {
	return p.sendCommand("set_property", "pause", false)
}

// Stop stops current playback
func (p *Player) Stop() error {
	return p.sendCommand("stop")
}
