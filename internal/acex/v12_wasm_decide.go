package acex

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const (
	V12DecisionABIVersion          = 1
	V12DecisionMaxNodes            = 16
	V12DecisionMaxCandidates       = 16
	V12DecisionCandidateStride     = V12DecisionMaxNodes * 4
	V12DecisionInputBytesPerNode   = 4
	V12DecisionMemoryOffset uint32 = 0
)

type V12CandidateMatrix struct {
	Rows [V12DecisionMaxNodes]uint32
}

type V12WasmDecisionExecutor struct {
	ctx      context.Context
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	module   api.Module
	decide   api.Function
	stack    [2]uint64
}

func CompileV12DecisionWasm(pattern V11DirectedPatternArtifact, metadata []byte) ([]byte, error) {
	if pattern.Version != 1 || pattern.Root != 0 || pattern.Nodes < 2 || pattern.Nodes > 4 || len(pattern.Edges) == 0 {
		return nil, fmt.Errorf("unsupported decision pattern")
	}
	for _, edge := range pattern.Edges {
		if edge.From < 0 || edge.From >= pattern.Nodes || edge.To < 0 || edge.To >= pattern.Nodes || edge.From == edge.To {
			return nil, fmt.Errorf("invalid decision pattern edge")
		}
	}

	types := []byte{
		0x02,
		0x60, 0x01, 0x7f, 0x01, 0x7f, // (i32) -> i32
		0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, // (i32,i32) -> i32
	}
	var module []byte
	module = append(module, 0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00)
	module = append(module, wasmSection(1, types)...)

	// Two functions: pattern matcher and candidate selector.
	module = append(module, wasmSection(3, []byte{0x02, 0x00, 0x01})...)

	// One linear-memory page. The host supplies raw adjacency rows here.
	module = append(module, wasmSection(5, []byte{0x01, 0x00, 0x01})...)

	// Export decide() and memory.
	exportSection := []byte{
		0x02,
		0x06, 'd', 'e', 'c', 'i', 'd', 'e', 0x00, 0x01,
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
	}
	module = append(module, wasmSection(7, exportSection)...)

	if len(metadata) > 0 {
		customPayload := append(wasmULEB(uint64(len(v12WasmCustomSection))), []byte(v12WasmCustomSection)...)
		customPayload = append(customPayload, metadata...)
		module = append(module, wasmSection(0, customPayload)...)
	}

	matcher := v12EmitPatternMatcher(pattern)
	decider := v12EmitDecisionSelector()

	codePayload := []byte{0x02}
	codePayload = append(codePayload, wasmULEB(uint64(len(matcher)))...)
	codePayload = append(codePayload, matcher...)
	codePayload = append(codePayload, wasmULEB(uint64(len(decider)))...)
	codePayload = append(codePayload, decider...)
	module = append(module, wasmSection(10, codePayload)...)
	return module, nil
}

func v12EmitPatternMatcher(pattern V11DirectedPatternArtifact) []byte {
	body := []byte{0x01, 0x03, 0x7f} // one local group: m1,m2,m3

	for i := 0; i < pattern.Nodes-1; i++ {
		body = append(body, v12I32Const(1)...)
		body = append(body, v12LocalSet(uint32(i+1))...)
	}

	body = append(body, v12EmitSearchLevel(pattern, 0)...)
	body = append(body, 0x41, 0x00, 0x0f, 0x0b) // return 0; end
	return body
}

