package player

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	if cmd == "mpv" {
		// Mock mpv: create a dummy unix socket
		var socketPath string
		for _, arg := range args {
			if strings.HasPrefix(arg, "--input-ipc-server=") {
				socketPath = strings.TrimPrefix(arg, "--input-ipc-server=")
			}
		}

		if socketPath != "" {
			listener, err := net.Listen("unix", socketPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to listen on unix socket: %v\n", err)
				os.Exit(1)
			}
			defer listener.Close()

			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					go func(c net.Conn) {
						defer c.Close()
						buf := make([]byte, 1024)
						for {
							n, err := c.Read(buf)
							if err != nil || n == 0 {
								break
							}
						}
					}(conn)
				}
			}()
		}

		// Keep running until killed
		time.Sleep(1 * time.Hour)
		os.Exit(0)
	}
	os.Exit(1)
}

func TestNewPlayer(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	player, err := NewPlayer()
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}
	defer player.Close()

	if player.cmd == nil {
		t.Errorf("Expected cmd to be set")
	}

	if player.conn == nil {
		t.Errorf("Expected conn to be set")
	}

	err = player.Play()
	if err != nil {
		t.Errorf("Play failed: %v", err)
	}

	err = player.Pause()
	if err != nil {
		t.Errorf("Pause failed: %v", err)
	}

	err = player.Load("dummy_url")
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}

	err = player.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}
