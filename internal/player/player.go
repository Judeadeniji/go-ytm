package player

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Player manages the mpv IPC client (connect, load, seek, filters, sleep timer)
type Player struct {
	socketPath string
	cmd        *exec.Cmd
	conn       net.Conn
	mu         sync.Mutex
	stderr     bytes.Buffer

	waitOnce sync.Once
	waitErr  error

	pendingMu sync.Mutex
	pending   map[int]chan ipcResponse
	nextReqID int

	events chan EndFileEvent
}

// EndFileEvent is emitted when mpv finishes a file (natural EOF, stop, error, …).
type EndFileEvent struct {
	Reason string
}

// IPCCommand represents a JSON IPC command for mpv
type IPCCommand struct {
	Command   []interface{} `json:"command"`
	RequestID int           `json:"request_id,omitempty"`
}

type ipcResponse struct {
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	RequestID int             `json:"request_id"`
	Event     string          `json:"event"`
	Reason    string          `json:"reason"`
}

var execCommand = exec.Command

const ipcConnectTimeout = 10 * time.Second

// NewPlayer starts an mpv instance with an IPC server and connects to it.
func NewPlayer() (*Player, error) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("go-ytm-mpv-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(socketPath)

	cmd := execCommand("mpv",
		"--idle=yes",
		"--no-video",
		"--force-window=no",
		"--audio-display=no",
		// Avoid contesting the TTY with the Bubble Tea UI (common hang/flake cause).
		"--no-terminal",
		"--really-quiet",
		// User scripts can delay or block IPC socket creation intermittently.
		"--load-scripts=no",
		"--input-ipc-server="+socketPath,
	)

	p := &Player{
		socketPath: socketPath,
		cmd:        cmd,
		pending:    make(map[int]chan ipcResponse),
		events:     make(chan EndFileEvent, 16),
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = &p.stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	if err := p.connect(ipcConnectTimeout); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("failed to connect to mpv IPC: %w", err)
	}

	p.startEventLoop()
	return p, nil
}

func (p *Player) connect(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := p.processAlive(); err != nil {
			return err
		}
		conn, err := net.Dial("unix", p.socketPath)
		if err == nil {
			p.conn = conn
			return nil
		}
		if time.Now().After(deadline) {
			_ = p.processAlive() // surface exit if it died at the last moment
			hint := strings.TrimSpace(p.stderr.String())
			if hint != "" {
				if len(hint) > 240 {
					hint = hint[:240] + "…"
				}
				return fmt.Errorf("timeout waiting for socket %s (mpv: %s)", p.socketPath, hint)
			}
			return fmt.Errorf("timeout waiting for socket %s", p.socketPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// processAlive returns an error if the mpv child has already exited.
func (p *Player) processAlive() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return fmt.Errorf("mpv process missing")
	}
	// Signal 0 checks liveness without affecting the process (Unix).
	if err := p.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		waitErr := p.reap()
		hint := strings.TrimSpace(p.stderr.String())
		if hint != "" {
			if len(hint) > 240 {
				hint = hint[:240] + "…"
			}
			return fmt.Errorf("mpv exited before IPC ready: %v (%s)", waitErr, hint)
		}
		if waitErr != nil {
			return fmt.Errorf("mpv exited before IPC ready: %w", waitErr)
		}
		return fmt.Errorf("mpv exited before IPC ready")
	}
	return nil
}

func (p *Player) reap() error {
	p.waitOnce.Do(func() {
		if p.cmd != nil {
			p.waitErr = p.cmd.Wait()
		}
	})
	return p.waitErr
}

func (p *Player) startEventLoop() {
	conn := p.conn
	go func() {
		defer close(p.events)
		if conn == nil {
			return
		}
		scanner := bufio.NewScanner(conn)
		// mpv responses can be large; bump token size.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var resp ipcResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}

			if resp.Event != "" {
				if resp.Event == "end-file" {
					select {
					case p.events <- EndFileEvent{Reason: resp.Reason}:
					default:
						// Drop if the UI isn't draining — don't stall the IPC loop.
					}
				}
				if resp.RequestID == 0 {
					continue
				}
			}

			p.pendingMu.Lock()
			ch, ok := p.pending[resp.RequestID]
			if ok {
				delete(p.pending, resp.RequestID)
			}
			p.pendingMu.Unlock()
			if ok {
				select {
				case ch <- resp:
				default:
				}
			}
		}
		if err := scanner.Err(); err != nil {
			if !isClosedConnectionError(err) {
				fmt.Fprintf(os.Stderr, "mpv ipc scanner error: %v\n", err)
			}
		}
	}()
}

// Events returns the channel of end-file notifications from mpv.
func (p *Player) Events() <-chan EndFileEvent {
	return p.events
}

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
		_ = p.sendCommandUnlocked("quit")
		_ = p.conn.Close()
		p.conn = nil
	}

	if p.cmd != nil && p.cmd.Process != nil {
		done := make(chan error, 1)
		go func() { done <- p.reap() }()
		select {
		case <-done:
		case <-time.After(800 * time.Millisecond):
			_ = p.cmd.Process.Kill()
			<-done
		}
	}

	_ = os.Remove(p.socketPath)
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

func (p *Player) sendCommand(args ...interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sendCommandUnlocked(args...)
}

func (p *Player) nextRequestID() int {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()
	p.nextReqID++
	if p.nextReqID == 0 {
		p.nextReqID = 1
	}
	return p.nextReqID
}

// getProperty sends get_property and waits for the matching response.
func (p *Player) getProperty(name string) (json.RawMessage, error) {
	id := p.nextRequestID()
	ch := make(chan ipcResponse, 1)

	p.pendingMu.Lock()
	p.pending[id] = ch
	p.pendingMu.Unlock()

	defer func() {
		p.pendingMu.Lock()
		delete(p.pending, id)
		p.pendingMu.Unlock()
	}()

	p.mu.Lock()
	if p.conn == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("not connected to mpv")
	}
	cmd := IPCCommand{Command: []interface{}{"get_property", name}, RequestID: id}
	b, err := json.Marshal(cmd)
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}
	b = append(b, '\n')
	_, err = p.conn.Write(b)
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("mpv: %s", resp.Error)
		}
		return resp.Data, nil
	case <-time.After(2 * time.Second):
		return nil, fmt.Errorf("mpv get_property %s timed out", name)
	}
}

// PositionSeconds returns the current playback position in seconds.
func (p *Player) PositionSeconds() (float64, error) {
	data, err := p.getProperty("time-pos")
	if err != nil {
		return 0, err
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, err
	}
	return v, nil
}

// DurationSeconds returns the current media duration in seconds.
func (p *Player) DurationSeconds() (float64, error) {
	data, err := p.getProperty("duration")
	if err != nil {
		return 0, err
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, err
	}
	return v, nil
}

// Load replaces the current media with url (stops current playback first).
func (p *Player) Load(url string) error {
	return p.sendCommand("loadfile", url, "replace")
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

// TogglePause cycles the pause state of mpv.
func (p *Player) TogglePause() error {
	return p.sendCommand("cycle", "pause")
}

// SeekRelative seeks relative to the current position by the given number of seconds.
func (p *Player) SeekRelative(seconds float64) error {
	return p.sendCommand("seek", seconds, "relative")
}

// SeekAbsolute seeks to an absolute position in seconds.
func (p *Player) SeekAbsolute(seconds float64) error {
	return p.sendCommand("seek", seconds, "absolute")
}
