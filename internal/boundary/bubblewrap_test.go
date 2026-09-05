package boundary

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBubblewrapMediatedChannelAndNetworkIsolation(t *testing.T) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bubblewrap is not installed")
	}
	probe := "/tmp/aacr-boundary-probe"
	// go test runs this package with internal/boundary as cwd, so the probe
	// package must be addressed relative to that directory, not the repo root.
	build := exec.Command("go", "build", "-o", probe, "../../cmd/aacr-boundary-probe")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build probe: %v\n%s", err, out)
	}

	runDir := t.TempDir()
	socket := filepath.Join(runDir, "broker.sock")
	listener, err := ListenUnix(socket)
	if err != nil { t.Fatal(err) }
	defer listener.Close()
	defer os.Remove(socket)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	broker := &Broker{
		Listener: listener,
		Handler: func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
			var got map[string]any
			if err := json.Unmarshal(payload, &got); err != nil { return nil, err }
			if got["probe"] != "mediated-channel" { return nil, os.ErrInvalid }
			return json.RawMessage(`{"accepted":true}`), nil
		},
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- broker.Serve(ctx) }()

	cmd, err := (BubblewrapBackend{BwrapPath: bwrap}).Start(ctx, StartOptions{
		Executable: probe,
		BrokerDir: runDir,
		BrokerPath: socket,
	})
	if err != nil { t.Fatal(err) }
	output, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("sandbox probe failed: %v\n%s", err, output) }
	text := string(output)
	if !strings.Contains(text, `{"accepted":true}`) { t.Fatalf("mediated channel was not reachable: %s", text) }
	if !strings.Contains(text, `{"network":"blocked"}`) { t.Fatalf("sandbox did not prove network isolation: %s", text) }

	cancel()
	_ = listener.Close()
	select {
	case <-serveErr:
	case <-time.After(time.Second):
		t.Fatal("broker did not stop")
	}
}

func TestListenUnixIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broker.sock")
	l, err := ListenUnix(path)
	if err != nil { t.Fatal(err) }
	defer l.Close()
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if got := info.Mode().Perm(); got != 0600 { t.Fatalf("socket mode = %o, want 0600", got) }
}

func TestStartRejectsRelativePaths(t *testing.T) {
	_, err := (BubblewrapBackend{BwrapPath: "/bin/false"}).Start(context.Background(), StartOptions{Executable: "probe", BrokerDir: "/tmp", BrokerPath: "/tmp/broker.sock"})
	if err == nil { t.Fatal("expected relative executable to be rejected") }
}

func TestBrokerRejectsOversizedUnterminatedFrame(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	max := 4 << 20
	done := make(chan error, 1)
	go func() {
		done <- serveConn(context.Background(), server, func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("handler must not be reached")
		}, max)
	}()

	w := bufio.NewWriter(client)
	chunk := strings.Repeat("x", 64<<10)
	written := 0
	for written <= max {
		n, err := w.WriteString(chunk)
		if err != nil { t.Fatal(err) }
		written += n
		if written > max { break }
	}
	if err := w.Flush(); err != nil { t.Fatal(err) }

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Fatalf("unexpected oversized-frame result: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("oversized frame was not rejected promptly")
	}
}
