package player

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
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
		// YouTube HTTP streams often report unseekable until demux; allow resume seeks.
		"--force-seekable=yes",
		"--cache=yes",
		// Prefetch/append next track for gapless handoff when crossfade is armed.
		"--gapless-audio=yes",
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
				slog.Error("mpv ipc scanner error", "err", err)
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
			select {
			case <-done:
			case <-time.After(time.Second):
			}
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

// BufferedSeconds returns how far into the track media is cached (seconds),
// suitable for a "bytes loaded" progress overlay. Prefers demuxer-cache-time;
// falls back to time-pos + demuxer-cache-duration.
func (p *Player) BufferedSeconds() (float64, error) {
	if data, err := p.getProperty("demuxer-cache-time"); err == nil {
		var v float64
		if err := json.Unmarshal(data, &v); err == nil && v >= 0 {
			return v, nil
		}
	}
	pos, posErr := p.PositionSeconds()
	if posErr != nil {
		pos = 0
	}
	data, err := p.getProperty("demuxer-cache-duration")
	if err != nil {
		return pos, err
	}
	var ahead float64
	if err := json.Unmarshal(data, &ahead); err != nil {
		return pos, err
	}
	if ahead < 0 {
		ahead = 0
	}
	return pos + ahead, nil
}

// Load replaces the current media with url (stops current playback first).
func (p *Player) Load(url string) error {
	return p.sendCommand("loadfile", url, "replace")
}

// Append adds url to the end of the mpv playlist without interrupting playback.
func (p *Player) Append(url string) error {
	return p.sendCommand("loadfile", url, "append")
}

// PlaylistClear removes all playlist entries.
func (p *Player) PlaylistClear() error {
	return p.sendCommand("playlist-clear")
}

// PlaylistNext forces advancement to the next playlist entry.
func (p *Player) PlaylistNext() error {
	return p.sendCommand("playlist-next", "force")
}

// PlaylistRemove drops the playlist entry at index (0-based).
func (p *Player) PlaylistRemove(index int) error {
	return p.sendCommand("playlist-remove", index)
}

// PlaylistCount returns how many entries are in the mpv playlist.
func (p *Player) PlaylistCount() (int, error) {
	data, err := p.getProperty("playlist-count")
	if err != nil {
		return 0, err
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// PlaylistPos returns the current playlist index (-1 if none).
func (p *Player) PlaylistPos() (int, error) {
	data, err := p.getProperty("playlist-pos")
	if err != nil {
		return -1, err
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return -1, err
	}
	return n, nil
}

// LoadAt loads url and asks mpv to start at startSec (seconds).
// Options are passed as a JSON object (mpv JSON IPC), which is more reliable
// than a "start=N" string for HTTP streams.
func (p *Player) LoadAt(url string, startSec float64) error {
	if startSec < 0.5 {
		return p.Load(url)
	}
	opts := map[string]any{"start": startSec}
	return p.sendCommand("loadfile", url, "replace", opts)
}

// Seekable reports whether mpv currently considers the file seekable.
func (p *Player) Seekable() (bool, error) {
	data, err := p.getProperty("seekable")
	if err != nil {
		return false, err
	}
	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return false, err
	}
	return v, nil
}

// SetTimePos sets the absolute playback position (seconds).
// Useful as a fallback when seek commands are ignored by HTTP demuxers.
func (p *Player) SetTimePos(seconds float64) error {
	return p.sendCommand("set_property", "time-pos", seconds)
}

// SeekUntil repeatedly seeks to target until the reported position is near it
// or the deadline / ctx fires. Waits for duration/seekable when possible.
func (p *Player) SeekUntil(ctx context.Context, target float64, deadline time.Time) error {
	if target < 0.5 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		dur, durErr := p.DurationSeconds()
		seekable, _ := p.Seekable()
		ready := seekable || (durErr == nil && dur > 1)
		if ready {
			if err := p.SeekAbsolute(target); err != nil {
				lastErr = err
				_ = p.SetTimePos(target)
			} else {
				lastErr = nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
			if pos, err := p.PositionSeconds(); err == nil && pos+2.5 >= target {
				return nil
			}
			// Property-set nudge if seek claimed success but position stuck early.
			if err := p.SetTimePos(target); err != nil {
				lastErr = err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	if lastErr != nil {
		return lastErr
	}
	if pos, err := p.PositionSeconds(); err == nil && pos+2.5 >= target {
		return nil
	}
	return fmt.Errorf("seek to %.1fs timed out", target)
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

// Volume returns the current mpv volume (typically 0–100).
func (p *Player) Volume() (float64, error) {
	data, err := p.getProperty("volume")
	if err != nil {
		return 0, err
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, err
	}
	return v, nil
}

// SetVolume sets mpv volume, clamped to 0–100.
func (p *Player) SetVolume(volume float64) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	return p.sendCommand("set_property", "volume", volume)
}

// Muted reports whether mpv audio is muted.
func (p *Player) Muted() (bool, error) {
	data, err := p.getProperty("mute")
	if err != nil {
		return false, err
	}
	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return false, err
	}
	return v, nil
}

// SetMute sets the mpv mute property.
func (p *Player) SetMute(mute bool) error {
	return p.sendCommand("set_property", "mute", mute)
}

// ToggleMute cycles the mute property.
func (p *Player) ToggleMute() error {
	return p.sendCommand("cycle", "mute")
}

// loudnormFilter is a practical one-pass loudness target for headphones.
const loudnormFilter = "loudnorm=I=-16:TP=-1.5:LRA=11"

// silenceSkipFilter removes silences throughout the track.
const silenceSkipFilter = "silenceremove=stop_periods=-1:stop_duration=1:stop_threshold=-50dB"

// SetAudioFilters enables or disables audio filters via mpv af.
func (p *Player) SetAudioFilters(normalize, silenceSkip bool, pitchSemi float64, eqFilter string) error {
	var filters []string
	if normalize {
		filters = append(filters, loudnormFilter)
	}
	if silenceSkip {
		filters = append(filters, silenceSkipFilter)
	}
	if pitchSemi != 0 {
		scale := math.Pow(2, pitchSemi/12.0)
		filters = append(filters, fmt.Sprintf("rubberband=pitch-scale=%.4f", scale))
	}
	if eqFilter != "" {
		filters = append(filters, eqFilter)
	}
	return p.sendCommand("set_property", "af", strings.Join(filters, ","))
}

// SetTempo sets the playback speed (tempo). Default is 1.0.
func (p *Player) SetTempo(speed float64) error {
	if speed < 0.25 {
		speed = 0.25
	}
	if speed > 4.0 {
		speed = 4.0
	}
	return p.sendCommand("set_property", "speed", speed)
}
