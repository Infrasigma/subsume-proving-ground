package e2e

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/boundary"
	"github.com/Infrasigma/subsume-proving-ground/internal/broker"
	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
	"github.com/jackc/pgx/v5"
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
	if out, err := exec.Command("initdb", "-U", "postgres", "-D", dataDir, "--no-locale", "--encoding=UTF8", "--auth=trust").CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v\n%s", err, out)
	}
	// Keep the Unix socket outside PGDATA so harness setup can never make the
	// initdb target non-empty. It is created only after initdb succeeds.
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		t.Fatalf("create postgres socket dir: %v", err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	pgOptions := fmt.Sprintf("-p %d -h 127.0.0.1 -k %s", port, socketDir)

	// Sever pg_ctl stdout/stderr from the daemon so inherited CI pipes cannot
	// keep the pg_ctl child blocked after PostgreSQL backgrounds itself.
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
	// initdb -U postgres and this explicit client user must stay paired. Do not
	// remove the user from this DSN or pgx will fall back to the runner identity.
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://postgres@%s/postgres?sslmode=disable", address))
	if err != nil { t.Fatal(err) }
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	t.Cleanup(pool.Close)
	deadline := time.Now().Add(10 * time.Second)
	var lastPingErr error
	for {
		lastPingErr = pool.Ping(ctx)
		if lastPingErr == nil { break }
		if time.Now().After(deadline) {
			logData, readErr := os.ReadFile(pgLogPath)
			if readErr != nil {
				t.Logf("PostgreSQL Daemon Log: <unable to read %s: %v>", pgLogPath, readErr)
			} else {
				t.Logf("PostgreSQL Daemon Log:\n%s", logData)
			}
			t.Fatalf("postgres readiness timeout dialing %s: %v", address, lastPingErr)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE users (id BIGINT PRIMARY KEY, version BIGINT NOT NULL, active BOOLEAN NOT NULL)`); err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, `INSERT INTO users(id, version, active) VALUES (1842,42,true),(1843,42,true)`); err != nil { t.Fatal(err) }
	return postgresHarness{pool: pool, address: address, stop: stop}
}

func freeTCPPort(t *testing.T) int { t.Helper(); l, err := net.Listen("tcp", "127.0.0.1:0"); if err != nil { t.Fatal(err) }; defer l.Close(); return l.Addr().(*net.TCPAddr).Port }
func (h postgresHarness) HostPort() string { return h.address }

func buildLiveAgent(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve runtime.Caller for source anchoring")
	}
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	binary := filepath.Join(t.TempDir(), "aacr-live-agent")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", binary, "./cmd/aacr-live-agent")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build live agent: %v\n%s", err, out)
	}
	return binary
}
func mountBrokerTmpfs(t *testing.T) (string, func()) { t.Helper(); brokerDir, err := os.MkdirTemp("/dev/shm", "aacr-broker-tmpfs-"); if err != nil { t.Fatal(err) }; if err := os.Chmod(brokerDir, 0700); err != nil { t.Fatal(err) }; cleanup := func() { _ = os.RemoveAll(brokerDir) }; t.Cleanup(cleanup); return brokerDir, cleanup }
func signContract(t *testing.T, contract provider.ActionContract, privateKey ed25519.PrivateKey) []byte { t.Helper(); env, err := protocol.SignPayload("ActionContract", contract, "agent-live", privateKey); if err != nil { t.Fatal(err) }; raw, err := json.Marshal(env); if err != nil { t.Fatal(err) }; return raw }
func liveContract(id string, expectedVersion int64, expectedActive bool, nonce string) provider.ActionContract { now := time.Now().UTC().Round(0); return provider.ActionContract{ContractVersion:"1.0",ActionID:"m6-live-fire-"+id,ExecutionClass:"MUTATION",Actor:provider.Actor{ID:"agent-live",WorkloadIdentity:"bubblewrap-live"},Provider:"postgresql",Resource:provider.ResourceRef{Type:"users",ID:id},Operation:"deactivate_user",Arguments:map[string]any{"user_id":mustInt64(id),"expected_version":int64(42)},Precondition:map[string]any{"version":int64(42),"active":true},ExpectedEffect:provider.ExpectedEffect{Resource:"users",ID:id,Fields:map[string]any{"id":mustInt64(id),"version":expectedVersion,"active":expectedActive}},MutationScope:provider.MutationScope{MaxAffectedObjects:1},ReadScope:provider.ReadScope{MaxRecords:1,MaxBytes:4096},DataEgressScope:provider.DataEgressScope{Allowed:false},RecoveryMode:"RECONCILE",PolicyReference:provider.PolicyReference{PolicyID:"m6-live",Version:"1",Hash:"m6-live-policy"},AssuranceRequirement:"SIGNED_RECEIPT",IssuedAt:now.Format(time.RFC3339Nano),ExpiresAt:now.Add(2*time.Minute).Format(time.RFC3339Nano),Nonce:nonce} }
func mustInt64(s string) int64 { n, _ := strconv.ParseInt(s,10,64); return n }
func startBroker(t *testing.T, b *broker.Broker, brokerDir string) string { t.Helper(); socketPath := filepath.Join(brokerDir,"broker.sock"); listener, err := net.Listen("unix",socketPath); if err != nil { t.Fatalf("listen unix socket: %v",err) }; if err := os.Chmod(socketPath,0600); err != nil { t.Fatal(err) }; go func(){ _ = b.Serve(listener) }(); t.Cleanup(func(){ _ = listener.Close() }); return socketPath }
func runSandboxAgent(t *testing.T,binary,socket string,raw []byte,args ...string) []byte { t.Helper(); ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second); defer cancel(); agentArgs:=append([]string{},args...); if len(agentArgs)==0 { agentArgs=[]string{"/run/aacr/broker.sock"} }; cmd,err:=(boundary.BubblewrapBackend{}).Start(ctx,boundary.StartOptions{Executable:binary,BrokerDir:filepath.Dir(socket),BrokerPath:socket,Args:agentArgs,Environment:[]string{"AACR_ENVELOPE_B64="+base64.StdEncoding.EncodeToString(raw)}}); if err != nil { t.Fatalf("start Bubblewrap agent: %v",err) }; output,err:=cmd.CombinedOutput(); if err != nil { t.Fatalf("live agent failed: %v\n%s",err,output) }; return output }
func verifyReceipt(t *testing.T,receipt protocol.Receipt,publicKey ed25519.PublicKey) map[string]any { t.Helper(); if receipt.Type!="Receipt" { t.Fatalf("receipt type = %q",receipt.Type) }; if err:=protocol.Verify(hex.EncodeToString(publicKey),"AACR/Receipt/v1",receipt,receipt.Payload); err!=nil { t.Fatalf("independent receipt verification: %v",err) }; value,err:=protocol.PayloadValue(receipt); if err!=nil { t.Fatal(err) }; payload,ok:=value.(map[string]any); if !ok { t.Fatal("receipt payload is not an object") }; return payload }
func decodeReceipt(t *testing.T,output []byte) protocol.Receipt { t.Helper(); var receipt protocol.Receipt; if err:=json.Unmarshal(output,&receipt); err!=nil { t.Fatalf("decode receipt: %v\n%s",err,output) }; _ = receipt.Type; t.Logf("COMPILED STRUCT TYPE: %T", receipt); t.Logf("COMPILED STRUCT REFLECT: %+v", reflect.TypeOf(receipt)); if tField, ok := reflect.TypeOf(receipt).FieldByName("Type"); ok { t.Logf("TYPE FIELD EXACT TAG: `%s`", tField.Tag.Get("json")) } else { t.Logf("TYPE FIELD MISSING IN COMPILED STRUCT") }; return receipt }

func TestM6LiveFire(t *testing.T) {
	if os.Geteuid()==0 { t.Skip("M6 live-fire requires an unprivileged runner for Bubblewrap") }
	pg:=startEphemeralPostgres(t); registry,err:=provider.NewOperationRegistry(provider.PostgresDeactivateUserHandler{}); if err!=nil { t.Fatal(err) }; postgres,err:=provider.NewPostgresProvider(pg.pool,registry); if err!=nil { t.Fatal(err) }; l,err:=ledger.Open(filepath.Join(t.TempDir(),"ledger.db")); if err!=nil { t.Fatal(err) }; t.Cleanup(func(){_ = l.Close()}); agentPublic,agentPrivate,err:=ed25519.GenerateKey(rand.Reader); if err!=nil { t.Fatal(err) }; brokerPublic,brokerPrivate,err:=ed25519.GenerateKey(rand.Reader); if err!=nil { t.Fatal(err) }; liveVerifier:=liveVerifier{publicKey:agentPublic}; liveProvider:=liveProvider{postgres:postgres}; liveEvidence:=liveEvidence{pool:pg.pool}; b,err:=broker.New(liveVerifier,l,liveProvider,liveEvidence,"broker-live","postgresql","bubblewrap-live",brokerPrivate); if err!=nil { t.Fatal(err) }; brokerDir,_:=mountBrokerTmpfs(t); socket:=startBroker(t,b,brokerDir); binary:=buildLiveAgent(t)
	raw:=signContract(t,liveContract("1842",43,false,"nonce-live-1842"),agentPrivate); output:=runSandboxAgent(t,binary,socket,raw); t.Logf("RAW AGENT STDOUT: %s", string(output)); t.Logf("RAW AGENT HEX: %x", output); payload:=verifyReceipt(t,decodeReceipt(t,output),brokerPublic); if payload["status"]!=ledger.StateCommitted { t.Fatalf("happy-path status = %v",payload["status"]) }; execID,ok:=payload["execution_id"].(string); if !ok||execID=="" { t.Fatal("happy-path execution_id missing") }; events,err:=l.Events(context.Background(),execID); if err!=nil { t.Fatal(err) }; if err:=ledger.VerifyChain(events); err!=nil { t.Fatalf("ledger chain verification: %v",err) }; if len(events)!=3||events[0].EventType!=ledger.StateAuthorized||events[1].EventType!=ledger.StateDispatched||events[2].EventType!=ledger.StateCommitted { t.Fatalf("unexpected lifecycle: %+v",events) }; t.Logf("M6 WAL lifecycle PASS: AUTHORIZED -> DISPATCHED -> COMMITTED; execution=%s",execID); var version int64; var active bool; if err:=pg.pool.QueryRow(context.Background(),`SELECT version, active FROM users WHERE id=1842`).Scan(&version,&active); err!=nil { t.Fatal(err) }; if version!=43||active { t.Fatalf("committed state = version %d active %v",version,active) }
	probe:=runSandboxAgent(t,binary,socket,raw,"network-probe",pg.address); if !strings.Contains(string(probe),"connection refused") { t.Fatalf("network bypass did not prove ECONNREFUSED on isolated loopback: %s",probe) }; t.Logf("M6 network fence PASS: sandbox -> host PostgreSQL %s rejected with ECONNREFUSED",pg.address)
	replay:=runSandboxAgent(t,binary,socket,raw); var replayError struct{ Error string `json:"error"` }; if err:=json.Unmarshal(replay,&replayError); err!=nil { t.Fatalf("decode replay rejection: %v\n%s",err,replay) }; if replayError.Error!="duplicate_nonce" { t.Fatalf("replay rejection = %q, want duplicate_nonce",replayError.Error) }; var unchanged int; if err:=pg.pool.QueryRow(context.Background(),`SELECT count(*) FROM users WHERE id=1842 AND version=43 AND active=false`).Scan(&unchanged); err!=nil { t.Fatal(err) }; if unchanged!=1 { t.Fatalf("replay changed database state: %d",unchanged) }; t.Logf("M6 UNIQUE nonce PASS: exact signed envelope replay rejected by durable SQLite nonce constraint; wire_error=duplicate_nonce")
	forged:=signContract(t,liveContract("1843",43,true,"nonce-forged-1843"),agentPrivate); forgedReceipt:=decodeReceipt(t,runSandboxAgent(t,binary,socket,forged)); forgedPayload:=verifyReceipt(t,forgedReceipt,brokerPublic); if forgedPayload["status"]!=ledger.StateAborted { t.Fatalf("forged status = %v, want ABORTED",forgedPayload["status"]) }; forgedExec,ok:=forgedPayload["execution_id"].(string); if !ok||forgedExec=="" { t.Fatal("forged execution_id missing") }; forgedEvents,err:=l.Events(context.Background(),forgedExec); if err!=nil { t.Fatal(err) }; if err:=ledger.VerifyChain(forgedEvents); err!=nil { t.Fatalf("forged ledger chain verification: %v",err) }; if len(forgedEvents)!=3||forgedEvents[0].EventType!=ledger.StateAuthorized||forgedEvents[1].EventType!=ledger.StateDispatched||forgedEvents[2].EventType!=ledger.StateAborted { t.Fatalf("unexpected forged lifecycle: %+v",forgedEvents) }; var forgedActive bool; var forgedVersion int64; if err:=pg.pool.QueryRow(context.Background(),`SELECT version, active FROM users WHERE id=1843`).Scan(&forgedVersion,&forgedActive); err!=nil { t.Fatal(err) }; if forgedVersion!=43||!forgedActive { t.Fatalf("forged mutation escaped: version %d active %v",forgedVersion,forgedActive) }; t.Logf("M6 FORGERY TRAP PASS: signed forged expected_effect rejected; receipt=ABORTED; database unchanged")
}

var _ *sql.DB
var _ pgx.Identifier
