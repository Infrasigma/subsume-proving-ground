package acex

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

type V9Counterexample struct {
	Input  V4Value
	Actual V4Value
	Expected V4Value
	Probe  int
}

type V9Certificate struct {
	ProbeMin       int
	ProbeMax       int
	Counterexamples []V9Counterexample
	FinalVerified  bool
	Rounds         int
	Resource       Resource
	Digest         string
}

type V9CEGISResult struct {
	Program       V4Expr
	Certificate   V9Certificate
	TrainInputs   []V4Value
	TrainOutputs  []V4Value
	FailureMemory []string
}

func V9OracleCounterexample(candidate, truth V4Expr, lib V4Library, lo, hi int) (V9Counterexample, bool, error) {
	if lo > hi {
		return V9Counterexample{}, false, errors.New("invalid oracle range")
	}
	for x := lo; x <= hi; x++ {
		in := V4Value{Type: V4Int, Int: x}
		actual, err := v4Eval(candidate, in, lib, map[string]V4Value{}, map[string]bool{})
		if err != nil {
			return V9Counterexample{}, false, err
		}
		expected, err := v4Eval(truth, in, lib, map[string]V4Value{}, map[string]bool{})
		if err != nil {
			return V9Counterexample{}, false, err
		}
		if !v4SameValue(actual, expected) {
			return V9Counterexample{
				Input: in, Actual: actual, Expected: expected, Probe: x,
			}, true, nil
		}
	}
	return V9Counterexample{}, false, nil
}

func V9AppendCounterexample(t *V4Task, c V9Counterexample) error {
	if t == nil {
		return errors.New("nil task")
	}
	t.TrainInputs = append(t.TrainInputs, c.Input)
	t.TrainOutput = append(t.TrainOutput, c.Expected)
	return nil
}

func V9CEGIS(truth V4Expr, seed V4Task, lib V4Library, maxSize, beam, lo, hi, maxRounds int) (V9CEGISResult, error) {
	if maxRounds < 1 {
		return V9CEGISResult{}, errors.New("maxRounds must be positive")
	}
	task := seed
	certificate := V9Certificate{ProbeMin: lo, ProbeMax: hi}
	failures := []string{}
	for round := 1; round <= maxRounds; round++ {
		certificate.Rounds = round
		res, err := v4Search(task, lib, maxSize, beam, nil)
		certificate.Resource.Search += res.Cost.Search
		certificate.Resource.Verify += res.Cost.Verify
		if err != nil {
			return V9CEGISResult{Certificate:certificate,FailureMemory:failures}, err
		}
		certificate.Resource.Memory++
		counter, found, err := V9OracleCounterexample(res.Program, truth, lib, lo, hi)
		certificate.Resource.Verify += hi - lo + 1
		if err != nil {
			return V9CEGISResult{Certificate:certificate,FailureMemory:failures}, err
		}
		if !found {
			certificate.FinalVerified = true
			certificate.Digest = hashString(v4Signature(res.Program)+"|"+strconv.Itoa(lo)+"|"+strconv.Itoa(hi))
			return V9CEGISResult{
				Program:res.Program,
				Certificate:certificate,
				TrainInputs:append([]V4Value(nil),task.TrainInputs...),
				TrainOutputs:append([]V4Value(nil),task.TrainOutput...),
				FailureMemory:failures,
			}, nil
		}
		failures = append(failures, fmt.Sprintf(
			"round=%d counterexample x=%d actual=%s expected=%s",
			round,counter.Probe,v9ValueString(counter.Actual),v9ValueString(counter.Expected)))
		if err := V9AppendCounterexample(&task,counter); err != nil {
			return V9CEGISResult{}, err
		}
	}
	return V9CEGISResult{
		Certificate:certificate,FailureMemory:failures,
		TrainInputs:task.TrainInputs,TrainOutputs:task.TrainOutput,
	}, fmt.Errorf("CEGIS exhausted after %d rounds", maxRounds)
}

func v9ValueString(v V4Value) string {
	switch v.Type {
	case V4Int:
		return "i:"+strconv.Itoa(v.Int)
	case V4Bool:
		if v.Bool { return "b:1" }
		return "b:0"
	default:
		return "?"
	}
}

type V9Mechanism struct {
	Name        string
	UseCEGIS    bool
	ProbeMin    int
	ProbeMax    int
	MaxRounds   int
	SearchSize  int
	SearchBeam  int
}

func (m V9Mechanism) Key() string {
	return fmt.Sprintf("%s|cegis=%t|probe=%d:%d|rounds=%d|size=%d|beam=%d",
		m.Name,m.UseCEGIS,m.ProbeMin,m.ProbeMax,m.MaxRounds,m.SearchSize,m.SearchBeam)
}

func V9EvaluateMechanism(m V9Mechanism, truth V4Expr, seed V4Task, lib V4Library) (V9CEGISResult,error) {
	if !m.UseCEGIS {
		res, err := v4Search(seed,lib,m.SearchSize,m.SearchBeam,nil)
		if err != nil { return V9CEGISResult{},err }
		return V9CEGISResult{
			Program:res.Program,
			Certificate:V9Certificate{
				ProbeMin:m.ProbeMin,ProbeMax:m.ProbeMax,FinalVerified:false,
				Rounds:1,Resource:res.Cost,
			},
		},nil
	}
	return V9CEGIS(truth,seed,lib,m.SearchSize,m.SearchBeam,m.ProbeMin,m.ProbeMax,m.MaxRounds)
}

func V9CounterexampleDrivenImprovement(truth V4Expr, seed V4Task, lib V4Library) (V9Mechanism, V9CEGISResult, error) {
	baseline := V9Mechanism{
		Name:"baseline",UseCEGIS:false,ProbeMin:-8,ProbeMax:8,
		MaxRounds:1,SearchSize:7,SearchBeam:600,
	}
	cegis := V9Mechanism{
		Name:"cegis",UseCEGIS:true,ProbeMin:-8,ProbeMax:8,
		MaxRounds:6,SearchSize:7,SearchBeam:600,
	}
	baseResult, baseErr := V9EvaluateMechanism(baseline,truth,seed,lib)
	candResult, candErr := V9EvaluateMechanism(cegis,truth,seed,lib)
	if candErr != nil {
		return baseline,candResult,candErr
	}
	if !candResult.Certificate.FinalVerified {
		return baseline,candResult,errors.New("CEGIS candidate is not independently verified")
	}
	if baseErr == nil && baseResult.Program.Kind == "input" && candResult.Program.Kind == "input" {
		if v4Signature(baseResult.Program) == v4Signature(candResult.Program) {
			_ = math.Abs(0)
		}
	}
	return cegis,candResult,nil
}
