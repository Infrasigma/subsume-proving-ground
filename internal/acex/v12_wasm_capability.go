package acex

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
)

const v12WasmCustomSection = "acex.capability.v12"

type V12CapabilityPayload struct {
	Version  int                              `json:"version"`
	Pattern  V11DirectedPatternArtifact       `json:"pattern"`
	IRDigest string                           `json:"ir_digest"`
}

type V12WasmCapability struct {
	Module  []byte
	SHA256  string
	Pattern V11DirectedPatternArtifact
}

func V12PatternPayload(pattern V11DirectedPatternArtifact, ir V12EffectIR) (V12CapabilityPayload, []byte, error) {
	irBytes, err := json.Marshal(ir)
	if err != nil {
		return V12CapabilityPayload{}, nil, err
	}
	h := sha256.Sum256(irBytes)
	payload := V12CapabilityPayload{
		Version:  1,
		Pattern:  pattern,
		IRDigest: fmt.Sprintf("%x", h[:]),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return V12CapabilityPayload{}, nil, err
	}
	return payload, data, nil
}

func V12PatternSignature(payload V12CapabilityPayload) uint64 {
	h := sha256.Sum256(mustJSON(payload))
	return binary.LittleEndian.Uint64(h[:8])
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func CompileV12WasmCapability(ir V12EffectIR) (V12WasmCapability, error) {
	if err := ir.Validate(); err != nil {
		return V12WasmCapability{}, err
	}
	payload, payloadBytes, err := V12PatternPayload(ir.Pattern, ir)
	if err != nil {
		return V12WasmCapability{}, err
	}
	sig := V12PatternSignature(payload)

	var module []byte
	module = append(module, 0x00, 0x61, 0x73, 0x6d) // \0asm
	module = append(module, 0x01, 0x00, 0x00, 0x00)

	// (i64) -> i32
	typeSection := []byte{0x01, 0x60, 0x01, 0x7e, 0x01, 0x7f}
	module = append(module, wasmSection(1, typeSection)...)

	// One function referring to type 0.
	module = append(module, wasmSection(3, []byte{0x01, 0x00})...)

	// export "match": func 0
	exportSection := []byte{0x01, 0x05, 'm', 'a', 't', 'c', 'h', 0x00, 0x00}
	module = append(module, wasmSection(7, exportSection)...)

	// Keep the learned symbolic payload inside a standard custom section. The
	// section is data, not a pointer into the learner's heap.
	customPayload := append(wasmULEB(uint64(len(v12WasmCustomSection))), []byte(v12WasmCustomSection)...)
	customPayload = append(customPayload, payloadBytes...)
	module = append(module, wasmSection(0, customPayload)...)

	// match(sig) -> 1 exactly when sig == the learned capability signature.
	body := []byte{0x00, 0x20, 0x00, 0x42}
	body = append(body, wasmSLEB(int64(sig))...)
	body = append(body, 0x51, 0x0b) // i64.eq; end
	codePayload := append(wasmULEB(1), wasmULEB(uint64(len(body)))...)
	codePayload = append(codePayload, body...)
	module = append(module, wasmSection(10, codePayload)...)

	h := sha256.Sum256(module)
	return V12WasmCapability{
		Module:  module,
		SHA256:  fmt.Sprintf("%x", h[:]),
		Pattern: payload.Pattern,
	}, nil
}

func LoadV12WasmCapability(module []byte) (V12WasmCapability, error) {
	payload, err := extractV12Payload(module)
	if err != nil {
		return V12WasmCapability{}, err
	}
	if payload.Version != 1 {
		return V12WasmCapability{}, fmt.Errorf("unsupported V12 artifact version %d", payload.Version)
	}
	pattern := V11DirectedExecutableRepresentation{
		Root:     payload.Pattern.Root,
		Nodes:    payload.Pattern.Nodes,
		Edges:    append([]V11DirectedPatternEdge(nil), payload.Pattern.Edges...),
		Valid:    true,
		Retained: true,
		Budget:   20000,
	}
	ir, err := BuildV12EffectIR(pattern)
	if err != nil {
		return V12WasmCapability{}, err
	}
	expected, rawIR, err := V12PatternPayload(payload.Pattern, ir)
	if err != nil {
		return V12WasmCapability{}, err
	}
	_ = rawIR
	if payload.IRDigest != expected.IRDigest {
		return V12WasmCapability{}, fmt.Errorf("effect IR digest mismatch")
	}
	if err := validateWasm(module); err != nil {
		return V12WasmCapability{}, err
	}
	cap := V12WasmCapability{Module: append([]byte(nil), module...), Pattern:payload.Pattern}
	h := sha256.Sum256(module)
	cap.SHA256 = fmt.Sprintf("%x", h[:])
	if ok, err := ExecuteV12Wasm(module, V12PatternSignature(expected)); err != nil || !ok {
		if err != nil {
			return V12WasmCapability{}, err
		}
		return V12WasmCapability{}, fmt.Errorf("artifact self-check rejected learned signature")
	}
	return cap, nil
}

func ExecuteV12Wasm(module []byte, signature uint64) (bool, error) {
	ctx := context.Background()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, module)
	if err != nil {
		return false, err
	}
	defer compiled.Close(ctx)

	instance, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return false, err
	}
	defer instance.Close(ctx)

	fn := instance.ExportedFunction("match")
	if fn == nil {
		return false, fmt.Errorf("V12 artifact lacks exported match function")
	}
	results, err := fn.Call(ctx, signature)
	if err != nil {
		return false, err
	}
	return len(results) == 1 && results[0] == 1, nil
}

