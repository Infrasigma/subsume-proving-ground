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

	if out, err := exec.Command("initdb", "-U", "postgres", "-D", dataDir, "--no-locale", "--encoding=UTF8", "--auth=trust").CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v\n%s", err, out)
	}
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		t.Fatalf("create postgres socket dir: %v", err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	pgOptions := fmt.Sprintf("-p %d -h 127.0.0.1 -k %s", port, socketDir)

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