package e2e

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/boundary"
	"github.com/Infrasigma/subsume-proving-ground/internal/broker"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresHarness owns a completely isolated ephemeral PostgreSQL instance.
type postgresHarness struct {
	pool    *pgxpool.Pool
	address string
	stop    func()
}

func startEphemeralPostgres(t *testing.T) postgresHarness {
	t.Helper()
	ctx := context.Background()
	port := freeTCPPort(t)
	root := t.TempDir()
	dataDir := filepath.Join(root, "pgdata")
	socketDir := filepath.Join(root, "pgsocket")

	// initdb must receive a genuinely empty path and create the cluster itself.
	if out, err := exec.Command("initdb", "-D", dataDir, "--no-locale", "--encoding=UTF8", "--auth=trust").CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v\n%s", err, out)
	}
	// Keep the Unix socket outside PGDATA so harness setup can never make the
	// initdb target non-empty. It is created only after initdb succeeds.
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		t.Fatalf("create postgres socket dir: %v", err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	pgOptions := fmt.Sprintf("-p %d -h 127.0.0.1 -k %s", port, socketDir)

	// pg_ctl backgrounds postgres. Never attach Go-owned pipes to pg_ctl's
	// stdout/stderr: the daemon inherits those descriptors and can keep them
	// open after pg_ctl exits, making Cmd.Wait block waiting for EOF forever.
	pgLogPath := filepath.Join(root, "postgres.log")
	pgLog, err := os.OpenFile(pgLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("create postgres log: %v", err)
	}
	cmd := exec.Command("pg_ctl", "-D", dataDir, "-o", pgOptions, "-w", "start")
	cmd.Stdout = pgLog
	cmd.Stderr = pgLog
	if err := cmd.Run(); err != nil {
		_ = pgLog.Close()
		if out, readErr := os.ReadFile(pgLogPath); readErr == nil {
			t.Fatalf("pg_ctl start: %v\n%s", err, out)
		}
		t.Fatalf("pg_ctl start: %v", err)
	}
	if err := pgLog.Close(); err != nil {
		t.Fatalf("close postgres log: %v", err)
	}

	stop := func() { _ = exec.Command("pg_ctl", "-D", dataDir, "-m", "immediate", "-w", "stop").Run() }
	t.Cleanup(stop)
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://postgres@%s/postgres?sslmode=disable", address))
	if err != nil { t.Fatal(err) }
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	t.Cleanup(pool.Close)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := pool.Ping(ctx); err == nil { break }
		if time.Now().After(deadline) { t.Fatalf("postgres readiness timeout: %v", err) }
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE users (id BIGINT PRIMARY KEY, version BIGINT NOT NULL, active BOOLEAN NOT NULL)`); err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, `INSERT INTO users(id, version, active) VALUES (1842,42,true),(1843,42,true)`); err != nil { t.Fatal(err) }
	return postgresHarness{pool: pool, address: address, stop: stop}
}

func freeTCPPort(t *testing.T) int { t.Helper(); l, err := net.Listen("tcp", "127.0.0.1:0"); if err != nil { t.Fatal(err) }; defer l.Close(); return l.Addr().(*net.TCPAddr).Port }
func (h postgresHarness) HostPort() string { return h.address }

func buildLiveAgent(t *testing.T) string {
	t.Helper(); binary := filepath.Join(t.TempDir(), "aacr-live-agent")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", binary, "./cmd/aacr-live-agent"); cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("build live agent: %v\n%s", err, out) }; return binary
}
func mountBrokerTmpfs(t *testing.T) (string, func()) { t.Helper(); brokerDir, err := os.MkdirTemp("/dev/shm", "aacr-broker-tmpfs-"); if err != nil { t.Fatal(err) }; if err := os.Chmod(brokerDir, 0700); err != nil { t.Fatal(err) }; cleanup := func() { _ = os.RemoveAll(brokerDir) }; t.Cleanup(cleanup); return brokerDir, cleanup }
func signContract(t *testing.T, contract provider.ActionContract, privateKey ed25519.PrivateKey) []byte { t.Helper(); env, err := protocol.SignPayload("ActionContract", contract, "agent-live", privateKey); if err != nil { t.Fatal(err) }; raw, err := json.Marshal(env); if err != nil { t.Fatal(err) }; return raw }
func liveContract(id string, expectedVersion int64, expectedActive bool, nonce string) provider.ActionContract { now := time.Now().UTC().Round(0); return provider.ActionContract{ContractVersion:"1.0",ActionID:"m6-live-fire-"+id,ExecutionClass:"MUTATION",Actor:provider.Actor{ID:"agent-live",WorkloadIdentity:"bubblewrap-live"},Provider:"postgresql",Resource:provider.ResourceRef{Type:"users",ID:id},Operation:"deactivate_user",Arguments:map[string]any{"user_id":mustInt64(id),"expected_version":int64(42)},Precondition:map[string]any{"version":int64(42),"active":true},ExpectedEffect:provider.ExpectedEffect{Resource:"users",ID:id,Fields:map[string]any{"id":mustInt64(id),"version":expectedVersion,"active":expectedActive}},MutationScope:provider.MutationScope{MaxAffectedObjects:1},ReadScope:provider.ReadScope{MaxRecords:1,MaxBytes:4096},DataEgressScope:provider.DataEgressScope{Allowed:false},RecoveryMode:"RECONCILE",PolicyReference:provider.PolicyReference{PolicyID:"m6-live",Version:"1",Hash:"m6-live-policy"},AssuranceRequirement:"SIGNED_RECEIPT",IssuedAt:now.Format(time.RFC3339Nano),ExpiresAt:now.Add(2*time.Minute).Format(time.RFC3339Nano),Nonce:nonce} }
func mustInt64(s string) int64 { n, _ := strconv.ParseInt(s,10,64); return n }
func startBroker(t *testing.T, b *broker.Broker, brokerDir string) string { t.Helper(); socketPath := filepath.Join(brokerDir,"broker.sock"); listener, err := net.Listen("unix",socketPath); if err != nil { t.Fatalf("listen unix socket: %v",err) }; if err := os.Chmod(socketPath,0600); err != nil { t.Fatal(err) }; go func(){ _ = b.Serve(listener) }(); t.Cleanup(func(){ _ = listener.Close() }); return socketPath }
func runSandboxAgent(t *testing.T,binary,socket string,raw []byte,args ...string) []byte { t.Helper(); ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second); defer cancel(); agentArgs:=append([]string{},args...); if len(agentArgs)==0 { agentArgs=[]string{"/run/aacr/broker.sock"} }; cmd,err:=(boundary.BubblewrapBackend{}).Start(ctx,boundary.StartOptions{Executable:binary,BrokerDir:filepath.Dir(socket),BrokerPath:socket,Args:agentArgs,Environment:[]string{"AACR_ENVELOPE_B64="+base64.StdEncoding.EncodeToString(raw)}}); if err != nil { t.Fatalf("start Bubblewrap agent: %v",err) }; output,err:=cmd.CombinedOutput(); if err != nil { t.Fatalf("live agent failed: %v\n%s",err,output) }; return output }