func v12EmitSearchLevel(pattern V11DirectedPatternArtifact, depth int) []byte {
	var out []byte
	localIndex := uint32(depth + 1)
	out = append(out, 0x02, 0x40) // block $exit
	out = append(out, 0x03, 0x40) // loop $continue

	// for (localIndex = 1; localIndex < 16; ++localIndex)
	out = append(out, v12LocalGet(localIndex)...)
	out = append(out, 0x41, 0x10, 0x4f) // >= 16
	out = append(out, 0x0d, 0x01)       // br_if $exit

	if depth > 0 {
		for prev := 0; prev < depth; prev++ {
			out = append(out, v12LocalGet(localIndex)...)
			out = append(out, v12LocalGet(uint32(prev+1))...)
			out = append(out, 0x46) // eq
			// Duplicate mapping: advance this loop cursor before continuing.
			out = append(out, 0x04, 0x40) // if
			out = append(out, v12LocalGet(localIndex)...)
			out = append(out, 0x41, 0x01, 0x6a)
			out = append(out, v12LocalSet(localIndex)...)
			out = append(out, 0x0c, 0x01) // br current loop
			out = append(out, 0x0b)       // end if
		}
	}

	if depth+1 == pattern.Nodes-1 {
		out = append(out, v12EmitMappingBody(pattern)...)
	} else {
		// Nested mapping variables are mutable Wasm locals. Reset the child
		// cursor for every outer-candidate iteration.
		out = append(out, v12I32Const(1)...)
		out = append(out, v12LocalSet(uint32(depth+2))...)
		out = append(out, v12EmitSearchLevel(pattern, depth+1)...)
	}

	// Advance this loop variable and continue.
	out = append(out, v12LocalGet(localIndex)...)
	out = append(out, 0x41, 0x01, 0x6a) // add 1
	out = append(out, v12LocalSet(localIndex)...)
	out = append(out, 0x0c, 0x00) // br loop
	out = append(out, 0x0b, 0x0b) // end loop, end block
	return out
}

func v12EmitMappingBody(pattern V11DirectedPatternArtifact) []byte {
	var out []byte
	out = append(out, 0x02, 0x40) // block $nextMapping

	for _, edge := range pattern.Edges {
		out = append(out, v12EmitEdgeTest(edge)...)
		// If edge is absent, abandon this mapping only.
		out = append(out, 0x45)                     // eqz
		out = append(out, 0x04, 0x40, 0x0c, 0x01, 0x0b) // if -> br mapping block
	}

	out = append(out, 0x41, 0x01, 0x0f) // return 1
	out = append(out, 0x0b)             // end mapping block
	return out
}

func v12EmitEdgeTest(edge V11DirectedPatternEdge) []byte {
	var out []byte
	// All non-root mapping indices are bounded to 1..15 by the search loops.
	if edge.From == 0 {
		out = append(out, v12LocalGet(0)...)
	} else {
		out = append(out, v12LocalGet(uint32(edge.From))...)
		out = append(out, 0x41, 0x10, 0x4f) // >=16
		out = append(out, 0x04, 0x40, 0x0c, 0x01, 0x0b) // impossible mapping -> next mapping
		out = append(out, v12LocalGet(uint32(edge.From))...)
		out = append(out, 0x41, 0x02, 0x74)
		out = append(out, v12LocalGet(0)...)
		out = append(out, 0x6a)
	}
	out = append(out, 0x28, 0x00, 0x00)
	if edge.To == 0 {
		out = append(out, 0x41, 0x00)
	} else {
		out = append(out, v12LocalGet(uint32(edge.To))...)
		out = append(out, 0x41, 0x10, 0x4f)
		out = append(out, 0x04, 0x40, 0x0c, 0x01, 0x0b)
		out = append(out, v12LocalGet(uint32(edge.To))...)
	}
	out = append(out, 0x76, 0x41, 0x01, 0x71)
	return out
}

func v12EmitDecisionSelector() []byte {
	body := []byte{0x01, 0x02, 0x7f} // candidate, base

	body = append(body, 0x41, 0x00, 0x21, 0x02) // candidate = 0
	body = append(body, 0x02, 0x40)               // block $exit
	body = append(body, 0x03, 0x40)               // loop $next

	// if candidate >= count, exit.
	body = append(body, 0x20, 0x02, 0x20, 0x01, 0x4f)
	body = append(body, 0x0d, 0x01)

	// base = ptr + candidate*64.
	body = append(body, 0x20, 0x00, 0x20, 0x02, 0x41, 0x40, 0x6c, 0x6a, 0x21, 0x03)
	// The matcher function is function index 0.
	body = append(body, 0x20, 0x03, 0x10, 0x00)
	body = append(body, 0x04, 0x40) // if match
	body = append(body, 0x20, 0x02, 0x0f)
	body = append(body, 0x0b)

	body = append(body, 0x20, 0x02, 0x41, 0x01, 0x6a, 0x21, 0x02)
	body = append(body, 0x0c, 0x00)
	body = append(body, 0x0b, 0x0b)
	body = append(body, 0x41, 0x7f, 0x0f, 0x0b) // -1
	return body
}

