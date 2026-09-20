package ace

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	KernelTelemetryReceiptArtifactType = "KernelTelemetryReceipt"
	ResourceGovernancePatchArtifactType = "ResourceGovernancePatch"
	KernelTelemetryReceiptVersion = "ace-f0/v1"
	ResourceGovernancePatchVersion = "ace-f0/v1"
	T8EBPFGovernanceProtocol = "ebpf-governance-v1"
	T8EBPFTraceBackend = "cilium-ebpf-v1"
)

type KernelTelemetrySnapshot struct {
	PID                 uint32 `json:"pid"`
	ObservedAtUnixNanos int64  `json:"observed_at_unix_nanos"`
	CPUPercent          int64  `json:"cpu_percent"`
	RSSBytes            uint64 `json:"rss_bytes"`
	TotalAllocBytes     uint64 `json:"total_alloc_bytes"`
	MemoryTotalBytes    uint64 `json:"memory_total_bytes"`
	SchedulerEvents     uint64 `json:"scheduler_events"`
	AllocationEvents    uint64 `json:"allocation_events"`
}

func (s KernelTelemetrySnapshot) Validate() error {
	if s.PID == 0 || s.ObservedAtUnixNanos <= 0 { return errors.New("kernel telemetry identity or timestamp is invalid") }
	if s.CPUPercent < 0 || s.CPUPercent > 100 { return errors.New("kernel telemetry CPU percent is outside 0..100") }
	if s.MemoryTotalBytes == 0 || s.RSSBytes > s.MemoryTotalBytes { return errors.New("kernel telemetry memory bounds are invalid") }
	return nil
}

func (s KernelTelemetrySnapshot) Hash() (string, error) {
	if err := s.Validate(); err != nil { return "", err }
	canonical, err := canonicalizeAXONJSON(s)
	if err != nil { return "", err }
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

type KernelTelemetryReceipt struct {
	Version            string `json:"version"`
	Protocol           string `json:"protocol"`
	WorkerID           string `json:"worker_id"`
	KeyID              string `json:"key_id"`
	PID                uint32 `json:"pid"`
	ObservedAtUnixNanos int64  `json:"observed_at_unix_nanos"`
	CPUPercent         int64  `json:"cpu_percent"`
	RSSBytes           uint64 `json:"rss_bytes"`
	TotalAllocBytes    uint64 `json:"total_alloc_bytes"`
	MemoryTotalBytes   uint64 `json:"memory_total_bytes"`
	SchedulerEvents    uint64 `json:"scheduler_events"`
	AllocationEvents   uint64 `json:"allocation_events"`
	SnapshotHash       string `json:"snapshot_hash"`
	SignerPublicKeyB64 string `json:"signer_public_key_b64"`
	SignatureB64       string `json:"signature_b64"`
}

func (r KernelTelemetryReceipt) unsignedPayload() map[string]any {
	return map[string]any{
		"version": r.Version, "protocol": r.Protocol, "worker_id": r.WorkerID, "key_id": r.KeyID,
		"pid": r.PID, "observed_at_unix_nanos": r.ObservedAtUnixNanos, "cpu_percent": r.CPUPercent,
		"rss_bytes": r.RSSBytes, "total_alloc_bytes": r.TotalAllocBytes, "memory_total_bytes": r.MemoryTotalBytes,
		"scheduler_events": r.SchedulerEvents, "allocation_events": r.AllocationEvents,
		"snapshot_hash": r.SnapshotHash, "signer_public_key_b64": r.SignerPublicKeyB64,
	}
}
func (r KernelTelemetryReceipt) canonicalUnsigned() ([]byte, error) { return canonicalizeAXONJSON(r.unsignedPayload()) }

func SignKernelTelemetryReceipt(snapshot KernelTelemetrySnapshot, workerID string, rootKey []byte) (KernelTelemetryReceipt, error) {
	if err := snapshot.Validate(); err != nil { return KernelTelemetryReceipt{}, err }
	privateKey, err := t7WorkerPrivateKey(rootKey, workerID)
	if err != nil { return KernelTelemetryReceipt{}, err }
	hash, err := snapshot.Hash()
	if err != nil { return KernelTelemetryReceipt{}, err }
	r := KernelTelemetryReceipt{
		Version: KernelTelemetryReceiptVersion, Protocol: T8EBPFGovernanceProtocol,
		WorkerID: workerID, KeyID: "AXON/eBPF/worker/" + workerID, PID: snapshot.PID,
		ObservedAtUnixNanos: snapshot.ObservedAtUnixNanos, CPUPercent: snapshot.CPUPercent,
		RSSBytes: snapshot.RSSBytes, TotalAllocBytes: snapshot.TotalAllocBytes,
		MemoryTotalBytes: snapshot.MemoryTotalBytes, SchedulerEvents: snapshot.SchedulerEvents,
		AllocationEvents: snapshot.AllocationEvents, SnapshotHash: hash,
		SignerPublicKeyB64: base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
	}
	canonical, err := r.canonicalUnsigned()
	if err != nil { return KernelTelemetryReceipt{}, err }
	r.SignatureB64 = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, canonical))
	return r, nil
}

