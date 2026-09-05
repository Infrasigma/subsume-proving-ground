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
	"sync"
)

var ErrBubblewrapUnavailable = errors.New("bubblewrap is unavailable")

// BubblewrapBackend runs an agent with no network namespace and only the
// explicitly mounted executable and broker directory visible. The broker
// remains outside the sandbox and owns all provider credentials/connections.
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

	bwrap := b.BwrapPath
	if bwrap == "" {
		var err error
		bwrap, err = exec.LookPath("bwrap")
		if err != nil {
			return nil, ErrBubblewrapUnavailable
		}
	}

	guestBrokerDir := "/aacr/run"
	guestExecutable := "/aacr/bin/agent"
	guestSocket := filepath.Join(guestBrokerDir, filepath.Base(opts.BrokerPath))

	args := []string{
		"--die-with-parent",
		"--new-session",
		"--clearenv",
		"--unshare-net",
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
		"--ro-bind", opts.Executable, guestExecutable,
		"--ro-bind", opts.BrokerDir, guestBrokerDir,
		guestExecutable,
	}
	args = append(args, opts.Args...)

	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Env = append([]string{}, opts.Environment...)
	cmd.Dir = "/"
	_ = guestSocket
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
// may do. Frames are newline-delimited JSON with a bounded size.
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
		max = 1 << 20
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
	reader := bufio.NewReaderSize(conn, max+1)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return err
	}
	if len(line) > max {
		return fmt.Errorf("broker frame exceeds maximum size of %d bytes", max)
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