func v12LocalGet(index uint32) []byte {
	return []byte{0x20, byte(index)}
}

func v12LocalSet(index uint32) []byte {
	return []byte{0x21, byte(index)}
}

func v12I32Const(v int32) []byte {
	return append([]byte{0x41}, wasmSLEB(int64(v))...)
}

func NewV12WasmDecisionExecutor(ctx context.Context, module []byte) (*V12WasmDecisionExecutor, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime := wazero.NewRuntime(ctx)
	compiled, err := runtime.CompileModule(ctx, module)
	if err != nil {
		runtime.Close(ctx)
		return nil, err
	}
	instance, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		compiled.Close(ctx)
		runtime.Close(ctx)
		return nil, err
	}
	fn := instance.ExportedFunction("decide")
	if fn == nil {
		instance.Close(ctx)
		compiled.Close(ctx)
		runtime.Close(ctx)
		return nil, fmt.Errorf("V12 artifact lacks decide export")
	}
	memory := instance.Memory()
	if memory == nil {
		instance.Close(ctx)
		compiled.Close(ctx)
		runtime.Close(ctx)
		return nil, fmt.Errorf("V12 artifact lacks exported memory")
	}
	return &V12WasmDecisionExecutor{
		ctx: ctx, runtime: runtime, compiled: compiled,
		module: instance, decide: fn,
	}, nil
}

func (e *V12WasmDecisionExecutor) Decide(input []byte, candidateCount uint32) (int32, error) {
	if e == nil || e.module == nil {
		return 0, fmt.Errorf("nil V12 executor")
	}
	if candidateCount == 0 || candidateCount > V12DecisionMaxCandidates {
		return 0, fmt.Errorf("candidate count out of bounds: %d", candidateCount)
	}
	want := int(candidateCount) * V12DecisionCandidateStride
	if len(input) != want {
		return 0, fmt.Errorf("invalid V12 decision input size: got=%d want=%d", len(input), want)
	}
	if ok := e.module.Memory().Write(V12DecisionMemoryOffset, input); !ok {
		return 0, fmt.Errorf("V12 decision input exceeds Wasm memory")
	}
	e.stack[0] = uint64(V12DecisionMemoryOffset)
	e.stack[1] = uint64(candidateCount)
	if err := e.decide.CallWithStack(e.ctx, e.stack[:]); err != nil {
		return 0, err
	}
	return int32(uint32(e.stack[0])), nil
}

func (e *V12WasmDecisionExecutor) Close(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = e.ctx
	}
	err1 := e.module.Close(ctx)
	err2 := e.compiled.Close(ctx)
	err3 := e.runtime.Close(ctx)
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return err3
}

func v12PatternCandidateMatrix(pattern V11DirectedPatternArtifact) V12CandidateMatrix {
	var candidate V12CandidateMatrix
	for _, edge := range pattern.Edges {
		candidate.Rows[edge.From] |= uint32(1) << uint32(edge.To)
	}
	return candidate
}

func EncodeV12DecisionInput(candidates []V12CandidateMatrix, buf []byte) ([]byte, error) {
	if len(candidates) == 0 || len(candidates) > V12DecisionMaxCandidates {
		return nil, fmt.Errorf("candidate count out of bounds: %d", len(candidates))
	}
	want := len(candidates) * V12DecisionCandidateStride
	if len(buf) < want {
		return nil, fmt.Errorf("buffer too small: got=%d want=%d", len(buf), want)
	}
	buf = buf[:want]
	for i := range candidates {
		base := i * V12DecisionCandidateStride
		for row := 0; row < V12DecisionMaxNodes; row++ {
			binary.LittleEndian.PutUint32(buf[base+row*4:], candidates[i].Rows[row])
		}
	}
	return buf, nil
}
