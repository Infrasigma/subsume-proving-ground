package e2e

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/boundary"
)

func TestM6LiveFireNetworkBypass(t *testing.T) {
	if os.Geteuid() == 0 { t.Skip("live-fire must run as an unprivileged user for Bubblewrap") }
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

	target := fmt.Sprintf("127.0.0.1:%d", pool.Config().ConnConfig.Port)
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
	if got := string(output); got == "" || !containsNetworkUnreachable(got) {
		t.Fatalf("expected ENETUNREACH from sandbox to PostgreSQL %s, got %q", target, got)
	}
	t.Logf("M6 network fence PASS: sandbox -> %s rejected with ENETUNREACH", target)
}
