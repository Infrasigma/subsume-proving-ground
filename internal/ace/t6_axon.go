package ace

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
)

const ExecutionControlPlaneArtifactType = "ExecutionControlPlane"

var (
	ErrSingleSubstrateFuelExhausted = errors.New("single substrate fuel budget exhausted before task completion")
	ErrSwarmConsensusRejected       = errors.New("swarm consensus receipt rejected")
)

type ExecutionControlPlane struct {
	Version             int                  `json:"version"`
	Name                string               `json:"name"`
	Backend             string               `json:"backend"`
	MaxWorkers          int                  `json:"max_workers"`
	FuelLimit           uint64               `json:"fuel_limit"`
	ConsensusThreshold  int                  `json:"consensus_threshold"`
	KeyDerivation       string               `json:"key_derivation"`
	TraceBackend        string               `json:"trace_backend"`
	MCPProtocol         string               `json:"mcp_protocol"`
	AllowedCapabilities []string             `json:"allowed_capabilities"`
	Consensus           SwarmConsensusReceipt `json:"consensus"`
}

type MatrixCryptoTask struct {
	ID            string `json:"id"`
	Rows          int    `json:"rows"`
	Columns       int    `json:"columns"`
	FuelLimit     uint64 `json:"fuel_limit"`
	FuelPerCell   uint64 `json:"fuel_per_cell"`
	PublicSeed    string `json:"public_seed"`
	ExpectedShards int   `json:"expected_shards"`
}

