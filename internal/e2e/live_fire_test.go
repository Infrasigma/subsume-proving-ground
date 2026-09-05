package e2e

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/boundary"
	"github.com/Infrasigma/subsume-proving-ground/internal/broker"
	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	"github.com/Infrasigma/subsume-proving-ground/internal/evidence"
	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

type liveVerifier struct{ publicKey ed25519.PublicKey }

func (v liveVerifier) Verify(raw []byte) (provider.ActionContract, error) {
	var env protocol.Envelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil { return provider.ActionContract{}, fmt.Errorf("decode envelope: %w", err) }
	var extra any
	if err := dec.Decode(&extra); err != io.EOF { return provider.ActionContract{}, errors.New("envelope contains trailing JSON") }
	if env.Type != "ActionContract" { return provider.ActionContract{}, fmt.Errorf("unexpected envelope type %q", env.Type) }
	value, err := protocol.PayloadValue(env)
	if err != nil { return provider.ActionContract{}, err }
	canonical, err := c14n.Canonicalize(value)
	if err != nil { return provider.ActionContract{}, fmt.Errorf("canonicalize contract: %w", err) }
	domain, err := protocol.DomainForType(env.Type)
	if err != nil { return provider.ActionContract{}, err }
	if err := protocol.Verify(hex.EncodeToString(v.publicKey), domain, env, canonical); err != nil { return provider.ActionContract{}, err }
	payload, err := json.Marshal(value)
	if err != nil { return provider.ActionContract{}, err }
	var contract provider.ActionContract
	pd := json.NewDecoder(bytes.NewReader(payload))
	pd.UseNumber()
	pd.DisallowUnknownFields()
	if err := pd.Decode(&contract); err != nil { return provider.ActionContract{}, fmt.Errorf("decode action contract: %w", err) }
	if err := contract.ValidateForMutation(); err != nil { return provider.ActionContract{}, err }
	return contract, nil
}

type liveProviderResult struct {
	executionID string
	mutation provider.MutationResult
}

type liveProvider struct{ postgres *provider.PostgresProvider }

func (p liveProvider) Execute(ctx context.Context, contract provider.ActionContract, capability protocol.Envelope) (any, error) {
	value, err := protocol.PayloadValue(capability)
	if err != nil { return nil, fmt.Errorf("decode capability: %w", err) }
	m, ok := value.(map[string]any)
	if !ok { return nil, errors.New("capability payload is not an object") }
	executionID, ok := m["execution_id"].(string)
	if !ok || executionID == "" { return nil, errors.New("capability execution_id missing") }
	result, err := p.postgres.Execute(ctx, contract)
	if err != nil { return nil, err }
	return liveProviderResult{executionID: executionID, mutation: result}, nil
}

type liveEvidence struct{ pool *pgxpool.Pool }

func (e liveEvidence) Verify(contract provider.ActionContract, providerResult any) (any, error) {
	live, ok := providerResult.(liveProviderResult)
	if !ok { return nil, errors.New("unexpected provider result type") }
	userID, err := strconv.ParseInt(contract.Resource.ID, 10, 64)
	if err != nil { return nil, fmt.Errorf("resource id: %w", err) }
	var id, version int64
	var active bool
	if err := e.pool.QueryRow(context.Background(), `SELECT id, version, active FROM users WHERE id=$1`, userID).Scan(&id, &version, &active); err != nil {
		return nil, fmt.Errorf("observe committed postgres state: %w", err)
	}
	observed := map[string]any{"id": id, "version": version, "active": active}
	expectedHash, err := canonicalHash(contract.ExpectedEffect.Fields)
	if err != nil { return nil, fmt.Errorf("canonicalize expected effect: %w", err) }
	observedHash, err := canonicalHash(observed)
	if err != nil { return nil, fmt.Errorf("canonicalize observed effect: %w", err) }
	if expectedHash != observedHash { return nil, provider.ErrEffectMismatch }
	return evidence.NewDeactivateUser(live.executionID, live.mutation.RowsAffected, observed)
}

func canonicalHash(v any) ([32]byte, error) {
	b, err := json.Marshal(v)
	if err != nil { return [32]byte{}, err }
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil { return [32]byte{}, err }
	canonical, err := c14n.Canonicalize(value)
	if err != nil { return [32]byte{}, err }
	return protocol.PayloadHash(canonical), nil
}

