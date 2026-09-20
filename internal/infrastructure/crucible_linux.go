//go:build linux

package infrastructure

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

type ResourceHandle struct {
	ResourceID string
	PID        int
	process    *os.Process
}

type ObservedResource struct {
	ResourceID   string
	PID          int
	ResourceType string
	State        string
	Command      string
	RSSBytes     uint64
	ObservedAt   time.Time
	StateHash    string
}

type InfrastructureReceipt struct {
	ProvisionExecutionID string
	ProvisionEventHashes []string
	ResourceID           string
	ObservedHash         string
	CommitEventHash      string
	ReclaimExecutionID   string
	ReclaimEventHashes   []string
	ReclamationVerified  bool
}

type CloudProviderAdapter interface {
	Provision(context.Context, protocol.InfrastructureContract) (ResourceHandle, error)
	Reclaim(context.Context, ResourceHandle) error
}

type IndependentInfrastructureAttestor interface {
	Observe(context.Context, protocol.InfrastructureContract) (ObservedResource, error)
	Verify(context.Context, protocol.InfrastructureContract, ObservedResource) error
	VerifyAbsent(context.Context, protocol.InfrastructureContract) (string, error)
}

type LocalSubprocessProvider struct{}

func (LocalSubprocessProvider) Provision(ctx context.Context, c protocol.InfrastructureContract) (ResourceHandle, error) {
	if err := c.Validate(); err != nil {
		return ResourceHandle{}, err
	}
	if c.Provider != "local-subprocess" || c.Operation != "provision" || c.MaxInstances != 1 {
		return ResourceHandle{}, errors.New("local subprocess provider requires a single provision instance")
	}
	seconds := c.MaxRuntimeSeconds
	if seconds > 30 {
		seconds = 30
	}
	cmd := exec.CommandContext(ctx, "sleep", strconv.FormatInt(seconds, 10))
	cmd.Env = append(os.Environ(), "ACE_T10_RESOURCE_ID="+c.ExpectedEffect.ResourceID)
	if err := cmd.Start(); err != nil {
		return ResourceHandle{}, fmt.Errorf("start local resource: %w", err)
	}
	return ResourceHandle{ResourceID: c.ExpectedEffect.ResourceID, PID: cmd.Process.Pid, process: cmd.Process}, nil
}