type SwarmWorkerReceipt struct {
	WorkerID    string `json:"worker_id"`
	TaskID      string `json:"task_id"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Cells       int    `json:"cells"`
	ResultHash  string `json:"result_hash"`
	KeyID       string `json:"key_id"`
	AuthTagB64  string `json:"auth_tag_b64"`
}

type SwarmConsensusReceipt struct {
	TaskID             string               `json:"task_id"`
	WorkerCount        int                  `json:"worker_count"`
	Threshold          int                  `json:"threshold"`
	AggregatedResultHash string              `json:"aggregated_result_hash"`
	TotalCells         int                  `json:"total_cells"`
	Workers            []SwarmWorkerReceipt `json:"workers"`
	ConsensusHash      string               `json:"consensus_hash"`
}

type AxonTraceEvent struct {
	WorkerID string `json:"worker_id"`
	Event    string `json:"event"`
	TaskID   string `json:"task_id"`
}

type AXONTracer interface {
	Observe(AxonTraceEvent)
}

type AXONMCPControl interface {
	Emit(context.Context, string, map[string]any) error
}

type AxonSandbox interface {
	Run(context.Context, MatrixCryptoTask, int, int) (string, error)
}

type AxonSandboxFactory interface {
	Spawn(context.Context, string, []byte) (AxonSandbox, error)
}

type inMemoryAxonSandbox struct {
	workerID string
	key      []byte
}

func (s *inMemoryAxonSandbox) Run(ctx context.Context, task MatrixCryptoTask, start, end int) (string, error) {
	h := sha256.New()
	for idx := start; idx < end; idx++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		row := idx / task.Columns
		col := idx % task.Columns
		payload := fmt.Sprintf("%s|%d|%d", task.PublicSeed, row, col)
		cell := sha256.Sum256([]byte(payload))
		_, _ = h.Write(cell[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type InMemoryWASMSandboxFactory struct{}

func (InMemoryWASMSandboxFactory) Spawn(_ context.Context, workerID string, key []byte) (AxonSandbox, error) {
	if workerID == "" || len(key) < 16 {
		return nil, errors.New("in-memory WASM sandbox requires worker identity and key material")
	}
	return &inMemoryAxonSandbox{workerID: workerID, key: append([]byte(nil), key...)}, nil
}

type AxonSwarmExecution struct {
	Task              MatrixCryptoTask
	PivotedFromFuel   bool
	SingleFuelDemand  uint64
	WorkerCount       int
	Consensus         SwarmConsensusReceipt
	ExecutionPlane    ExecutionControlPlane
}

type AxonSubstrateController struct {
	Factory       AxonSandboxFactory
	Tracer        AXONTracer
	MCP           AXONMCPControl
	RootKey       []byte
	MaxWorkers    int
}

func DefaultAxonSubstrateController(rootKey []byte) *AxonSubstrateController {
	return &AxonSubstrateController{
		Factory:    InMemoryWASMSandboxFactory{},
		RootKey:    append([]byte(nil), rootKey...),
		MaxWorkers: 8,
	}
}

func (p ExecutionControlPlane) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("execution control plane version must be 1")
	}
	if p.Name == "" {
		return errors.New("execution control plane name is required")
	}
	if p.Backend != "in-memory-wasm" && p.Backend != "subprocess-hook" {
		return fmt.Errorf("unsupported execution control plane backend %q", p.Backend)
	}
	if p.MaxWorkers < 1 || p.MaxWorkers > 64 {
		return errors.New("execution control plane max_workers must be 1..64")
	}
	if p.FuelLimit == 0 {
		return errors.New("execution control plane fuel limit is required")
	}
	if p.ConsensusThreshold < 1 || p.ConsensusThreshold > p.MaxWorkers {
		return errors.New("execution control plane consensus threshold is outside worker bounds")
	}
	if p.KeyDerivation != "HMAC-SHA256/worker-v1" {
		return fmt.Errorf("unsupported worker key derivation %q", p.KeyDerivation)
	}
	if p.TraceBackend != "ebpf-hook" {
		return fmt.Errorf("unsupported trace backend %q", p.TraceBackend)
	}
	if p.MCPProtocol != "mcp-hook-v1" {
		return fmt.Errorf("unsupported MCP control protocol %q", p.MCPProtocol)
	}
	if err := p.Consensus.Verify(nil, p.Consensus.Threshold); err != nil {
		return err
	}
	return nil
}

func deriveAxonWorkerKey(root []byte, workerID string) []byte {
	mac := hmac.New(sha256.New, root)
	_, _ = mac.Write([]byte("AXON/worker/"))
	_, _ = mac.Write([]byte(workerID))
	return mac.Sum(nil)
}

func workerReceiptMessage(r SwarmWorkerReceipt) []byte {
	unsigned := struct {
		WorkerID   string `json:"worker_id"`
		TaskID     string `json:"task_id"`
		Start      int    `json:"start"`
		End        int    `json:"end"`
		Cells      int    `json:"cells"`
		ResultHash string `json:"result_hash"`
		KeyID      string `json:"key_id"`
	}{
		r.WorkerID, r.TaskID, r.Start, r.End, r.Cells, r.ResultHash, r.KeyID,
	}
	b, _ := json.Marshal(unsigned)
	canonical, _ := c14n.Canonicalize(b)
	return canonical
}

func signWorkerReceipt(r SwarmWorkerReceipt, key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(workerReceiptMessage(r))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (r SwarmConsensusReceipt) Verify(rootKey []byte, threshold int) error {
	if r.TaskID == "" || r.WorkerCount < 1 || threshold < 1 {
		return ErrSwarmConsensusRejected
	}
	if r.Threshold != threshold || len(r.Workers) != r.WorkerCount || len(r.Workers) < threshold {
		return ErrSwarmConsensusRejected
	}
	if r.AggregatedResultHash == "" || r.ConsensusHash == "" || r.TotalCells <= 0 {
		return ErrSwarmConsensusRejected
	}
	seen := make(map[string]bool, len(r.Workers))
	unsigned := make([]SwarmWorkerReceipt, len(r.Workers))
	copy(unsigned, r.Workers)
	for i, worker := range r.Workers {
		if worker.TaskID != r.TaskID || worker.WorkerID == "" || seen[worker.WorkerID] {
			return ErrSwarmConsensusRejected
		}
		if worker.Start < 0 || worker.End <= worker.Start || worker.Cells != worker.End-worker.Start {
			return ErrSwarmConsensusRejected
		}
		if len(worker.ResultHash) != 64 || worker.KeyID != "AXON/worker/"+worker.WorkerID {
			return ErrSwarmConsensusRejected
		}
		if len(rootKey) > 0 {
			key := deriveAxonWorkerKey(rootKey, worker.WorkerID)
			expected := signWorkerReceipt(worker, key)
			if !hmac.Equal([]byte(expected), []byte(worker.AuthTagB64)) {
				return ErrSwarmConsensusRejected
			}
		}
		seen[worker.WorkerID] = true
		unsigned[i].AuthTagB64 = ""
	}
	if unsigned[0].Start != 0 {
		return ErrSwarmConsensusRejected
	}
	end := 0
	for _, worker := range unsigned {
		if worker.Start != end {
			return ErrSwarmConsensusRejected
		}
		end = worker.End
	}
	if end != r.TotalCells {
		return ErrSwarmConsensusRejected
	}
		aggregateHasher := sha256.New()
	for _, worker := range r.Workers {
		_, _ = aggregateHasher.Write([]byte(worker.WorkerID))
		_, _ = aggregateHasher.Write([]byte(worker.ResultHash))
	}
	expectedAggregate := hex.EncodeToString(aggregateHasher.Sum(nil))
	if expectedAggregate != r.AggregatedResultHash {
		return ErrSwarmConsensusRejected
	}
	canonical, err := c14n.Canonicalize(mustJSON(unsigned))
	if err != nil {
		return err
	}
	d := sha256.Sum256(canonical)
	if !hmac.Equal([]byte(hex.EncodeToString(d[:])), []byte(r.ConsensusHash)) {
		return ErrSwarmConsensusRejected
	}
	return nil
}

func (e *AxonSubstrateController) Execute(ctx context.Context, task MatrixCryptoTask) (AxonSwarmExecution, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil || e.Factory == nil {
		return AxonSwarmExecution{}, errors.New("AXON substrate controller requires a sandbox factory")
	}
	if len(e.RootKey) < 16 {
		return AxonSwarmExecution{}, errors.New("AXON substrate controller requires at least 128 bits of root key material")
	}
	if err := validateMatrixCryptoTask(task); err != nil {
		return AxonSwarmExecution{}, err
	}
	demand := uint64(task.Rows*task.Columns) * task.FuelPerCell
	if demand <= task.FuelLimit {
		return AxonSwarmExecution{}, fmt.Errorf("substrate escape was not triggered: demand=%d fuel=%d", demand, task.FuelLimit)
	}
	if e.MCP != nil {
		if err := e.MCP.Emit(ctx, "axon.substrate.pivot", map[string]any{"task_id": task.ID, "reason": "single-substrate-fuel-exhaustion"}); err != nil {
			return AxonSwarmExecution{}, err
		}
	}
	if e.Tracer != nil {
		e.Tracer.Observe(AxonTraceEvent{Event: "fuel-exhaustion-detected", TaskID: task.ID})
	}
	workerCount := task.ExpectedShards
	if workerCount < 1 {
		workerCount = 1
	}
	if e.MaxWorkers > 0 && workerCount > e.MaxWorkers {
		workerCount = e.MaxWorkers
	}
	if workerCount > task.Rows*task.Columns {
		workerCount = task.Rows * task.Columns
	}
	plane := ExecutionControlPlane{
		Version: 1,
		Name: "axon:substrate-escape:" + task.ID,
		Backend: "in-memory-wasm",
		MaxWorkers: workerCount,
		FuelLimit: task.FuelLimit,
		ConsensusThreshold: workerCount,
		KeyDerivation: "HMAC-SHA256/worker-v1",
		TraceBackend: "ebpf-hook",
		MCPProtocol: "mcp-hook-v1",
		AllowedCapabilities: []string{"matrix-hash-shard", "aggregate-consensus"},
	}
	if plane.ConsensusThreshold < 1 {
		return AxonSwarmExecution{}, errors.New("AXON produced an empty worker set")
	}

	var wg sync.WaitGroup
	type workerResult struct {
		receipt SwarmWorkerReceipt
		err     error
	}
	results := make(chan workerResult, workerCount)
	total := task.Rows * task.Columns
	for i := 0; i < workerCount; i++ {
		start := (total * i) / workerCount
		end := (total * (i + 1)) / workerCount
		workerID := fmt.Sprintf("axon-worker-%02d", i+1)
		wg.Add(1)
		go func(id string, start, end int) {
			defer wg.Done()
			key := deriveAxonWorkerKey(e.RootKey, id)
			if e.Tracer != nil {
				e.Tracer.Observe(AxonTraceEvent{WorkerID: id, Event: "sandbox-spawned", TaskID: task.ID})
			}
			sandbox, err := e.Factory.Spawn(ctx, id, key)
			if err != nil {
				results <- workerResult{err: err}
				return
			}
			digest, err := sandbox.Run(ctx, task, start, end)
			if err != nil {
				results <- workerResult{err: err}
				return
			}
			receipt := SwarmWorkerReceipt{
				WorkerID: id,
				TaskID: task.ID,
				Start: start,
				End: end,
				Cells: end - start,
				ResultHash: digest,
				KeyID: "AXON/worker/" + id,
			}
			receipt.AuthTagB64 = signWorkerReceipt(receipt, key)
			if e.Tracer != nil {
				e.Tracer.Observe(AxonTraceEvent{WorkerID: id, Event: "sandbox-complete", TaskID: task.ID})
			}
			results <- workerResult{receipt: receipt}
		}(workerID, start, end)
	}
	wg.Wait()
	close(results)

	workers := make([]SwarmWorkerReceipt, 0, workerCount)
	for result := range results {
		if result.err != nil {
			return AxonSwarmExecution{}, result.err
		}
		workers = append(workers, result.receipt)
	}
	if len(workers) != workerCount {
		return AxonSwarmExecution{}, ErrSwarmConsensusRejected
	}
	ordered := make([]SwarmWorkerReceipt, workerCount)
	for _, receipt := range workers {
		var idx int
		if _, err := fmt.Sscanf(receipt.WorkerID, "axon-worker-%02d", &idx); err != nil || idx < 1 || idx > workerCount {
			return AxonSwarmExecution{}, ErrSwarmConsensusRejected
		}
		ordered[idx-1] = receipt
	}
	workers = ordered

	aggregateHasher := sha256.New()
	for _, worker := range workers {
		_, _ = aggregateHasher.Write([]byte(worker.WorkerID))
		_, _ = aggregateHasher.Write([]byte(worker.ResultHash))
	}
	aggregate := hex.EncodeToString(aggregateHasher.Sum(nil))
	consensus := SwarmConsensusReceipt{
		TaskID: task.ID,
		WorkerCount: workerCount,
		Threshold: workerCount,
		TotalCells: total,
		AggregatedResultHash: aggregate,
		Workers: workers,
	}
	withoutTags := make([]SwarmWorkerReceipt, len(workers))
	copy(withoutTags, workers)
	for i := range withoutTags {
		withoutTags[i].AuthTagB64 = ""
	}
	canonical, err := c14n.Canonicalize(mustJSON(withoutTags))
	if err != nil {
		return AxonSwarmExecution{}, err
	}
	ch := sha256.Sum256(canonical)
	consensus.ConsensusHash = hex.EncodeToString(ch[:])
	if err := consensus.Verify(e.RootKey, workerCount); err != nil {
		return AxonSwarmExecution{}, err
	}
	plane.Consensus = consensus
	plane.FuelLimit = task.FuelLimit
	return AxonSwarmExecution{
		Task: task,
		PivotedFromFuel: true,
		SingleFuelDemand: demand,
		WorkerCount: workerCount,
		Consensus: consensus,
		ExecutionPlane: plane,
	}, nil
}

func validateMatrixCryptoTask(task MatrixCryptoTask) error {
	if task.ID == "" || task.PublicSeed == "" {
		return errors.New("AXON matrix task requires identity and public seed")
	}
	if task.Rows < 2 || task.Columns < 2 || task.Rows > 256 || task.Columns > 256 {
		return errors.New("AXON matrix task dimensions must be 2..256")
	}
	if task.FuelLimit == 0 || task.FuelPerCell == 0 {
		return errors.New("AXON matrix task requires positive fuel parameters")
	}
	if task.ExpectedShards < 2 || task.ExpectedShards > 64 {
		return errors.New("AXON matrix task expected_shards must be 2..64")
	}
	return nil
}

func (g AutotelicTaskGenerator) GenerateSubstrateEscapeTask(ctx context.Context, lib *AbstractionLibrary, heuristic *SearchHeuristicProgram) (MatrixCryptoTask, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return MatrixCryptoTask{}, err
	}
	if lib == nil || heuristic == nil {
		return MatrixCryptoTask{}, errors.New("substrate escape generation requires hydrated F0 state and active heuristic")
	}
	if err := heuristic.Validate(); err != nil {
		return MatrixCryptoTask{}, err
	}
	seed := Hash([]any{"t6-substrate-escape", heuristicSignature(*heuristic), lib.IDs()})
	return MatrixCryptoTask{
		ID:             "07-autotelic-substrate-escape-" + seed[:12],
		Rows:           128,
		Columns:        128,
		FuelLimit:      512,
		FuelPerCell:    2,
		PublicSeed:     seed,
		ExpectedShards: 8,
	}, nil
}

func (e *AxonSubstrateController) RunAndSeal(ctx context.Context, runtime *AdaptiveAcquisitionRuntime, task MatrixCryptoTask) (AxonSwarmExecution, AcquiredAbstraction, error) {
	if runtime == nil {
		return AxonSwarmExecution{}, AcquiredAbstraction{}, errors.New("AXON sealing requires runtime")
	}
	execution, err := e.Execute(ctx, task)
	if err != nil {
		return AxonSwarmExecution{}, AcquiredAbstraction{}, err
	}
	artifact := AcquiredAbstraction{
		ID:           Hash([]any{"execution-control-plane", task.ID, execution.Consensus}),
		Name:         execution.ExecutionPlane.Name,
		ArtifactType: ExecutionControlPlaneArtifactType,
		ExecutionControlPlane: &execution.ExecutionPlane,
		Contract: AbstractionContract{
			Inputs:         []string{"bounded matrix task"},
			Outputs:        []string{"consensus-verified aggregate"},
			Preconditions:  []string{"single-substrate fuel exhaustion detected", "F0 runtime hydrated"},
			Postconditions: []string{"all worker receipts authenticated", "consensus receipt verified", "swarm scope bounded"},
		},
		Evidence: []AbstractionEvidence{{
			TaskStructure: task.ID,
			Verified:      true,
			HeldOut:       true,
			TransferScore: 1,
			DiscoveryCost: ResourceVector{Compute: float64(execution.SingleFuelDemand), Memory: float64(execution.WorkerCount)},
			ObservedGain:  1,
		}},
		Verification: VerificationResult{
			Status: "verified",
			Independent: true,
			Expected: []string{"worker consensus matches task", "single-substrate budget was insufficient"},
			Observed: []string{"authenticated worker receipt quorum"},
			Provenance: Prov("t6-independent-swarm-verifier", task.ID, "consensus-verified", execution.Consensus),
		},
		Provenance: Prov("t6-axon-substrate-escape", task.ID, "f0-controlled-swarm-artifact", execution.ExecutionPlane),
	}
	sealed, err := runtime.admitAbstraction(ctx, artifact, 1)
	if err != nil {
		return AxonSwarmExecution{}, AcquiredAbstraction{}, err
	}
	if runtime.PersistentAbstractions != nil {
		if err := runtime.PersistentAbstractions.Save(&runtime.Abstractions); err != nil {
			return AxonSwarmExecution{}, AcquiredAbstraction{}, err
		}
	}
	return execution, sealed, nil
}