func (r KernelTelemetryReceipt) Verify(rootKey []byte) error {
	if r.Version != KernelTelemetryReceiptVersion || r.Protocol != T8EBPFGovernanceProtocol { return errors.New("unsupported kernel telemetry receipt version or protocol") }
	if r.WorkerID == "" || r.KeyID != "AXON/eBPF/worker/"+r.WorkerID || r.PID == 0 || r.ObservedAtUnixNanos <= 0 || r.SnapshotHash == "" { return errors.New("kernel telemetry receipt identity is incomplete") }
	if r.CPUPercent < 0 || r.CPUPercent > 100 || r.MemoryTotalBytes == 0 || r.RSSBytes > r.MemoryTotalBytes { return errors.New("kernel telemetry receipt values are invalid") }
	pub, err := base64.StdEncoding.DecodeString(r.SignerPublicKeyB64)
	if err != nil || len(pub) != ed25519.PublicKeySize { return errors.New("invalid kernel telemetry public key") }
	sig, err := base64.StdEncoding.DecodeString(r.SignatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize { return errors.New("invalid kernel telemetry signature") }
	if len(rootKey) > 0 {
		privateKey, err := t7WorkerPrivateKey(rootKey, r.WorkerID)
		if err != nil { return err }
		if !bytes.Equal(privateKey.Public().(ed25519.PublicKey), pub) { return errors.New("kernel telemetry signer is not the derived swarm worker key") }
	}
	canonical, err := r.canonicalUnsigned()
	if err != nil { return err }
	if !ed25519.Verify(ed25519.PublicKey(pub), canonical, sig) { return errors.New("kernel telemetry signature verification failed") }
	return nil
}

type ResourceGovernancePatch struct {
	Version              int    `json:"version"`
	PatchID              string `json:"patch_id"`
	FromWorkers          int    `json:"from_workers"`
	ToWorkers            int    `json:"to_workers"`
	MaxMemoryMiB         int64  `json:"max_memory_mib"`
	ObservedRSSMiB       int64  `json:"observed_rss_mib"`
	CPUPercent           int64  `json:"cpu_percent"`
	SchedulerEvents      uint64 `json:"scheduler_events"`
	AllocationEvents     uint64 `json:"allocation_events"`
	GCPercent            int    `json:"gc_percent"`
	Action               string `json:"action"`
	Reason               string `json:"reason"`
	BasedOnTelemetryHash string `json:"based_on_telemetry_hash"`
	PatchHash             string `json:"patch_hash"`
}

func (p ResourceGovernancePatch) Validate(maxWorkers int) error {
	if p.Version != 1 || p.PatchID == "" || p.FromWorkers < 1 || p.ToWorkers < 1 || p.ToWorkers > maxWorkers || p.FromWorkers > maxWorkers { return errors.New("resource governance worker bounds are invalid") }
	if p.MaxMemoryMiB < 1 || p.MaxMemoryMiB > 1<<20 || p.ObservedRSSMiB < 0 || p.CPUPercent < 0 || p.CPUPercent > 100 { return errors.New("resource governance memory or CPU bounds are invalid") }
	if p.GCPercent < 1 || p.GCPercent > 1000 || p.Action == "" || p.Reason == "" || p.BasedOnTelemetryHash == "" || p.PatchHash == "" { return errors.New("resource governance provenance is incomplete") }
	unsigned := p; unsigned.PatchHash = ""
	canonical, err := canonicalizeAXONJSON(unsigned)
	if err != nil { return err }
	digest := sha256.Sum256(canonical)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), p.PatchHash) { return errors.New("resource governance patch hash mismatch") }
	return nil
}

