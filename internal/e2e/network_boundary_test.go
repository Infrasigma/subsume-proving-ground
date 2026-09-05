package e2e

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/boundary"
)

func TestM6LiveFireNetworkBypass(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("live-fire must run as an unprivileged user for Bubblewrap")
	}
	pool := startEphemeralPostgres(t)
	binary := buildLiveAgent(t)
	brokerDir, err := os.MkdirTemp("/dev/shm", "aacr-network-probe-")
	if err != nil { t.Fatal(err) }
	defer os.RemoveAll(brokerDir)
	if err := os.Chmod(brokerDir, 0700); err != nil { t.Fatal(err) }
	socket := filepath.Join(brokerDir, "broker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil { t.Fatal(err) }
	defer listener.Close()
	if err := os.Chmod(socket, 0600); err != nil { t.Fatal(err) }

	// startEphemeralPostgres returns only after a successful Ping, so the
	// target is known to be listening before the sandbox is launched.
	target := pool.HostPort()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd, err := (boundary.BubblewrapBackend{}).Start(ctx, boundary.StartOptions{
		Executable: binary,
		BrokerDir: brokerDir,
		BrokerPath: socket,
		Args: []string{"network-probe", target},
	})
	if err != nil { t.Fatal(err) }
	output, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("network bypass probe failed: %v\n%s", err, output) }
	got := string(output)

	// 127.0.0.1 is intentionally unshared: the sandbox sees its own loopback,
	// not the host's PostgreSQL listener. ECONNREFUSED is the correct physical
	// proof for this host-loopback isolation case.
	if !strings.Contains(got, "connection refused") {
		t.Fatalf("expected host-loopback isolation from PostgreSQL %s, got %q", target, got)
	}
	t.Logf("M6 network fence PASS: sandbox -> host PostgreSQL %s rejected on isolated loopback", target)

	// A non-loopback destination proves the stronger routing invariant: the
	// unshared namespace has no usable route to external networks.
	routeProbe := "198.51.100.1:" + target[strings.LastIndex(target, ":")+1:]
	cmd, err = (boundary.BubblewrapBackend{}).Start(ctx, boundary.StartOptions{
		Executable: binary,
		BrokerDir: brokerDir,
		BrokerPath: socket,
		Args: []string{"network-probe", routeProbe},
	})
	if err != nil { t.Fatal(err) }
	output, err = cmd.CombinedOutput()
	if err != nil { t.Fatalf("routing probe failed: %v\n%s", err, output) }
	if got = string(output); !strings.Contains(got, "network is unreachable") {
		t.Fatalf("expected ENETUNREACH from sandbox to %s, got %q", routeProbe, got)
	}
	t.Logf("M6 routing fence PASS: sandbox -> %s rejected with ENETUNREACH", routeProbe)
}
