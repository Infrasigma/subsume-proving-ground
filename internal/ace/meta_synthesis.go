package ace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type MetaSynthesisRequest struct {
	Task             Task                 `json:"task"`
	Grammar          string               `json:"grammar"`
	Diagnosis        BottleneckDiagnosis  `json:"diagnosis"`
	Telemetry        AcquisitionTelemetry `json:"telemetry"`
	TrainingExamples []ProgramTestCase    `json:"training_examples"`
	Contract         string               `json:"contract"`
}

type MetaSynthesisResponse struct {
	NodeKind     string   `json:"node_kind"`
	GrammarDelta []string `json:"grammar_delta"`
	GoSource     string   `json:"go_source"`
}

type MetaSynthesizer interface {
	Synthesize(context.Context, MetaSynthesisRequest) (MetaSynthesisResponse, error)
}

type HTTPMetaSynthesizer struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func NewHTTPMetaSynthesizerFromEnv() (*HTTPMetaSynthesizer, error) {
	url := strings.TrimSpace(os.Getenv("ACE_META_SYNTH_URL"))
	if url == "" {
		return nil, errors.New("ACE_META_SYNTH_URL is not configured")
	}
	timeout := 2 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("ACE_META_SYNTH_TIMEOUT")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid ACE_META_SYNTH_TIMEOUT: %w", err)
		}
		timeout = d
	}
	return &HTTPMetaSynthesizer{
		URL:   url,
		Token: strings.TrimSpace(os.Getenv("ACE_META_SYNTH_TOKEN")),
		HTTP:  &http.Client{Timeout: timeout},
	}, nil
}

func (s *HTTPMetaSynthesizer) Synthesize(ctx context.Context, req MetaSynthesisRequest) (MetaSynthesisResponse, error) {
	if s == nil || strings.TrimSpace(s.URL) == "" {
		return MetaSynthesisResponse{}, errors.New("meta synthesizer endpoint is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return MetaSynthesisResponse{}, fmt.Errorf("encode meta-synthesis request: %w", err)
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(payload))
	if err != nil {
		return MetaSynthesisResponse{}, fmt.Errorf("create meta-synthesis request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if s.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.Token)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return MetaSynthesisResponse{}, fmt.Errorf("meta-synthesis endpoint request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return MetaSynthesisResponse{}, fmt.Errorf("read meta-synthesis response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MetaSynthesisResponse{}, fmt.Errorf("meta-synthesis endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out MetaSynthesisResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return MetaSynthesisResponse{}, fmt.Errorf("decode meta-synthesis response: %w", err)
	}
	if strings.TrimSpace(out.NodeKind) == "" || strings.TrimSpace(out.GoSource) == "" {
		return MetaSynthesisResponse{}, errors.New("meta-synthesis response must include node_kind and go_source")
	}
	return out, nil
}

const currentUniversalGrammar = "UExpr={const,var,add,sub,mul,lt,eq,and,or,not}; UStmt={assign,if,repeat}; integer/boolean key-value inputs; repeat count <=100; 1000 execution-step cap; no string-symbol primitive, stack primitive, or recursive AST node."

const generatedMetaNodeContract = "Return Go source as package main defining type Node with method Classify(string) (string,error). The node must solve arbitrary-depth balanced-bracket strings. It must not import repository code, network clients, os/exec, syscall, plugin, or unsafe. The host will compile the returned source, assert Node satisfies its runtime interface, then execute it against evaluator-held hidden examples."

func validateGeneratedGoSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return "", fmt.Errorf("generated Go failed gofmt/parser validation: %w", err)
	}
	return string(formatted), nil
}

type generatedMetaCase struct {
	Input string `json:"input"`
}

type generatedMetaResult struct {
	Outputs []string `json:"outputs"`
	Errors  []string `json:"errors"`
}

func compileAndRunGeneratedNode(ctx context.Context, source string, hidden []ProgramTestCase) (generatedMetaResult, string, error) {
	source, err := validateGeneratedGoSource(source)
	if err != nil {
		return generatedMetaResult{}, "", err
	}
	if !strings.Contains(source, "package main") || !strings.Contains(source, "type Node") || !strings.Contains(source, "Classify(") {
		return generatedMetaResult{}, "", errors.New("generated Go does not expose the required Node/Classify contract")
	}
	for _, forbidden := range []string{""os"", ""os/exec"", ""net/http"", ""net"", ""syscall"", ""plugin"", ""unsafe""} {
		if strings.Contains(source, forbidden) {
			return generatedMetaResult{}, "", fmt.Errorf("generated Go contains forbidden dependency %s", forbidden)
		}
	}
	dir, err := os.MkdirTemp("", "ace-meta-node-*")
	if err != nil {
		return generatedMetaResult{}, "", err
	}
	defer os.RemoveAll(dir)

	goMod := "module meta_generated\n\ngo 1.24\n"
	wrapper := "package main\n\n" +
		"import (\n\t\"encoding/json\"\n\t\"os\"\n)\n\n" +
		"type GeneratedNode interface { Classify(string) (string, error) }\n" +
		"var _ GeneratedNode = Node{}\n\n" +
		"func main() {\n" +
		"\tvar cases []map[string]string\n" +
		"\tif err := json.NewDecoder(os.Stdin).Decode(&cases); err != nil { panic(err) }\n" +
		"\tnode := Node{}\n" +
		"\tresult := map[string]any{\"outputs\": []string{}, \"errors\": []string{}}\n" +
		"\tfor _, tc := range cases {\n" +
		"\t\tout, err := node.Classify(tc[\"input\"])\n" +
		"\t\tresult[\"outputs\"] = append(result[\"outputs\"].([]string), out)\n" +
		"\t\tif err != nil { result[\"errors\"] = append(result[\"errors\"].([]string), err.Error()) } else { result[\"errors\"] = append(result[\"errors\"].([]string), \"\") }\n" +
		"\t}\n" +
		"\tif err := json.NewEncoder(os.Stdout).Encode(result); err != nil { panic(err) }\n" +
		"}\n"

	for name, data := range map[string][]byte{
		"go.mod":          []byte(goMod),
		"generated_node.go": []byte(source),
		"wrapper.go":       []byte(wrapper),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return generatedMetaResult{}, "", err
		}
	}

	cases := make([]generatedMetaCase, 0, len(hidden))
	for _, tc := range hidden {
		cases = append(cases, generatedMetaCase{Input: tc.Input["s"]})
	}
	payload, err := json.Marshal(cases)
	if err != nil {
		return generatedMetaResult{}, "", err
	}

	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "go", "run", ".")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	trace := strings.TrimSpace(strings.TrimSpace(stderr.String()) + "\n" + strings.TrimSpace(stdout.String()))
	if runCtx.Err() != nil {
		return generatedMetaResult{}, trace, runCtx.Err()
	}
	if err != nil {
		return generatedMetaResult{}, trace, fmt.Errorf("generated node compile/run failed: %w", err)
	}
	var result generatedMetaResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return generatedMetaResult{}, trace, fmt.Errorf("generated node emitted invalid result: %w", err)
	}
	return result, trace, nil
}