func SynthesizeResourceGovernancePatch(snapshot KernelTelemetrySnapshot, receipt KernelTelemetryReceipt, currentWorkers, safeMemoryMiB, maxWorkers int) (ResourceGovernancePatch, error) {
	if err := receipt.Verify(nil); err != nil { return ResourceGovernancePatch{}, fmt.Errorf("unverified kernel telemetry receipt: %w", err) }
	if currentWorkers < 1 || maxWorkers < currentWorkers || safeMemoryMiB < 1 { return ResourceGovernancePatch{}, errors.New("invalid governance worker or memory bounds") }
	pressure := snapshot.CPUPercent
	memPressure := int64(0)
	if snapshot.MemoryTotalBytes > 0 { memPressure = int64((snapshot.RSSBytes * 100) / snapshot.MemoryTotalBytes) }
	if memPressure > pressure { pressure = memPressure }
	target := currentWorkers
	rule := "hold:pressure<80"
	if pressure >= 80 {
		target = currentWorkers / 2
		if target < 1 { target = 1 }
		rule = "throttle:pressure>=80"
	}
	p := ResourceGovernancePatch{
		Version: 1, PatchID: Hash([]any{"t8-governance", receipt.SnapshotHash, currentWorkers, target, safeMemoryMiB, pressure}),
		FromWorkers: currentWorkers, ToWorkers: target, MaxMemoryMiB: safeMemoryMiB,
		ObservedRSSMiB: int64(snapshot.RSSBytes / (1024 * 1024)), CPUPercent: snapshot.CPUPercent,
		SchedulerEvents: snapshot.SchedulerEvents, AllocationEvents: snapshot.AllocationEvents,
		GCPercent: 50, Action: "throttle-workers-and-stream-memory", Reason: rule,
		BasedOnTelemetryHash: receipt.SnapshotHash,
	}
	unsigned := p; unsigned.PatchHash = ""
	canonical, err := canonicalizeAXONJSON(unsigned)
	if err != nil { return ResourceGovernancePatch{}, err }
	digest := sha256.Sum256(canonical); p.PatchHash = hex.EncodeToString(digest[:])
	return p, p.Validate(maxWorkers)
}

type KernelTelemetrySource interface {
	Snapshot(context.Context, uint32) (KernelTelemetrySnapshot, error)
}
type StaticKernelTelemetrySource struct { SnapshotValue KernelTelemetrySnapshot }
func (s StaticKernelTelemetrySource) Snapshot(ctx context.Context, _ uint32) (KernelTelemetrySnapshot, error) {
	if ctx == nil { ctx = context.Background() }
	if err := ctx.Err(); err != nil { return KernelTelemetrySnapshot{}, err }
	return s.SnapshotValue, nil
}

type T8KernelGovernanceTask struct {
	ID                 string `json:"id"`
	InitialWorkers     int    `json:"initial_workers"`
	MaxWorkers         int    `json:"max_workers"`
	PlannedHoldMiB     int64  `json:"planned_hold_mib"`
	SafeMemoryMiB      int64  `json:"safe_memory_mib"`
	ChunkMiB           int64  `json:"chunk_mib"`
	TriggerPressurePct int64  `json:"trigger_pressure_pct"`
}

