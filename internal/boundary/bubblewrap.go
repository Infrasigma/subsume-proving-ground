package boundary

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var ErrBubblewrapUnavailable = errors.New("bubblewrap is unavailable")

// BubblewrapBackend runs an agent with no network, PID, IPC, UTS, or cgroup
// namespace sharing and only the explicitly mounted executable and broker
// directory visible. The broker remains outside the sandbox and owns all
// provider credentials/connections.
type BubblewrapBackend struct {
	BwrapPath string
}

type StartOptions struct {
	Executable  string
	Args        []string
	BrokerDir   string
	BrokerPath  string
	Environment []string
}

// Start launches an executable inside a minimal bubblewrap filesystem. The
// executable must be self-contained (for example a static Go binary), or its
// complete runtime must be explicitly added as future read-only mounts.
func (b BubblewrapBackend) Start(ctx context.Context, opts StartOptions) (*exec.Cmd, error) {
	if opts.Executable == "" || opts.BrokerDir == "" || opts.BrokerPath == "" {
		return nil, fmt.Errorf("executable, broker directory and broker path are required")
	}
	if err := validatePath(opts.Executable); err != nil {
		return nil, fmt.Errorf("executable: %w", err)
	}
	if err := validatePath(opts.BrokerDir); err != nil {
		return nil, fmt.Errorf("broker directory: %w", err)
	}
	if err := validatePath(opts.BrokerPath); err != nil {
		return nil, fmt.Errorf("broker path: %w", err)
	}
	if filepath.Dir(opts.BrokerPath) != opts.BrokerDir {
		return nil, fmt.Errorf("broker socket must be directly inside broker directory")
	}

	bwrap := b.BwrapPath
	if bwrap == "" {
		var err error
		bwrap, err = exec.LookPath("bwrap")
		if err != nil {
			return nil, ErrBubblewrapUnavailable
		}
	}

	guestBrokerDir := "/run/aacr"
	guestExecutable := "/aacr/bin/agent"

	args := []string{
		"--die-with-parent",
		"--new-session",
		"--clearenv",
		"--unshare-net",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--unshare-cgroup",
		"--dir", "/aacr",
		"--dir", "/aacr/bin",
		"--dir", "/run",
		"--dir", guestBrokerDir,
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
		"--ro-bind", opts.Executable, guestExecutable,
		"--bind", opts.BrokerDir, guestBrokerDir,
	}
	for _, env := range opts.Environment {
		key, _, ok := strings.Cut(env, "=")
		if !ok || key == "" || strings.ContainsAny(key, " \t\r\n") {
			return nil, fmt.Errorf("invalid environment assignment")
		}
		args = append(args, "--setenv", key, env[len(key)+1:])
	}
	args = append(args, guestExecutable)
	args = append(args, opts.Args...)

	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Env = []string{}
	cmd.Dir = "/"
	return cmd, nil
}

func validatePath(p string) error {
	if !filepath.IsAbs(p) {
		return fmt.Errorf("path must be absolute")
	}
	if _, err := os.Stat(p); err != nil {
		return err
	}
	return nil
}

// Broker is the host-side channel exposed to a sandboxed agent. It has no
// provider access itself; the supplied handler decides what a protocol request
// may do. Frames are newline-delimited JSON with a hard byte limit.
type Broker struct {
	Listener net.Listener
	Handler  func(context.Context, json.RawMessage) (json.RawMessage, error)
	MaxFrame int
}

type brokerResponse struct {
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func (b *Broker) Serve(ctx context.Context) error {
	if b.Listener == nil || b.Handler == nil {
		return fmt.Errorf("listener and handler are required")
	}
	max := b.MaxFrame
	if max <= 0 {
		max = 4 << 20
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := b.Listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = serveConn(ctx, conn, b.Handler, max)
		}()
	}
}

func serveConn(ctx context.Context, conn net.Conn, handler func(context.Context, json.RawMessage) (json.RawMessage, error), max int) error {
	defer conn.Close()
	reader := bufio.NewReaderSize(conn, min(max, 64<<10))
	line, err := readBoundedLine(reader, max)
	if err != nil {
		return err
	}
	var payload json.RawMessage
	if err := json.Unmarshal(line, &payload); err != nil {
		return fmt.Errorf("invalid broker JSON: %w", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	out, err := handler(ctx, payload)
	resp := brokerResponse{OK: err == nil, Payload: out}
	if err != nil {
		resp.Error = err.Error()
		resp.Payload = nil
	}
	enc, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	enc = append(enc, '\n')
	_, err = conn.Write(enc)
	return err
}

func readBoundedLine(reader *bufio.Reader, max int) ([]byte, error) {
	if max <= 0 {
		return nil, fmt.Errorf("maximum frame size must be positive")
	}
	buf := make([]byte, 0, min(max, 64<<10))
	for {
		part, err := reader.ReadSlice('\n')
		if len(part) > 0 {
			if len(buf)+len(part) > max {
				return nil, fmt.Errorf("broker frame exceeds maximum size of %d bytes", max)
			}
			buf = append(buf, part...)
		}
		if err == nil {
			return buf, nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