func (LocalSubprocessProvider) Reclaim(ctx context.Context, h ResourceHandle) error {
	if h.PID <= 0 {
		return errors.New("invalid resource pid")
	}
	p := h.process
	if p == nil {
		var err error
		p, err = os.FindProcess(h.PID)
		if err != nil {
			return err
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = p.Kill()
	waitDone := make(chan struct {
		state *os.ProcessState
		err   error
	}, 1)
	go func() {
		state, err := p.Wait()
		waitDone <- struct {
			state *os.ProcessState
			err   error
		}{state: state, err: err}
	}()
	select {
	case result := <-waitDone:
		if result.err == nil || errors.Is(result.err, os.ErrProcessDone) {
			return nil
		}
		if _, ok := result.err.(*exec.ExitError); ok {
			if result.state == nil {
				return result.err
			}
			if result.state.ExitCode() != 0 {
				return nil
			}
			return nil
		}
		return result.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type ProcfsInfrastructureAttestor struct{}

func (ProcfsInfrastructureAttestor) Observe(ctx context.Context, c protocol.InfrastructureContract) (ObservedResource, error) {
	if err := c.Validate(); err != nil {
		return ObservedResource{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pid, err := findResourcePID(c.ExpectedEffect.ResourceID)
	if err != nil {
		return ObservedResource{}, err
	}
	if err := ctx.Err(); err != nil {
		return ObservedResource{}, err
	}
	command, err := readCmdline(pid)
	if err != nil {
		return ObservedResource{}, err
	}
	rss, err := readRSS(pid)
	if err != nil {
		return ObservedResource{}, err
	}
	observed := ObservedResource{
		ResourceID: c.ExpectedEffect.ResourceID,
		PID: pid,
		ResourceType: "local-process",
		State: "running",
		Command: command,
		RSSBytes: rss,
		ObservedAt: time.Now().UTC(),
	}
	hash, err := hashObserved(observed)
	if err != nil {
		return ObservedResource{}, err
	}
	observed.StateHash = hash
	return observed, nil
}

func (ProcfsInfrastructureAttestor) Verify(ctx context.Context, c protocol.InfrastructureContract, observed ObservedResource) error {
	if c.Operation != "provision" {
		return errors.New("provision verification requires provision contract")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if observed.ResourceID != c.ExpectedEffect.ResourceID ||
		observed.ResourceType != c.ResourceType ||
		observed.State != "running" ||
		observed.PID <= 0 ||
		observed.StateHash == "" {
		return errors.New("independent observation does not satisfy expected infrastructure effect")
	}
	if !strings.Contains(observed.Command, "sleep") {
		return fmt.Errorf("observed process command %q is outside the local provider contract", observed.Command)
	}
	if observed.RSSBytes > uint64(c.MaxMemoryBytes) {
		return fmt.Errorf("observed RSS %d exceeds contract memory limit %d", observed.RSSBytes, c.MaxMemoryBytes)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	repeat, err := (ProcfsInfrastructureAttestor{}).Observe(ctx, c)
	if err != nil {
		return err
	}
	if repeat.PID != observed.PID || repeat.StateHash != observed.StateHash {
		return errors.New("independent read-back was not stable")
	}
	return nil
}

func (ProcfsInfrastructureAttestor) VerifyAbsent(ctx context.Context, c protocol.InfrastructureContract) (string, error) {
	if c.Operation != "reclaim" {
		return "", errors.New("reclamation verification requires reclaim contract")
	}
	if err := c.Validate(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	pid, err := findResourcePID(c.ExpectedEffect.ResourceID)
	if err == nil && pid > 0 {
		return "", fmt.Errorf("resource %q still exists as pid %d", c.ExpectedEffect.ResourceID, pid)
	}
	if !errors.Is(err, os.ErrNotExist) && err != nil {
		return "", err
	}
	payload := map[string]any{
		"resource_id": c.ExpectedEffect.ResourceID,
		"state":       "absent",
		"observed_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return "", err
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		return "", err
	}
	return protocolHex(canonical), nil
}

func GenerateInfrastructureContract(now time.Time) (protocol.InfrastructureContract, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id := protocolHash([]any{"t10-local-crucible-v1", now.UnixNano()})
	c := protocol.InfrastructureContract{
		ContractVersion:   "ace-t10/v1",
		ContractID:        id,
		Operation:         "provision",
		Provider:          "local-subprocess",
		ResourceType:      "local-process",
		MaxInstances:      1,
		MaxRuntimeSeconds: 15,
		MaxCostCPUTimeMS:  5000,
		MaxMemoryBytes:    16 * 1024 * 1024,
		ExpectedEffect: protocol.InfrastructureEffect{
			ResourceType: "local-process",
			ResourceID:   id,
			State:        "running",
			Fields: map[string]any{
				"command": "sleep",
			},
		},
		PolicyHash: policyHash(),
		IssuedAt:   now.UTC(),
		ExpiresAt:  now.UTC().Add(2 * time.Minute),
		Nonce:      protocolHash([]any{"t10-nonce", id}),
	}
	if err := c.Validate(); err != nil {
		return protocol.InfrastructureContract{}, err
	}
	return c, nil
}

func RunLocalCrucible(ctx context.Context, l *ledger.Ledger, provider CloudProviderAdapter, attestor IndependentInfrastructureAttestor, signerID string, privateKey ed25519.PrivateKey) (InfrastructureReceipt, error) {
	if l == nil || provider == nil || attestor == nil || signerID == "" || len(privateKey) != ed25519.PrivateKeySize {
		return InfrastructureReceipt{}, errors.New("T10 crucible requires ledger, provider, attestor, and signer")
	}
	contract, err := GenerateInfrastructureContract(time.Now().UTC())
	if err != nil {
		return InfrastructureReceipt{}, err
	}
	return runProvisionAndReclaim(ctx, l, provider, attestor, contract, signerID, privateKey)
}

func runProvisionAndReclaim(ctx context.Context, l *ledger.Ledger, provider CloudProviderAdapter, attestor IndependentInfrastructureAttestor, contract protocol.InfrastructureContract, signerID string, privateKey ed25519.PrivateKey) (InfrastructureReceipt, error) {
	capabilityPayload := map[string]any{
		"capability_type": "InfrastructureContract",
		"contract":        contract,
	}
	capability, err := protocol.SignPayload("Capability", capabilityPayload, signerID, privateKey)
	if err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := verifyCapability(capability, signerID, privateKey.Public().(ed25519.PublicKey)); err != nil {
		return InfrastructureReceipt{}, err
	}

	executionID := contract.ContractID
	if err := l.AppendAuthorized(ctx, executionID, capability); err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := l.AppendDispatched(ctx, executionID, contract.Nonce, contract.ExpiresAt.Format(time.RFC3339Nano)); err != nil {
		return InfrastructureReceipt{}, err
	}

	handle, err := provider.Provision(ctx, contract)
	if err != nil {
		_ = l.AppendTerminal(ctx, executionID, ledger.StateAborted, map[string]any{"error": err.Error()}, "provider provision failed")
		return InfrastructureReceipt{}, err
	}

	observed, err := attestor.Observe(ctx, contract)
	if err != nil {
		_ = l.AppendTerminal(ctx, executionID, ledger.StateIndeterminate, map[string]any{"error": err.Error()}, "independent observation failed")
		_ = provider.Reclaim(context.Background(), handle)
		return InfrastructureReceipt{}, err
	}
	if err := attestor.Verify(ctx, contract, observed); err != nil {
		_ = l.AppendTerminal(ctx, executionID, ledger.StateIndeterminate, map[string]any{"observed": observed, "error": err.Error()}, "independent verification failed")
		_ = provider.Reclaim(context.Background(), handle)
		return InfrastructureReceipt{}, err
	}
	if _, err := l.Append(ctx, eventID(), executionID, ledger.StateEffectObserved, map[string]any{
		"resource_id": observed.ResourceID,
		"pid":         observed.PID,
		"state_hash":  observed.StateHash,
	}); err != nil {
		_ = provider.Reclaim(context.Background(), handle)
		return InfrastructureReceipt{}, err
	}
	if _, err := l.Append(ctx, eventID(), executionID, ledger.StateVerified, map[string]any{
		"verification":       "independent-procfs-readback",
		"observed_state_hash": observed.StateHash,
	}); err != nil {
		_ = provider.Reclaim(context.Background(), handle)
		return InfrastructureReceipt{}, err
	}
	if err := l.AppendTerminal(ctx, executionID, ledger.StateCommitted, map[string]any{
		"resource_id":         observed.ResourceID,
		"observed_state_hash": observed.StateHash,
		"provider_claimed_pid": handle.PID,
	}, "verified infrastructure resource provisioned"); err != nil {
		_ = provider.Reclaim(context.Background(), handle)
		return InfrastructureReceipt{}, err
	}

	reclaim, err := contract.ReclamationContract()
	if err != nil {
		return InfrastructureReceipt{}, err
	}
	reclaimCapability, err := protocol.SignPayload("Capability", map[string]any{
		"capability_type":   "InfrastructureContract",
		"contract":          reclaim,
		"parent_execution":  executionID,
	}, signerID, privateKey)
	if err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := verifyCapability(reclaimCapability, signerID, privateKey.Public().(ed25519.PublicKey)); err != nil {
		return InfrastructureReceipt{}, err
	}

	reclaimID := reclaim.ContractID
	if err := l.AppendAuthorized(ctx, reclaimID, reclaimCapability); err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := l.AppendDispatched(ctx, reclaimID, reclaim.Nonce, reclaim.ExpiresAt.Format(time.RFC3339Nano)); err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := provider.Reclaim(ctx, handle); err != nil {
		_ = l.AppendTerminal(ctx, reclaimID, ledger.StateAborted, map[string]any{"error": err.Error()}, "resource reclamation failed")
		return InfrastructureReceipt{}, err
	}
	reclaimedHash, err := attestor.VerifyAbsent(ctx, reclaim)
	if err != nil {
		_ = l.AppendTerminal(ctx, reclaimID, ledger.StateIndeterminate, map[string]any{"error": err.Error()}, "independent reclamation verification failed")
		return InfrastructureReceipt{}, err
	}
	if _, err := l.Append(ctx, eventID(), reclaimID, ledger.StateEffectObserved, map[string]any{
		"resource_id": reclaim.ExpectedEffect.ResourceID,
		"state_hash":  reclaimedHash,
		"state":       "absent",
	}); err != nil {
		return InfrastructureReceipt{}, err
	}
	if _, err := l.Append(ctx, eventID(), reclaimID, ledger.StateVerified, map[string]any{
		"verification":             "independent-procfs-absence-readback",
		"reclamation_state_hash": reclaimedHash,
	}); err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := l.AppendTerminal(ctx, reclaimID, ledger.StateCommitted, map[string]any{
		"resource_id":             reclaim.ExpectedEffect.ResourceID,
		"reclamation_state_hash": reclaimedHash,
	}, "verified infrastructure resource reclaimed"); err != nil {
		return InfrastructureReceipt{}, err
	}

	provisionEvents, err := l.Events(ctx, executionID)
	if err != nil {
		return InfrastructureReceipt{}, err
	}
	reclaimEvents, err := l.Events(ctx, reclaimID)
	if err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := ledger.VerifyChain(provisionEvents); err != nil {
		return InfrastructureReceipt{}, err
	}
	if err := ledger.VerifyChain(reclaimEvents); err != nil {
		return InfrastructureReceipt{}, err
	}
	return InfrastructureReceipt{
		ProvisionExecutionID: executionID,
		ProvisionEventHashes: eventHashes(provisionEvents),
		ResourceID:           observed.ResourceID,
		ObservedHash:         observed.StateHash,
		CommitEventHash:      provisionEvents[len(provisionEvents)-1].EventHash,
		ReclaimExecutionID:   reclaimID,
		ReclaimEventHashes:   eventHashes(reclaimEvents),
		ReclamationVerified:  true,
	}, nil
}

func verifyCapability(env protocol.Envelope, signerID string, publicKey ed25519.PublicKey) error {
	if env.Type != "Capability" || env.SignerID != signerID {
		return errors.New("invalid infrastructure capability envelope")
	}
	payload, err := protocol.PayloadValue(env)
	if err != nil {
		return err
	}
	canonical, err := c14n.Canonicalize(payload)
	if err != nil {
		return err
	}
	domain, err := protocol.DomainForType(env.Type)
	if err != nil {
		return err
	}
	publicHex := fmt.Sprintf("%x", []byte(publicKey))
	if err := protocol.Verify(publicHex, domain, env, canonical); err != nil {
		return err
	}
	return nil
}

func findResourcePID(resourceID string) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	var matches []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		environ, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "environ"))
		if err != nil {
			continue
		}
		want := "ACE_T10_RESOURCE_ID=" + resourceID
		for _, item := range bytes.Split(environ, []byte{0}) {
			if string(item) == want {
				matches = append(matches, pid)
				break
			}
		}
	}
	sort.Ints(matches)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return 0, os.ErrNotExist
	default:
		return 0, fmt.Errorf("multiple processes claim infrastructure resource %q: %v", resourceID, matches)
	}
}

func processExists(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return err == nil
}

func readCmdline(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.ReplaceAll(string(bytes.TrimRight(data, "\x00")), "\x00", " ")), nil
}

func readRSS(pid int) (uint64, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "statm"))
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, errors.New("invalid /proc statm")
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return pages * uint64(os.Getpagesize()), nil
}

func hashObserved(v ObservedResource) (string, error) {
	payload := struct {
		ResourceID   string `json:"resource_id"`
		PID          int    `json:"pid"`
		ResourceType string `json:"resource_type"`
		State        string `json:"state"`
		Command      string `json:"command"`
		RSSBytes     uint64 `json:"rss_bytes"`
	}{
		ResourceID: v.ResourceID,
		PID: v.PID,
		ResourceType: v.ResourceType,
		State: v.State,
		Command: v.Command,
		RSSBytes: v.RSSBytes,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return "", err
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		return "", err
	}
	return protocolHex(canonical), nil
}

func protocolHash(v any) string {
	b, _ := json.Marshal(v)
	sum := protocol.PayloadHash(b)
	return fmt.Sprintf("%x", sum[:])
}

func policyHash() string {
	return protocolHash([]string{
		"t10-local-crucible",
		"single-instance",
		"bounded-runtime",
		"procfs-independent-readback",
	})
}

func protocolHex(b []byte) string {
	sum := protocol.PayloadHash(b)
	return fmt.Sprintf("%x", sum[:])
}

func eventID() string {
	now := time.Now().UTC().UnixNano()
	return protocolHash([]any{"t10-event", now, os.Getpid()})
}

func eventHashes(events []ledger.Event) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.EventHash)
	}
	return out
}