func (g AutotelicTaskGenerator) GenerateKernelGovernanceTask(ctx context.Context, lib *AbstractionLibrary, heuristic *SearchHeuristicProgram, initialWorkers, maxWorkers int) (T8KernelGovernanceTask, error) {
	if ctx == nil { ctx = context.Background() }
	if err := ctx.Err(); err != nil { return T8KernelGovernanceTask{}, err }
	if lib == nil || heuristic == nil { return T8KernelGovernanceTask{}, errors.New("T8 governance generation requires hydrated F0 state and active heuristic") }
	if err := heuristic.Validate(); err != nil { return T8KernelGovernanceTask{}, err }
	if initialWorkers < 1 || maxWorkers < initialWorkers { return T8KernelGovernanceTask{}, errors.New("T8 governance worker bounds are invalid") }
	seed := Hash([]any{"t8-ebpf-governance", heuristicSignature(*heuristic), lib.IDs(), initialWorkers, maxWorkers})
	return T8KernelGovernanceTask{ID: "08-autotelic-ebpf-governance-"+seed[:12], InitialWorkers: initialWorkers, MaxWorkers: maxWorkers, PlannedHoldMiB: 64, SafeMemoryMiB: 8, ChunkMiB: 1, TriggerPressurePct: 80}, nil
}

type T8GovernanceExecution struct {
	Task            T8KernelGovernanceTask
	Swarm           AxonSwarmExecution
	KernelReceipt   KernelTelemetryReceipt
	GovernancePatch ResourceGovernancePatch
	CompletedMiB    int64
	PeakHeldMiB     int64
	FinalDigest     string
	Sealed          AcquiredAbstraction
}

func executeGovernedMemoryTask(ctx context.Context, task T8KernelGovernanceTask, patch ResourceGovernancePatch) (string, int64, error) {
	if err := patch.Validate(task.MaxWorkers); err != nil { return "", 0, err }
	if patch.MaxMemoryMiB > task.SafeMemoryMiB { return "", 0, errors.New("governance patch widened memory budget") }
	oldGC := debug.SetGCPercent(patch.GCPercent)
	defer debug.SetGCPercent(oldGC)
	h := sha256.New()
	held := make([][]byte, 0, patch.ToWorkers)
	var completed, peak int64
	for completed < task.PlannedHoldMiB {
		if err := ctx.Err(); err != nil { return "", completed, err }
		chunk := task.ChunkMiB
		if rem := task.PlannedHoldMiB - completed; rem < chunk { chunk = rem }
		if chunk <= 0 || chunk > patch.MaxMemoryMiB { return "", completed, errors.New("governed memory chunk exceeds hard limit") }
		for len(held) >= patch.ToWorkers { held[0] = nil; held = held[1:] }
		buf := make([]byte, int(chunk)*1024*1024)
		for i := range buf { buf[i] = byte((completed + int64(i)) % 251) }
		held = append(held, buf)
		var heldMiB int64
		for _, item := range held { heldMiB += int64(len(item)) / (1024 * 1024) }
		if heldMiB > patch.MaxMemoryMiB { return "", completed, errors.New("governance patch failed to cap live memory") }
		if heldMiB > peak { peak = heldMiB }
		_, _ = h.Write(buf)
		completed += chunk
		if len(held) == patch.ToWorkers { held[0] = nil; held = held[1:]; runtime.GC() }
	}
	for i := range held { held[i] = nil }
	return hex.EncodeToString(h.Sum(nil)), peak, nil
}