type postgresHarness struct {
	pool *pgxpool.Pool
	address string
	stop func()
}

func startEphemeralPostgres(t *testing.T) postgresHarness {
	t.Helper()
	if runtime.GOOS != "linux" { t.Fatal("M6 live-fire requires Linux") }
	initdb := commandPath(t, "initdb")
	pgctl := commandPath(t, "pg_ctl")
	root := t.TempDir()
	dataDir := filepath.Join(root, "postgres")
	socketDir := filepath.Join(root, "socket")
	if err := os.MkdirAll(socketDir, 0700); err != nil { t.Fatal(err) }
	if err := runTimed(30*time.Second, initdb, "-D", dataDir, "-U", "aacr_e2e", "--auth=trust", "--no-locale"); err != nil { t.Fatalf("initdb: %v", err) }
	port := reserveTCPPort(t)
	address := fmt.Sprintf("127.0.0.1:%d", port)
	logPath := filepath.Join(root, "postgres.log")
	options := fmt.Sprintf("-p %d -h 127.0.0.1 -k %s", port, socketDir)
	if err := runTimed(30*time.Second, pgctl, "-D", dataDir, "-l", logPath, "-o", options, "-w", "-t", "20", "start"); err != nil {
		log, _ := os.ReadFile(logPath)
		t.Fatalf("start postgres: %v\n%s", err, log)
	}
	stop := func() { _ = runTimed(15*time.Second, pgctl, "-D", dataDir, "-m", "immediate", "stop") }
	t.Cleanup(stop)
	dsn := fmt.Sprintf("postgres://aacr_e2e@%s/postgres?sslmode=disable", address)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var pool *pgxpool.Pool
	var err error
	for {
		pool, err = pgxpool.New(ctx, dsn)
		if err == nil && pool.Ping(ctx) == nil { break }
		if pool != nil { pool.Close() }
		select {
		case <-ctx.Done(): t.Fatalf("connect to ephemeral postgres: %v", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `CREATE TABLE users (id BIGINT PRIMARY KEY, version BIGINT NOT NULL, active BOOLEAN NOT NULL)`); err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, `INSERT INTO users(id, version, active) VALUES (1842, 42, true), (1843, 42, true)`); err != nil { t.Fatal(err) }
	return postgresHarness{pool: pool, address: address, stop: stop}
}

func commandPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil { t.Fatalf("%s is required for M6 live-fire: %v", name, err) }
	return path
}

func runTimed(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() != nil { return fmt.Errorf("%w: %s", ctx.Err(), output.String()) }
	if err != nil { return fmt.Errorf("%w: %s", err, output.String()) }
	return nil
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func buildLiveAgent(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("runtime.Caller failed") }
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	binary := filepath.Join(t.TempDir(), "aacr-live-agent")
	cmd := exec.Command("go", "build", "-a", "-trimpath", "-o", binary, "./cmd/aacr-live-agent")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("build live agent: %v\n%s", err, output) }
	return binary
}

func mountBrokerTmpfs(t *testing.T) (string, func()) {
	t.Helper()
	mountPoint := filepath.Join(t.TempDir(), "broker-tmpfs")
	if err := os.Mkdir(mountPoint, 0700); err != nil { t.Fatal(err) }
	uid, gid := strconv.Itoa(os.Geteuid()), strconv.Itoa(os.Getegid())
	if err := runTimed(10*time.Second, "sudo", "mount", "-t", "tmpfs", "-o", "size=16m,uid="+uid+",gid="+gid+",mode=0700", "tmpfs", mountPoint); err != nil {
		t.Fatalf("mount broker tmpfs: %v", err)
	}
	cleanup := func() { _ = runTimed(10*time.Second, "sudo", "umount", mountPoint) }
	t.Cleanup(cleanup)
	return mountPoint, cleanup
}

func signContract(t *testing.T, contract provider.ActionContract, privateKey ed25519.PrivateKey) []byte {
	t.Helper()
	env, err := protocol.SignPayload("ActionContract", contract, "agent-live", privateKey)
	if err != nil { t.Fatal(err) }
	raw, err := json.Marshal(env)
	if err != nil { t.Fatal(err) }
	return raw
}

