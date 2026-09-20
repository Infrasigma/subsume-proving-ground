package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/ace"
	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/t2"
)

func main() {
	var (
		ledgerPath = flag.String("ledger", "state/t2/admissions.db", "SQLite F0 admission ledger")
		libraryPath = flag.String("library", "state/t2/abstractions.json", "durable acquired-abstraction library")
		taskDir = flag.String("task-dir", "state/t2/tasks", "directory-backed reactor queue")
		poll = flag.Duration("poll", time.Second, "task queue polling interval")
		maxTasks = flag.Int("max-tasks", 0, "stop after this many tasks; zero means continuously run")
		seed = flag.Bool("seed-defaults", false, "write the two default frontier tasks when missing")
	)
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*ledgerPath), 0700); err != nil {
		log.Fatal(err)
	}
	if *seed {
		if err := seedDefaultTasks(*taskDir); err != nil {
			log.Fatal(err)
		}
	}

	signerID := os.Getenv("T2_KMS_SIGNER_ID")
	trustedPublicKey := os.Getenv("T2_TRUSTED_KMS_PUBLIC_KEY_B64")
	if signerID == "" || trustedPublicKey == "" {
		log.Fatal("T2_KMS_SIGNER_ID and T2_TRUSTED_KMS_PUBLIC_KEY_B64 are required")
	}
	kms, err := t2.NewRemoteKMSFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	store, err := ledger.Open(*ledgerPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	persistent, err := ace.NewPersistentAbstractionLibrary(*libraryPath)
	if err != nil {
		log.Fatal(err)
	}
	queue, err := ace.NewFileReactorTaskQueue(*taskDir, *poll)
	if err != nil {
		log.Fatal(err)
	}

	var substrateEscape *ace.AxonSubstrateController
	if encoded := os.Getenv("T6_SWARM_ROOT_KEY_B64"); encoded != "" {
		rootKey, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			log.Fatalf("decode T6_SWARM_ROOT_KEY_B64: %v", decodeErr)
		}
		substrateEscape = ace.DefaultAxonSubstrateController(rootKey)
	}
	runtime := &ace.AdaptiveAcquisitionRuntime{
		Abstractions:            ace.AbstractionLibrary{},
		PersistentAbstractions:  persistent,
		AbstractionKMS:          kms,
		AdmissionLedger:         store,
		KMSSignerID:             signerID,
		TrustedKMSPublicKeyB64:  trustedPublicKey,
	}
	reactor := &ace.ContinuousReactor{
		Runtime:             runtime,
		Queue:               queue,
		Admissions:          store,
		PersistentLibrary:   persistent,
		TrustedSignerID:     signerID,
		TrustedPublicKeyB64: trustedPublicKey,
		Verifier:            ace.DefaultT3DomainEscapeVerifier(),
		MaxTasks:            *maxTasks,
		AutotelicGenerator:  ace.DefaultAutotelicTaskGenerator(),
		MaxAutotelicTasks:   1,
		SubstrateEscape:      substrateEscape,
		MaxSubstrateEscapes:  1,
		Logf:                log.Printf,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	results, err := reactor.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}
	failed := 0
	for _, result := range results {
		if !result.Solved {
			failed++
		}
	}
	log.Printf("T2_T3_REACTOR_EXIT processed=%d failed=%d continuous=%t", len(results), failed, *maxTasks == 0)
}

func seedDefaultTasks(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tasks := append([]ace.ReactorTask{}, ace.DefaultT2ReactorTasks()...)
	tasks = append(tasks, ace.DefaultT4MetacognitiveTasks()...)
	for i, task := range tasks {
		path := filepath.Join(dir, fmt.Sprintf("%02d-%s.json", i+1, task.ID))
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		payload, err := json.MarshalIndent(task, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, append(payload, '\n'), 0600); err != nil {
			return err
		}
	}
	return nil
}