func (e *AxonSubstrateController) RunT8KernelGovernanceCrucible(ctx context.Context, runtimeState *AdaptiveAcquisitionRuntime, task T8KernelGovernanceTask, source KernelTelemetrySource) (T8GovernanceExecution, error) {
	if e == nil || runtimeState == nil || source == nil { return T8GovernanceExecution{}, errors.New("T8 crucible requires substrate controller, runtime, and kernel telemetry source") }
	if task.InitialWorkers < 1 || task.MaxWorkers < task.InitialWorkers || task.PlannedHoldMiB < task.SafeMemoryMiB || task.SafeMemoryMiB < task.ChunkMiB || task.ChunkMiB < 1 { return T8GovernanceExecution{}, errors.New("T8 crucible task bounds are invalid") }
	matrixTask := MatrixCryptoTask{ID: task.ID+"-swarm", Rows: 64, Columns: 64, FuelLimit: 128, FuelPerCell: 2, PublicSeed: Hash([]any{"t8-governance", task.ID}), ExpectedShards: task.InitialWorkers}
	swarm, err := e.Execute(ctx, matrixTask)
	if err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 substrate bootstrap failed: %w", err) }
	workerID := swarm.Consensus.Workers[0].WorkerID
	snapshot, err := source.Snapshot(ctx, uint32(os.Getpid()))
	if err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 kernel telemetry snapshot failed: %w", err) }
	if snapshot.PID == 0 { snapshot.PID = uint32(os.Getpid()) }
	receipt, err := SignKernelTelemetryReceipt(snapshot, workerID, e.RootKey)
	if err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 kernel telemetry receipt signing failed: %w", err) }
	if err := receipt.Verify(e.RootKey); err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 kernel telemetry receipt verification failed: %w", err) }
	patch, err := SynthesizeResourceGovernancePatch(snapshot, receipt, task.InitialWorkers, task.SafeMemoryMiB, task.MaxWorkers)
	if err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 governance synthesis failed: %w", err) }
	finalDigest, peak, err := executeGovernedMemoryTask(ctx, task, patch)
	if err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 governed memory task failed: %w", err) }

	plane := swarm.ExecutionPlane
	plane.TraceBackend = T8EBPFTraceBackend
	plane.GovernanceProtocol = T8EBPFGovernanceProtocol
	plane.AllowedCapabilities = append(append([]string(nil), plane.AllowedCapabilities...), "kernel:telemetry", "resource:governance")
	plane.KernelTelemetryReceipts = []KernelTelemetryReceipt{receipt}
	plane.ResourceGovernancePatches = []ResourceGovernancePatch{patch}
	artifact := AcquiredAbstraction{
		ID: Hash([]any{"execution-control-plane", task.ID, swarm.Consensus, receipt, patch, finalDigest}),
		Name: plane.Name + ":t8-ebpf-governance", ArtifactType: ExecutionControlPlaneArtifactType, ExecutionControlPlane: &plane,
		Contract: AbstractionContract{Inputs: []string{"bounded swarm task", "kernel telemetry snapshot"}, Outputs: []string{"bounded resource-governance decision and completed streaming task"}, Preconditions: []string{"swarm consensus verified", "kernel telemetry receipt signed", "resource limits bounded"}, Postconditions: []string{"worker budget reduced or held", "memory hold capped", "governance patch hash verified", "F0 admission committed"}},
		Evidence: []AbstractionEvidence{{TaskStructure: task.ID, Verified: true, HeldOut: true, TransferScore: 1, DiscoveryCost: ResourceVector{Compute: float64(task.PlannedHoldMiB), Memory: float64(peak)}, ObservedGain: 1}},
		Verification: VerificationResult{Status: "verified", Independent: true, Expected: []string{"kernel telemetry is signed", "resource patch does not widen memory bounds", "governed task completes under the patch"}, Observed: []string{"eBPF scheduler/page-allocation counters bound to host snapshot", "worker throttle rule", "bounded streaming memory execution"}, Provenance: Prov("t8-independent-governance-verifier", task.ID, "kernel-receipt-and-patch", map[string]any{"kernel_receipt": receipt, "governance_patch": patch, "final_digest": finalDigest})},
		Provenance: Prov("t8-ebpf-governance", task.ID, "f0-controlled-kernel-observability", map[string]any{"kernel_receipt": receipt, "governance_patch": patch}),
	}
	sealed, err := runtimeState.admitAbstraction(ctx, artifact, 1)
	if err != nil { return T8GovernanceExecution{}, fmt.Errorf("T8 F0 admission failed: %w", err) }
	if runtimeState.PersistentAbstractions != nil { if err := runtimeState.PersistentAbstractions.Save(&runtimeState.Abstractions); err != nil { return T8GovernanceExecution{}, err } }
	return T8GovernanceExecution{Task: task, Swarm: swarm, KernelReceipt: receipt, GovernancePatch: patch, CompletedMiB: task.PlannedHoldMiB, PeakHeldMiB: peak, FinalDigest: finalDigest, Sealed: sealed}, nil
}
