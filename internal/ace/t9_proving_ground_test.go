package ace

import (
	"context"
	"strings"
	"testing"
)

type t9FakeCommitter struct {
	branches []string
	commits  []string
	files    map[string]string
}

func (f *t9FakeCommitter) CreateBranch(_ context.Context, branch, _ string) error {
	f.branches = append(f.branches, branch)
	return nil
}

func (f *t9FakeCommitter) CommitFiles(_ context.Context, _ string, files map[string]string, message string) (string, error) {
	if f.files == nil {
		f.files = map[string]string{}
	}
	for k, v := range files {
		f.files[k] = v
	}
	f.commits = append(f.commits, message)
	return Hash([]any{message, files}), nil
}

type t9FakeRunner struct{ n int }

func (r *t9FakeRunner) Run(_ context.Context, _ string, _ string) (T9RunResult, error) {
	r.n++
	if r.n == 1 {
		return T9RunResult{Passed: false, Output: "adversarial digest mismatch"}, nil
	}
	return T9RunResult{Passed: true, Output: "ok"}, nil
}

func TestT9DeterministicAdversarialGeneratorProducesCompleteGoTest(t *testing.T) {
	c, err := (DeterministicAdversarialGenerator{}).Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.TestSource, "func TestT9GeneratedAdversarial_") {
		t.Fatal("generated artifact is not a complete Go test")
	}
	if c.TestPath == "" || c.CandidatePath == "" || c.ExpectedDigest == "" {
		t.Fatal("generated challenge missing required artifact fields")
	}
}

func TestT9CrucibleFailureRepairAndF0Admission(t *testing.T) {
	runtime := newF0TestRuntime(t)
	committer := &t9FakeCommitter{}
	runner := &t9FakeRunner{}
	g := &AdversarialProvingGround{
		Generator: DeterministicAdversarialGenerator{},
		Runner: runner,
		Synthesizer: DeterministicT9PatchSynthesizer{},
		Committer: committer,
		Runtime: &runtime,
		BaseRef: "7dfe7098b89c767aa6932d89674ce897f0774cb3",
	}
	result, err := g.RunCrucible(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.FinalPassed || result.AdmissionRef == "" || result.AdmissionHash == "" {
		t.Fatalf("crucible did not seal verified result: %+v", result)
	}
	if len(committer.branches) != 1 || len(committer.commits) != 2 {
		t.Fatalf("expected isolated branch plus test+patch commits, got branches=%d commits=%d", len(committer.branches), len(committer.commits))
	}
	if !strings.Contains(result.InitialFailure, "adversarial") {
		t.Fatalf("initial failure was not captured: %q", result.InitialFailure)
	}
}

func TestT9PatchSynthesizerRejectsPassingCandidate(t *testing.T) {
	c, err := (DeterministicAdversarialGenerator{}).Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = (DeterministicT9PatchSynthesizer{}).Synthesize(context.Background(), c, T9RunResult{Passed: true})
	if err == nil {
		t.Fatal("passing candidate must not trigger recursive patch synthesis")
	}
}