func EncodeV12WasmBase64(module []byte) string {
	return base64.StdEncoding.EncodeToString(module)
}

func DecodeV12WasmBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func wasmSection(id byte, payload []byte) []byte {
	out := []byte{id}
	out = append(out, wasmULEB(uint64(len(payload)))...)
	out = append(out, payload...)
	return out
}

func wasmULEB(v uint64) []byte {
	out := make([]byte, 0, 10)
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}

func wasmSLEB(v int64) []byte {
	out := make([]byte, 0, 10)
	more := true
	for more {
		b := byte(v & 0x7f)
		sign := b&0x40 != 0
		v >>= 7
		if (v == 0 && !sign) || (v == -1 && sign) {
			more = false
		} else {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}

func readWasmULEB(data []byte, pos *int) (uint64, error) {
	var value uint64
	var shift uint
	for {
		if *pos >= len(data) || shift > 63 {
			return 0, fmt.Errorf("invalid wasm LEB")
		}
		b := data[*pos]
		*pos++
		value |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, nil
		}
		shift += 7
	}
}

func extractV12Payload(module []byte) (V12CapabilityPayload, error) {
	if len(module) < 8 || string(module[:4]) != "\x00asm" || binary.LittleEndian.Uint32(module[4:8]) != 1 {
		return V12CapabilityPayload{}, fmt.Errorf("invalid wasm header")
	}
	pos := 8
	var found *V12CapabilityPayload
	for pos < len(module) {
		id := module[pos]
		pos++
		size, err := readWasmULEB(module, &pos)
		if err != nil {
			return V12CapabilityPayload{}, err
		}
		if size > uint64(len(module)-pos) {
			return V12CapabilityPayload{}, fmt.Errorf("wasm section exceeds module")
		}
		end := pos + int(size)
		if id == 0 {
			nameLen, err := readWasmULEB(module, &pos)
			if err != nil || nameLen > uint64(end-pos) {
				return V12CapabilityPayload{}, fmt.Errorf("invalid custom section")
			}
			name := string(module[pos : pos+int(nameLen)])
			pos += int(nameLen)
			if name == v12WasmCustomSection {
				var payload V12CapabilityPayload
				if err := json.Unmarshal(module[pos:end], &payload); err != nil {
					return V12CapabilityPayload{}, err
				}
				if found != nil {
					return V12CapabilityPayload{}, fmt.Errorf("duplicate V12 capability section")
				}
				found = &payload
			}
		}
		pos = end
	}
	if found == nil {
		return V12CapabilityPayload{}, fmt.Errorf("V12 capability payload not found")
	}
	return *found, nil
}

func validateWasm(module []byte) error {
	ctx := context.Background()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer runtime.Close(ctx)
	compiled, err := runtime.CompileModule(ctx, module)
	if err != nil {
		return err
	}
	defer compiled.Close(ctx)
	return nil
}