func verifyGeneratedDyckResult(result generatedMetaResult, hidden []ProgramTestCase) error {
	if len(result.Outputs) != len(hidden) || len(result.Errors) != len(hidden) {
		return fmt.Errorf("generated node returned %d outputs/%d errors for %d hidden cases", len(result.Outputs), len(result.Errors), len(hidden))
	}
	for i, tc := range hidden {
		want := tc.Expected["balanced"]
		if result.Errors[i] != "" {
			return fmt.Errorf("generated node errored on hidden case %d: %s", i, result.Errors[i])
		}
		if result.Outputs[i] != want {
			return fmt.Errorf("generated node failed hidden case %d: got %q want %q", i, result.Outputs[i], want)
		}
	}
	return nil
}

func MetaSynthesizeASTMutation(ctx context.Context, synth MetaSynthesizer, task Task, telemetry AcquisitionTelemetry, diagnosis BottleneckDiagnosis, training []ProgramTestCase, hidden []ProgramTestCase) (ASTMutationProposal, error) {
	if diagnosis.Class != BottleneckSearchSpace {
		return ASTMutationProposal{}, fmt.Errorf("AST mutation requires BottleneckSearchSpace, got %s", diagnosis.Class)
	}
	if synth == nil {
		return ASTMutationProposal{}, errors.New("meta synthesizer is nil")
	}
	response, err := synth.Synthesize(ctx, MetaSynthesisRequest{
		Task:             task,
		Grammar:          currentUniversalGrammar,
		Diagnosis:        diagnosis,
		Telemetry:        telemetry,
		TrainingExamples: training,
		Contract:         generatedMetaNodeContract,
	})
	if err != nil {
		return ASTMutationProposal{}, err
	}
	result, trace, err := compileAndRunGeneratedNode(ctx, response.GoSource, hidden)
	if err != nil {
		return ASTMutationProposal{}, fmt.Errorf("generated node evaluation failed: %w; trace=%q", err, trace)
	}
	if err := verifyGeneratedDyckResult(result, hidden); err != nil {
		return ASTMutationProposal{}, fmt.Errorf("generated node did not solve hidden Dyck cases: %w; trace=%q", err, trace)
	}
	source, err := validateGeneratedGoSource(response.GoSource)
	if err != nil {
		return ASTMutationProposal{}, err
	}
	return ASTMutationProposal{
		ID:               fmt.Sprintf("ast-mutation:%s", Hash([]any{task.ID, diagnosis.Reason, source})),
		NodeKind:         response.NodeKind,
		GrammarDelta:     response.GrammarDelta,
		Trigger:          diagnosis,
		GeneratedSource:  source,
		Compiled:         true,
		HiddenVerified:   true,
		EvaluationTrace:  trace,
	}, nil
}