func liveContract(id string, expectedVersion int64, expectedActive bool, nonce string) provider.ActionContract {
	now := time.Now().UTC().Round(0)
	return provider.ActionContract{
		ContractVersion: "1.0", ActionID: "m6-live-fire-" + id, ExecutionClass: "MUTATION",
		Actor: provider.Actor{ID: "agent-live", WorkloadIdentity: "bubblewrap-live"}, Provider: "postgresql",
		Resource: provider.ResourceRef{Type: "users", ID: id}, Operation: "deactivate_user",
		Arguments: map[string]any{"user_id": mustInt64(id), "expected_version": int64(42)},
		Precondition: map[string]any{"version": int64(42), "active": true},
		ExpectedEffect: provider.ExpectedEffect{Resource: "users", ID: id, Fields: map[string]any{"id": mustInt64(id), "version": expectedVersion, "active": expectedActive}},
		MutationScope: provider.MutationScope{MaxAffectedObjects: 1}, ReadScope: provider.ReadScope{MaxRecords: 1, MaxBytes: 4096},
		DataEgressScope: provider.DataEgressScope{Allowed: false}, RecoveryMode: "RECONCILE",
		PolicyReference: provider.PolicyReference{PolicyID: "m6-live", Version: "1", Hash: "m6-live-policy"}, AssuranceRequirement: "SIGNED_RECEIPT",
		IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(2*time.Minute).Format(time.RFC3339Nano), Nonce: nonce,
	}
}

func mustInt64(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }

func startBroker(t *testing.T, b *broker.Broker, brokerDir string) string {
	t.Helper()
	socketPath := filepath.Join(brokerDir, "broker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil { t.Fatalf("listen unix socket: %v", err) }
	if err := os.Chmod(socketPath, 0600); err != nil { t.Fatal(err) }
	go func() { _ = b.Serve(listener) }()
	t.Cleanup(func() { _ = listener.Close() })
	return socketPath
}

func runSandboxAgent(t *testing.T, binary, socket string, raw []byte, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	agentArgs := append([]string{}, args...)
	if len(agentArgs) == 0 { agentArgs = []string{socket} }
	cmd, err := (boundary.BubblewrapBackend{}).Start(ctx, boundary.StartOptions{
		Executable: binary, BrokerDir: filepath.Dir(socket), BrokerPath: socket, Args: agentArgs,
		Environment: []string{"AACR_ENVELOPE_B64=" + base64.StdEncoding.EncodeToString(raw)},
	})
	if err != nil { t.Fatalf("start Bubblewrap agent: %v", err) }
	output, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("live agent failed: %v\n%s", err, output) }
	return output
}

func verifyReceipt(t *testing.T, receipt protocol.Receipt, publicKey ed25519.PublicKey) map[string]any {
	t.Helper()
	if receipt.Type != "Receipt" { t.Fatalf("receipt type = %q", receipt.Type) }
	if err := protocol.Verify(hex.EncodeToString(publicKey), "AACR/Receipt/v1", receipt, receipt.Payload); err != nil { t.Fatalf("independent receipt verification: %v", err) }
	value, err := protocol.PayloadValue(receipt)
	if err != nil { t.Fatal(err) }
	payload, ok := value.(map[string]any)
	if !ok { t.Fatal("receipt payload is not an object") }
	return payload
}

func decodeReceipt(t *testing.T, output []byte) protocol.Receipt {
	t.Helper()
	var receipt protocol.Receipt
	if err := json.Unmarshal(output, &receipt); err != nil { t.Fatalf("decode receipt: %v\n%s", err, output) }
	return receipt
}

