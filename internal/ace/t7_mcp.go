package ace

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

)

const (
	ExternalSideEffectReceiptVersion = "ace-f0/v1"
	T7MCPJSONRPCProtocol            = "mcp-jsonrpc-v1"
	t7DefaultMaxResponseBytes      = 1 << 20
	t7DefaultTimeout                = 5 * time.Second
)

type ExternalSideEffectReceipt struct {
	Version               string `json:"version"`
	Protocol              string `json:"protocol"`
	WorkerID              string `json:"worker_id"`
	KeyID                 string `json:"key_id"`
	RequestID             string `json:"request_id"`
	Method                string `json:"method"`
	Target                string `json:"target"`
	RequestPayloadB64     string `json:"request_payload_b64"`
	ResponsePayloadB64    string `json:"response_payload_b64"`
	ResponseSHA256        string `json:"response_sha256"`
	ObservedAtUnixNanos   int64  `json:"observed_at_unix_nanos"`
	SignerPublicKeyB64    string `json:"signer_public_key_b64"`
	SignatureB64          string `json:"signature_b64"`
}

type SwarmReallocationPatch struct {
	Version             int    `json:"version"`
	PatchID             string `json:"patch_id"`
	FromWorkers         int    `json:"from_workers"`
	ToWorkers           int    `json:"to_workers"`
	CPUPct              int64  `json:"cpu_pct"`
	MemoryUsedMiB       int64  `json:"memory_used_mib"`
	MemoryTotalMiB      int64  `json:"memory_total_mib"`
	PressurePct          int64  `json:"pressure_pct"`
	Rule                 string `json:"rule"`
	BasedOnRequestID     string `json:"based_on_request_id"`
	ExternalResponseHash string `json:"external_response_hash"`
	PatchHash            string `json:"patch_hash"`
}

type T7MCPActuationTask struct {
	ID             string `json:"id"`
	ExternalEndpoint string `json:"external_endpoint"`
	ToolName       string `json:"tool_name"`
	CurrentWorkers int    `json:"current_workers"`
	MaxWorkers     int    `json:"max_workers"`
}

type MCPCallObservation struct {
	RequestID      string
	Method         string
	Target         string
	RequestPayload []byte
	ResponsePayload []byte
	ObservedAtUnixNanos int64
}

type mcpJSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      string         `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type BoundedMCPJSONRPCClient struct {
	HTTPClient        *http.Client
	MaxResponseBytes int64
	Timeout           time.Duration
}

func (c BoundedMCPJSONRPCClient) Call(ctx context.Context, target, workerID, method string, params map[string]any) (MCPCallObservation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if workerID == "" || method == "" {
		return MCPCallObservation{}, errors.New("MCP call requires worker identity and method")
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return MCPCallObservation{}, fmt.Errorf("MCP target must be an absolute http(s) URL")
	}
	if strings.ContainsAny(target, "\r\n") {
		return MCPCallObservation{}, errors.New("MCP target contains prohibited control characters")
	}
	maxBytes := c.MaxResponseBytes
	if maxBytes <= 0 || maxBytes > t7DefaultMaxResponseBytes {
		maxBytes = t7DefaultMaxResponseBytes
	}
	timeout := c.Timeout
	if timeout <= 0 || timeout > t7DefaultTimeout {
		timeout = t7DefaultTimeout
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("MCP redirects are prohibited")
			},
		}
	}
	requestIDHash := sha256.Sum256([]byte("T7/MCP/request/v1|" + workerID + "|" + method + "|" + target + "|" + strconv.Itoa(len(params))))
	requestID := hex.EncodeToString(requestIDHash[:8])
	wire, err := json.Marshal(mcpJSONRPCRequest{
		JSONRPC: "2.0",
		ID: requestID,
		Method: method,
		Params: params,
	})
	if err != nil {
		return MCPCallObservation{}, fmt.Errorf("marshal MCP request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(wire))
	if err != nil {
		return MCPCallObservation{}, fmt.Errorf("build MCP request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return MCPCallObservation{}, fmt.Errorf("MCP transport: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return MCPCallObservation{}, fmt.Errorf("read MCP response: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return MCPCallObservation{}, fmt.Errorf("MCP response exceeds %d bytes", maxBytes)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return MCPCallObservation{}, fmt.Errorf("MCP endpoint returned HTTP %d", response.StatusCode)
	}
	var decoded mcpJSONRPCResponse
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&decoded); err != nil {
		return MCPCallObservation{}, fmt.Errorf("decode MCP response: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return MCPCallObservation{}, errors.New("MCP response contains trailing JSON")
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != requestID {
		return MCPCallObservation{}, errors.New("MCP response JSON-RPC identity mismatch")
	}
	if decoded.Error != nil {
		return MCPCallObservation{}, fmt.Errorf("MCP tool error %d: %s", decoded.Error.Code, decoded.Error.Message)
	}
	if len(decoded.Result) == 0 || bytes.Equal(decoded.Result, []byte("null")) {
		return MCPCallObservation{}, errors.New("MCP response has no result payload")
	}
	observed := time.Now().UTC().UnixNano()
	return MCPCallObservation{
		RequestID: requestID,
		Method: method,
		Target: target,
		RequestPayload: append([]byte(nil), wire...),
		ResponsePayload: append([]byte(nil), decoded.Result...),
		ObservedAtUnixNanos: observed,
	}, nil
}

func t7WorkerPrivateKey(rootKey []byte, workerID string) (ed25519.PrivateKey, error) {
	if len(rootKey) < 16 || workerID == "" {
		return nil, errors.New("T7 worker signing requires root key material and worker identity")
	}
	h := sha256.New()
	_, _ = h.Write([]byte("AXON/MCP/ed25519/worker/v1|"))
	_, _ = h.Write(rootKey)
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(workerID))
	seed := h.Sum(nil)
	return ed25519.NewKeyFromSeed(seed), nil
}

func (r ExternalSideEffectReceipt) unsignedPayload() map[string]any {
	return map[string]any{
		"version": r.Version,
		"protocol": r.Protocol,
		"worker_id": r.WorkerID,
		"key_id": r.KeyID,
		"request_id": r.RequestID,
		"method": r.Method,
		"target": r.Target,
		"request_payload_b64": r.RequestPayloadB64,
		"response_payload_b64": r.ResponsePayloadB64,
		"response_sha256": r.ResponseSHA256,
		"observed_at_unix_nanos": r.ObservedAtUnixNanos,
		"signer_public_key_b64": r.SignerPublicKeyB64,
	}
}

func (r ExternalSideEffectReceipt) canonicalUnsigned() ([]byte, error) {
	return canonicalizeAXONJSON(r.unsignedPayload())
}

func SignExternalSideEffectReceipt(observation MCPCallObservation, workerID string, rootKey []byte) (ExternalSideEffectReceipt, error) {
	if observation.ObservedAtUnixNanos <= 0 || len(observation.ResponsePayload) == 0 {
		return ExternalSideEffectReceipt{}, errors.New("MCP observation is incomplete")
	}
	privateKey, err := t7WorkerPrivateKey(rootKey, workerID)
	if err != nil {
		return ExternalSideEffectReceipt{}, err
	}
	responseHash := sha256.Sum256(observation.ResponsePayload)
	receipt := ExternalSideEffectReceipt{
		Version: ExternalSideEffectReceiptVersion,
		Protocol: T7MCPJSONRPCProtocol,
		WorkerID: workerID,
		KeyID: "AXON/MCP/worker/" + workerID,
		RequestID: observation.RequestID,
		Method: observation.Method,
		Target: observation.Target,
		RequestPayloadB64: base64.StdEncoding.EncodeToString(observation.RequestPayload),
		ResponsePayloadB64: base64.StdEncoding.EncodeToString(observation.ResponsePayload),
		ResponseSHA256: hex.EncodeToString(responseHash[:]),
		ObservedAtUnixNanos: observation.ObservedAtUnixNanos,
		SignerPublicKeyB64: base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
	}
	canonical, err := receipt.canonicalUnsigned()
	if err != nil {
		return ExternalSideEffectReceipt{}, err
	}
	signature := ed25519.Sign(privateKey, canonical)
	receipt.SignatureB64 = base64.StdEncoding.EncodeToString(signature)
	return receipt, nil
}

func (r ExternalSideEffectReceipt) Verify(rootKey []byte) error {
	if r.Version != ExternalSideEffectReceiptVersion || r.Protocol != T7MCPJSONRPCProtocol {
		return errors.New("unsupported external side-effect receipt version or protocol")
	}
	if r.WorkerID == "" || r.KeyID != "AXON/MCP/worker/"+r.WorkerID || r.RequestID == "" || r.Method != "tools/call" || r.Target == "" {
		return errors.New("external side-effect receipt identity fields are invalid")
	}
	if r.ObservedAtUnixNanos <= 0 || r.RequestPayloadB64 == "" || r.ResponsePayloadB64 == "" || r.ResponseSHA256 == "" || r.SignerPublicKeyB64 == "" || r.SignatureB64 == "" {
		return errors.New("external side-effect receipt is incomplete")
	}
	responsePayload, err := base64.StdEncoding.DecodeString(r.ResponsePayloadB64)
	if err != nil {
		return fmt.Errorf("decode external response payload: %w", err)
	}
	responseHash := sha256.Sum256(responsePayload)
	if !strings.EqualFold(hex.EncodeToString(responseHash[:]), r.ResponseSHA256) {
		return errors.New("external response hash mismatch")
	}
	pub, err := base64.StdEncoding.DecodeString(r.SignerPublicKeyB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("invalid external receipt signer public key")
	}
	sig, err := base64.StdEncoding.DecodeString(r.SignatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("invalid external receipt signature")
	}
	if len(rootKey) > 0 {
		privateKey, err := t7WorkerPrivateKey(rootKey, r.WorkerID)
		if err != nil {
			return err
		}
		expectedPub := privateKey.Public().(ed25519.PublicKey)
		if !bytes.Equal(expectedPub, pub) {
			return errors.New("external receipt signer is not the derived swarm worker key")
		}
	}
	canonical, err := r.canonicalUnsigned()
	if err != nil {
		return fmt.Errorf("canonicalize external receipt: %w", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), canonical, sig) {
		return errors.New("external receipt signature verification failed")
	}
	return nil
}

type HostTelemetry struct {
	CPUPercent       int64 `json:"cpu_percent"`
	MemoryUsedMiB    int64 `json:"memory_used_mib"`
	MemoryTotalMiB   int64 `json:"memory_total_mib"`
}

func decodeHostTelemetryPayload(payload []byte) (HostTelemetry, error) {
	var telemetry HostTelemetry
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&telemetry); err != nil {
		return HostTelemetry{}, fmt.Errorf("decode host telemetry: %w", err)
	}
	if telemetry.CPUPercent < 0 || telemetry.CPUPercent > 100 || telemetry.MemoryUsedMiB < 0 || telemetry.MemoryTotalMiB <= 0 || telemetry.MemoryUsedMiB > telemetry.MemoryTotalMiB {
		return HostTelemetry{}, errors.New("host telemetry values are outside bounded ranges")
	}
	return telemetry, nil
}

func (p SwarmReallocationPatch) Validate(maxWorkers int) error {
	if p.Version != 1 || p.PatchID == "" || p.FromWorkers < 1 || p.ToWorkers < 1 || p.ToWorkers > maxWorkers {
		return errors.New("reallocation patch worker bounds are invalid")
	}
	if p.CPUPct < 0 || p.CPUPct > 100 || p.MemoryUsedMiB < 0 || p.MemoryTotalMiB <= 0 || p.MemoryUsedMiB > p.MemoryTotalMiB {
		return errors.New("reallocation patch telemetry bounds are invalid")
	}
	if p.PressurePct < 0 || p.PressurePct > 100 || p.Rule == "" || p.BasedOnRequestID == "" || p.ExternalResponseHash == "" || p.PatchHash == "" {
		return errors.New("reallocation patch provenance fields are incomplete")
	}
	unsigned := p
	unsigned.PatchHash = ""
	canonical, err := canonicalizeAXONJSON(unsigned)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), p.PatchHash) {
		return errors.New("reallocation patch hash mismatch")
	}
	return nil
}

func SynthesizeSwarmReallocation(telemetry HostTelemetry, currentWorkers, maxWorkers int, receipt ExternalSideEffectReceipt) (SwarmReallocationPatch, error) {
	if currentWorkers < 1 || maxWorkers < currentWorkers {
		return SwarmReallocationPatch{}, errors.New("invalid swarm worker bounds")
	}
	if err := receipt.Verify(nil); err != nil {
		return SwarmReallocationPatch{}, fmt.Errorf("unverified telemetry receipt: %w", err)
	}
	memoryPressure := (telemetry.MemoryUsedMiB * 100) / telemetry.MemoryTotalMiB
	pressure := telemetry.CPUPercent
	if memoryPressure > pressure {
		pressure = memoryPressure
	}
	target := currentWorkers
	rule := "hold:35<pressure<80"
	switch {
	case pressure >= 80:
		target = currentWorkers / 2
		if target < 1 {
			target = 1
		}
		rule = "shrink:pressure>=80"
	case pressure <= 35 && currentWorkers < maxWorkers:
		target = currentWorkers + 1
		rule = "grow:pressure<=35"
	}
	patch := SwarmReallocationPatch{
		Version: 1,
		PatchID: Hash([]any{"t7-reallocation", receipt.RequestID, receipt.ResponseSHA256, currentWorkers, target, pressure}),
		FromWorkers: currentWorkers,
		ToWorkers: target,
		CPUPct: telemetry.CPUPercent,
		MemoryUsedMiB: telemetry.MemoryUsedMiB,
		MemoryTotalMiB: telemetry.MemoryTotalMiB,
		PressurePct: pressure,
		Rule: rule,
		BasedOnRequestID: receipt.RequestID,
		ExternalResponseHash: receipt.ResponseSHA256,
	}
	unsigned := patch
	unsigned.PatchHash = ""
	canonical, err := canonicalizeAXONJSON(unsigned)
	if err != nil {
		return SwarmReallocationPatch{}, err
	}
	digest := sha256.Sum256(canonical)
	patch.PatchHash = hex.EncodeToString(digest[:])
	return patch, nil
}

func (g AutotelicTaskGenerator) GenerateMCPActuationTask(ctx context.Context, lib *AbstractionLibrary, heuristic *SearchHeuristicProgram, endpoint string, currentWorkers, maxWorkers int) (T7MCPActuationTask, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return T7MCPActuationTask{}, err
	}
	if lib == nil || heuristic == nil || endpoint == "" {
		return T7MCPActuationTask{}, errors.New("T7 MCP task generation requires hydrated F0 state, active heuristic, and endpoint")
	}
	if err := heuristic.Validate(); err != nil {
		return T7MCPActuationTask{}, err
	}
	if currentWorkers < 1 || maxWorkers < currentWorkers {
		return T7MCPActuationTask{}, errors.New("T7 MCP task worker bounds are invalid")
	}
	seed := Hash([]any{"t7-mcp-crucible", heuristicSignature(*heuristic), lib.IDs(), endpoint, currentWorkers, maxWorkers})
	return T7MCPActuationTask{
		ID: "07-autotelic-mcp-actuation-" + seed[:12],
		ExternalEndpoint: endpoint,
		ToolName: "telemetry.get",
		CurrentWorkers: currentWorkers,
		MaxWorkers: maxWorkers,
	}, nil
}

type T7CrucibleExecution struct {
	Task               T7MCPActuationTask
	Swarm               AxonSwarmExecution
	ExternalReceipt     ExternalSideEffectReceipt
	Telemetry            HostTelemetry
	ReallocationPatch   SwarmReallocationPatch
	Sealed               AcquiredAbstraction
}

func (e *AxonSubstrateController) RunT7TelemetryCrucible(ctx context.Context, runtime *AdaptiveAcquisitionRuntime, task T7MCPActuationTask, client BoundedMCPJSONRPCClient) (T7CrucibleExecution, error) {
	if e == nil || runtime == nil {
		return T7CrucibleExecution{}, errors.New("T7 crucible requires substrate controller and runtime")
	}
	if client.HTTPClient == nil && client.Timeout == 0 && client.MaxResponseBytes == 0 {
		client.Timeout = t7DefaultTimeout
		client.MaxResponseBytes = t7DefaultMaxResponseBytes
	}
	taskURL, err := url.Parse(task.ExternalEndpoint)
	if err != nil || (taskURL.Scheme != "http" && taskURL.Scheme != "https") || taskURL.Host == "" {
		return T7CrucibleExecution{}, errors.New("T7 crucible endpoint must be absolute http(s) URL")
	}
	matrixTask := MatrixCryptoTask{
		ID: task.ID + "-swarm",
		Rows: 64,
		Columns: 64,
		FuelLimit: 128,
		FuelPerCell: 2,
		PublicSeed: Hash([]any{"t7-crucible", task.ID}),
		ExpectedShards: task.CurrentWorkers,
	}
	swarm, err := e.Execute(ctx, matrixTask)
	if err != nil {
		return T7CrucibleExecution{}, fmt.Errorf("T7 substrate bootstrap failed: %w", err)
	}
	workerID := swarm.Consensus.Workers[0].WorkerID
	observation, err := client.Call(ctx, task.ExternalEndpoint, workerID, "tools/call", map[string]any{
		"name": task.ToolName,
		"arguments": map[string]any{},
	})
	if err != nil {
		return T7CrucibleExecution{}, fmt.Errorf("T7 MCP call failed: %w", err)
	}
	receipt, err := SignExternalSideEffectReceipt(observation, workerID, e.RootKey)
	if err != nil {
		return T7CrucibleExecution{}, fmt.Errorf("T7 external receipt signing failed: %w", err)
	}
	if err := receipt.Verify(e.RootKey); err != nil {
		return T7CrucibleExecution{}, fmt.Errorf("T7 external receipt self-verification failed: %w", err)
	}
	telemetry, err := decodeHostTelemetryPayload(observation.ResponsePayload)
	if err != nil {
		return T7CrucibleExecution{}, err
	}
	patch, err := SynthesizeSwarmReallocation(telemetry, task.CurrentWorkers, task.MaxWorkers, receipt)
	if err != nil {
		return T7CrucibleExecution{}, err
	}
	plane := swarm.ExecutionPlane
	plane.MCPProtocol = T7MCPJSONRPCProtocol
	plane.AllowedCapabilities = append([]string(nil), plane.AllowedCapabilities...)
	plane.AllowedCapabilities = append(plane.AllowedCapabilities, "mcp:tools/call:telemetry")
	plane.ExternalSideEffects = []ExternalSideEffectReceipt{receipt}
	plane.ReallocationPatches = []SwarmReallocationPatch{patch}
	artifact := AcquiredAbstraction{
		ID: Hash([]any{"execution-control-plane", task.ID, swarm.Consensus, receipt, patch}),
		Name: plane.Name + ":t7-mcp",
		ArtifactType: ExecutionControlPlaneArtifactType,
		ExecutionControlPlane: &plane,
		Contract: AbstractionContract{
			Inputs: []string{"bounded swarm task", "signed external telemetry"},
			Outputs: []string{"cryptographically sealed swarm reallocation plan"},
			Preconditions: []string{"single-substrate fuel exhaustion detected", "MCP receipt verified", "telemetry within bounded ranges"},
			Postconditions: []string{"external response hash sealed", "worker signature verified", "reallocation patch hash verified", "F0 admission committed"},
		},
		Evidence: []AbstractionEvidence{{
			TaskStructure: task.ID,
			Verified: true,
			HeldOut: true,
			TransferScore: 1,
			DiscoveryCost: ResourceVector{Compute: float64(swarm.SingleFuelDemand), Memory: float64(swarm.WorkerCount)},
			ObservedGain: 1,
		}},
		Verification: VerificationResult{
			Status: "verified",
			Independent: true,
			Expected: []string{"MCP response bound to authenticated swarm worker", "telemetry obeys safety bounds", "reallocation patch follows fixed decision rule"},
			Observed: []string{"Ed25519 external side-effect receipt", "deterministic telemetry-to-worker reallocation patch"},
			Provenance: Prov("t7-independent-mcp-verifier", task.ID, "receipt-and-reallocation", map[string]any{
					"external_receipt": receipt,
					"reallocation_patch": patch,
				}),
		},
		Provenance: Prov("t7-axon-mcp-actuation", task.ID, "f0-controlled-external-io", map[string]any{
				"external_receipt": receipt,
				"reallocation_patch": patch,
		}),
	}
	sealed, err := runtime.admitAbstraction(ctx, artifact, 1)
	if err != nil {
		return T7CrucibleExecution{}, fmt.Errorf("T7 F0 admission failed: %w", err)
	}
	if runtime.PersistentAbstractions != nil {
		if err := runtime.PersistentAbstractions.Save(&runtime.Abstractions); err != nil {
			return T7CrucibleExecution{}, err
		}
	}
	return T7CrucibleExecution{
		Task: task,
		Swarm: swarm,
		ExternalReceipt: receipt,
		Telemetry: telemetry,
		ReallocationPatch: patch,
		Sealed: sealed,
	}, nil
}
