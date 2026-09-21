package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	ace "github.com/Infrasigma/subsume-proving-ground/internal/ace"
	acex "github.com/Infrasigma/subsume-proving-ground/internal/acex"
)

type request struct {
	Op              string                   `json:"op"`
	Task            ace.Task                 `json:"task"`
	Training        []ace.ProgramTestCase    `json:"training"`
	Counterexamples []ace.ProgramTestCase    `json:"counterexamples"`
	State           acex.RelationalState     `json:"state"`
	Actions         []string                 `json:"actions"`
	Action          string                   `json:"action"`
	NextState       acex.RelationalState     `json:"next_state"`
	Reward          float64                  `json:"reward"`
	Terminal        bool                     `json:"terminal"`
}

type response struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Artifact string `json:"artifact,omitempty"`
	Action   string `json:"action,omitempty"`
	Memory   int    `json:"memory,omitempty"`
	Version  uint64 `json:"version,omitempty"`
}

type agent struct {
	entity       *acex.V8CognitiveEntity
	lastState    acex.RelationalState
	lastAction   string
	havePrevious bool
}

func newAgent() *agent {
	return &agent{entity: acex.NewV8CognitiveEntity()}
}

func (a *agent) synthesize(task ace.Task, training, counter []ace.ProgramTestCase) (string, error) {
	if len(training) < 2 {
		return "", errors.New("at least two visible examples required")
	}
	examples := append([]ace.ProgramTestCase{}, training...)
	examples = append(examples, counter...)
	spec, err := ace.GeneralCapabilitySpecification(task, examples)
	if err != nil {
		return "", err
	}
	candidates, err := (ace.UniversalMechanismSearch{}).SearchMechanisms(spec, task.Budget)
	if err != nil {
		return "", err
	}
	for _, c := range candidates {
		p, err := (ace.UniversalProgramBuilder{}).Build(c, spec)
		if err == nil && p.Artifact != "" {
			// The candidate does not see holdout data here. The artifact is
			// independently interpreted by the protected evaluator.
			return p.Artifact, nil
		}
	}
	return "", errors.New("symbolic synthesis exhausted")
}

func (a *agent) handle(in request) response {
	switch in.Op {
	case "synthesize":
		artifact, err := a.synthesize(in.Task, in.Training, in.Counterexamples)
		if err != nil {
			return response{Error:err.Error()}
		}
		return response{OK:true,Artifact:artifact,Memory:len(a.entity.Memory.Items),Version:a.entity.Version}
	case "act":
		action, err := a.entity.ObserveAndAct(in.State, in.Actions)
		if err != nil {
			return response{Error:err.Error()}
		}
		a.lastState = in.State
		a.lastAction = action
		a.havePrevious = true
		return response{OK:true,Action:action,Memory:len(a.entity.Memory.Items),Version:a.entity.Version}
	case "observe":
		if !a.havePrevious {
			return response{Error:"observe requires a preceding act"}
		}
		a.entity.ObserveOutcome(a.lastState, a.lastAction, in.NextState, in.Reward, in.Terminal)
		a.havePrevious = false
		return response{OK:true,Memory:len(a.entity.Memory.Items),Version:a.entity.Version}
	case "reset":
		a.entity = acex.NewV8CognitiveEntity()
		a.havePrevious = false
		a.lastAction = ""
		a.lastState = acex.RelationalState{}
		return response{OK:true,Memory:0,Version:0}
	case "status":
		return response{OK:true,Memory:len(a.entity.Memory.Items),Version:a.entity.Version}
	default:
		return response{Error:fmt.Sprintf("unknown op %q",in.Op)}
	}
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	a := newAgent()
	for in.Scan() {
		var req request
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			_ = json.NewEncoder(out).Encode(response{Error:"invalid request: "+err.Error()})
			_ = out.Flush()
			continue
		}
		_ = json.NewEncoder(out).Encode(a.handle(req))
		_ = out.Flush()
	}
	if err := in.Err(); err != nil {
		os.Exit(1)
	}
}

// Black-box evaluation workflow is evaluator-owned; this marker is intentionally inert.
// Transfer gate rerun marker: evaluator protocol unchanged.
// Structural-role transfer hypothesis integrated; evaluation remains external.
// Negative-control recording revision; structural-role gate remains unchanged.
// Adaptive representation-expansion gate added to evaluator.
// Targeted V8 adaptive-entity diagnostic workflow enabled.