func TestM6LiveFire(t *testing.T) {
	if os.Geteuid() == 0 { t.Skip("M6 live-fire requires an unprivileged runner for Bubblewrap") }
	pg := startEphemeralPostgres(t)
	registry, err := provider.NewOperationRegistry(provider.PostgresDeactivateUserHandler{})
	if err != nil { t.Fatal(err) }
	postgres, err := provider.NewPostgresProvider(pg.pool, registry)
	if err != nil { t.Fatal(err) }
	l, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = l.Close() })
	agentPublic, agentPrivate, err := ed25519.GenerateKey(nil)
	if err != nil { t.Fatal(err) }
	brokerPublic, brokerPrivate, err := ed25519.GenerateKey(nil)
	if err != nil { t.Fatal(err) }
	b, err := broker.New(liveVerifier{publicKey: agentPublic}, l, liveProvider{postgres: postgres}, liveEvidence{pool: pg.pool}, "broker-live", "postgresql", "bubblewrap-live", brokerPrivate)
	if err != nil { t.Fatal(err) }
	brokerDir, _ := mountBrokerTmpfs(t)
	socket := startBroker(t, b, brokerDir)
	binary := buildLiveAgent(t)

	raw := signContract(t, liveContract("1842", 43, false, "nonce-live-1842"), agentPrivate)
	output := runSandboxAgent(t, binary, socket, raw)
	payload := verifyReceipt(t, decodeReceipt(t, output), brokerPublic)
	if payload["status"] != ledger.StateCommitted { t.Fatalf("happy-path status = %v", payload["status"]) }
	execID, ok := payload["execution_id"].(string)
	if !ok || execID == "" { t.Fatal("happy-path execution_id missing") }
	events, err := l.Events(context.Background(), execID)
	if err != nil { t.Fatal(err) }
	if err := ledger.VerifyChain(events); err != nil { t.Fatalf("ledger chain verification: %v", err) }
	if len(events) != 3 || events[0].EventType != ledger.StateAuthorized || events[1].EventType != ledger.StateDispatched || events[2].EventType != ledger.StateCommitted { t.Fatalf("unexpected lifecycle: %+v", events) }
	var version int64
	var active bool
	if err := pg.pool.QueryRow(context.Background(), `SELECT version, active FROM users WHERE id=1842`).Scan(&version, &active); err != nil { t.Fatal(err) }
	if version != 43 || active { t.Fatalf("committed state = version %d active %v", version, active) }

	probe := runSandboxAgent(t, binary, socket, raw, "network-probe", pg.address)
	if !strings.Contains(string(probe), "network is unreachable") { t.Fatalf("network bypass did not prove ENETUNREACH: %s", probe) }

	replay := runSandboxAgent(t, binary, socket, raw)
	var replayReceipt protocol.Receipt
	if err := json.Unmarshal(replay, &replayReceipt); err != nil { t.Fatal(err) }
	if replayReceipt.Type != "" || len(replayReceipt.Payload) != 0 { t.Fatalf("replay unexpectedly returned a receipt: %s", replay) }
	var unchanged int
	if err := pg.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE id=1842 AND version=43 AND active=false`).Scan(&unchanged); err != nil { t.Fatal(err) }
	if unchanged != 1 { t.Fatalf("replay changed database state: %d", unchanged) }

	forged := signContract(t, liveContract("1843", 43, true, "nonce-forged-1843"), agentPrivate)
	forgedReceipt := decodeReceipt(t, runSandboxAgent(t, binary, socket, forged))
	forgedPayload := verifyReceipt(t, forgedReceipt, brokerPublic)
	if forgedPayload["status"] != ledger.StateAborted { t.Fatalf("forged status = %v, want ABORTED", forgedPayload["status"]) }
	forgedExec, ok := forgedPayload["execution_id"].(string)
	if !ok || forgedExec == "" { t.Fatal("forged execution_id missing") }
	forgedEvents, err := l.Events(context.Background(), forgedExec)
	if err != nil { t.Fatal(err) }
	if err := ledger.VerifyChain(forgedEvents); err != nil { t.Fatalf("forged ledger chain verification: %v", err) }
	if len(forgedEvents) != 3 || forgedEvents[2].EventType != ledger.StateAborted { t.Fatalf("forged lifecycle: %+v", forgedEvents) }
	if err := pg.pool.QueryRow(context.Background(), `SELECT version, active FROM users WHERE id=1843`).Scan(&version, &active); err != nil { t.Fatal(err) }
	if version != 42 || !active { t.Fatalf("forged mutation was committed: version %d active %v", version, active) }

	t.Logf("M6 LIVE FIRE PASS: execution=%s postgres=%s replay=blocked forgery=aborted", execID, pg.address)
}
