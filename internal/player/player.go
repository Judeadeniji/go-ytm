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

// NewPlayer starts an mpv instance with an IPC server and connects to it.
func NewPlayer() (*Player, error) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("go-ytm-mpv-%d.sock", time.Now().UnixNano()))
	os.Remove(socketPath)

	cmd := execCommand("mpv",
		"--idle",
		"--no-video",
		"--force-window=no",
		"--audio-display=no",
		"--input-ipc-server="+socketPath,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	p := &Player{
		socketPath: socketPath,
		cmd:        cmd,
		pending:    make(map[int]chan ipcResponse),
		events:     make(chan EndFileEvent, 16),
	}

	if err := p.connect(5 * time.Second); err != nil {
		p.Close()
		return nil, fmt.Errorf("failed to connect to mpv IPC: %w", err)
	}

	p.startEventLoop()
	return p, nil
}

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

func (p *Player) startEventLoop() {
	go func() {
		defer close(p.events)
		scanner := bufio.NewScanner(p.conn)
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
		p.conn.Close()
	}

	if p.cmd != nil && p.cmd.Process != nil {
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
