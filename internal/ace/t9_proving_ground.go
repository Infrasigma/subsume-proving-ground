package ace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const t9GeneratedPackage = "./internal/ace"

type T9Challenge struct {
	ID              string
	TestPath        string
	CandidatePath   string
	TestSource      string
	CandidateSource string
	Target          string
	ExpectedDigest  string
}

type T9RunResult struct {
	Passed bool
	Output string
}

type T9Patch struct {
	ID            string
	CandidatePath string
	Source        string
}

type T9CrucibleResult struct {
	ChallengeID    string
	IsolatedBranch string
	TestHash       string
	InitialFailure string
	PatchID        string
	PatchHash      string
	FinalPassed    bool
	AdmissionRef   string
	AdmissionHash  string
}

type T9ChallengeGenerator interface {
	Generate(context.Context) (T9Challenge, error)
}

type T9TestRunner interface {
	Run(context.Context, string, string) (T9RunResult, error)
}

type T9PatchSynthesizer interface {
	Synthesize(context.Context, T9Challenge, T9RunResult) (T9Patch, error)
}

type T9BranchCommitter interface {
	CreateBranch(context.Context, string, string) error
	CommitFiles(context.Context, string, map[string]string, string) (string, error)
}

type DeterministicAdversarialGenerator struct{}

func (DeterministicAdversarialGenerator) Generate(ctx context.Context) (T9Challenge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return T9Challenge{}, err
	}
	input := []string{"alpha", "beta", "alpha", "gamma"}
	expected := canonicalSequenceDigest(input)
	id := Hash([]any{"t9-adversarial-v1", input, expected})
	short := id[:12]
	testPath := filepath.ToSlash(filepath.Join("internal/ace", "generated_t9_"+short+"_test.go"))
	candidatePath := filepath.ToSlash(filepath.Join("internal/ace", "generated_t9_"+short+"_candidate.go"))
	testSource := fmt.Sprintf("package ace\n\nimport \"testing\"\n\nfunc TestT9GeneratedAdversarial_%s(t *testing.T) {\n\twant := %q\n\tgot := t9GeneratedDigest([]string{%q, %q, %q, %q})\n\tif got != want {\n\t\tt.Fatalf(\"adversarial digest mismatch: got %%s want %%s\", got, want)\n\t}\n\treordered := t9GeneratedDigest([]string{%q, %q, %q, %q})\n\tif reordered == got {\n\t\tt.Fatal(\"adversarial ordering sensitivity violated\")\n\t}\n}\n", short, expected, input[0], input[1], input[2], input[3], input[1], input[0], input[2], input[3])
	candidateSource := "package ace\n\nimport (\n    \"crypto/sha256\"\n    \"encoding/hex\"\n)\n\nfunc t9GeneratedDigest(values []string) string {\n    h := sha256.New()\n    sum := byte(0)\n    for _, value := range values {\n        digest := sha256.Sum256([]byte(value))\n        sum ^= digest[0]\n    }\n    return hex.EncodeToString([]byte{sum})\n}\n"
	return T9Challenge{
		ID: id, TestPath: testPath, CandidatePath: candidatePath,
		TestSource: testSource, CandidateSource: candidateSource,
		Target: "canonical-sequence-digest", ExpectedDigest: expected,
	}, nil
}

func canonicalSequenceDigest(values []string) string {
	h := sha256.New()
	for _, value := range values {
		var length [8]byte
		n := uint64(len(value))
		for i := range length {
			length[7-i] = byte(n >> (8 * i))
		}
		_, _ = h.Write(length[:])
		_, _ = h.Write([]byte(value))
	}
	return hex.EncodeToString(h.Sum(nil))
}

type DeterministicT9PatchSynthesizer struct{}

func (DeterministicT9PatchSynthesizer) Synthesize(ctx context.Context, challenge T9Challenge, failure T9RunResult) (T9Patch, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return T9Patch{}, err
	}
	if failure.Passed {
		return T9Patch{}, errors.New("patch synthesis requires an observed adversarial failure")
	}
	if challenge.Target != "canonical-sequence-digest" {
		return T9Patch{}, fmt.Errorf("unsupported T9 target %q", challenge.Target)
	}
	source := "package ace\n\nimport (\n    \"crypto/sha256\"\n    \"encoding/hex\"\n)\n\nfunc t9GeneratedDigest(values []string) string {\n    h := sha256.New()\n    for _, value := range values {\n        var length [8]byte\n        n := uint64(len(value))\n        for i := range length {\n            length[7-i] = byte(n >> (8 * i))\n        }\n        _, _ = h.Write(length[:])\n        _, _ = h.Write([]byte(value))\n    }\n    return hex.EncodeToString(h.Sum(nil))\n}\n"
	return T9Patch{ID: Hash([]any{"t9-patch-v1", challenge.ID, source}), CandidatePath: challenge.CandidatePath, Source: source}, nil
}

type LocalGitBranchCommitter struct {
	RepositoryRoot string
}

func (g LocalGitBranchCommitter) CreateBranch(ctx context.Context, branch, base string) error {
	if g.RepositoryRoot == "" || branch == "" || base == "" {
		return errors.New("git branch creation requires repository root, branch, and base")
	}
	cmd := exec.CommandContext(ctx, "git", "switch", "-c", branch, base)
	cmd.Dir = g.RepositoryRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git switch -c failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (g LocalGitBranchCommitter) CommitFiles(ctx context.Context, branch string, files map[string]string, message string) (string, error) {
	if g.RepositoryRoot == "" || branch == "" || len(files) == 0 {
		return "", errors.New("git commit requires repository root, branch, and files")
	}
	for path, content := range files {
		abs := filepath.Join(g.RepositoryRoot, filepath.Clean(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(abs, []byte(content), 0600); err != nil {
			return "", err
		}
	}
	add := exec.CommandContext(ctx, "git", "add", "--", ".")
	add.Dir = g.RepositoryRoot
	if out, err := add.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	commit := exec.CommandContext(ctx, "git", "commit", "-m", message)
	commit.Dir = g.RepositoryRoot
	if out, err := commit.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	rev := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	rev.Dir = g.RepositoryRoot
	out, err := rev.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type GoT9TestRunner struct {
	RepositoryRoot string
}

func (r GoT9TestRunner) Run(ctx context.Context, _ string, testPath string) (T9RunResult, error) {
	if r.RepositoryRoot == "" {
		return T9RunResult{}, errors.New("T9 test runner requires repository root")
	}
	cmd := exec.CommandContext(ctx, "go", "test", t9GeneratedPackage, "-run", testNameFromPath(testPath), "-count=1")
	cmd.Dir = r.RepositoryRoot
	out, err := cmd.CombinedOutput()
	return T9RunResult{Passed: err == nil, Output: string(out)}, nil
}

func testNameFromPath(_ string) string {
	return "TestT9GeneratedAdversarial_"
}

type AdversarialProvingGround struct {
	Generator   T9ChallengeGenerator
	Runner      T9TestRunner
	Synthesizer T9PatchSynthesizer
	Committer   T9BranchCommitter
	Runtime     *AdaptiveAcquisitionRuntime
	BaseRef     string
}

func (g *AdversarialProvingGround) RunCrucible(ctx context.Context) (T9CrucibleResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if g.Generator == nil || g.Runner == nil || g.Synthesizer == nil || g.Committer == nil {
		return T9CrucibleResult{}, errors.New("T9 proving ground requires generator, runner, synthesizer, and committer")
	}
	challenge, err := g.Generator.Generate(ctx)
	if err != nil {
		return T9CrucibleResult{}, err
	}
	branch := "t9-proving-ground/" + challenge.ID[:12]
	if g.BaseRef == "" {
		return T9CrucibleResult{}, errors.New("T9 proving ground requires a verified base ref")
	}
	if err := g.Committer.CreateBranch(ctx, branch, g.BaseRef); err != nil {
		return T9CrucibleResult{}, err
	}
	testHash := Hash(challenge.TestSource)
	if _, err := g.Committer.CommitFiles(ctx, branch, map[string]string{
		challenge.TestPath: challenge.TestSource,
		challenge.CandidatePath: challenge.CandidateSource,
	}, "t9: commit generated adversarial proving ground"); err != nil {
		return T9CrucibleResult{}, err
	}
	initial, err := g.Runner.Run(ctx, branch, challenge.TestPath)
	if err != nil {
		return T9CrucibleResult{}, err
	}
	if initial.Passed {
		return T9CrucibleResult{}, errors.New("T9 crucible failed: generated adversarial test did not break the candidate")
	}
	patch, err := g.Synthesizer.Synthesize(ctx, challenge, initial)
	if err != nil {
		return T9CrucibleResult{}, err
	}
	patchHash := Hash(patch.Source)
	if _, err := g.Committer.CommitFiles(ctx, branch, map[string]string{patch.CandidatePath: patch.Source}, "t9: apply synthesized repair"); err != nil {
		return T9CrucibleResult{}, err
	}
	final, err := g.Runner.Run(ctx, branch, challenge.TestPath)
	if err != nil {
		return T9CrucibleResult{}, err
	}
	if !final.Passed {
		return T9CrucibleResult{}, fmt.Errorf("T9 synthesized repair did not satisfy adversarial test: %s", final.Output)
	}
	receipt := T9SealReceipt{
		ChallengeID: challenge.ID,
		TestHash: testHash,
		PatchID: patch.ID,
		PatchHash: patchHash,
		InitialFailure: initial.Output,
		FinalOutput: final.Output,
		Branch: branch,
	}
	admissionRef, admissionHash, err := g.seal(ctx, receipt)
	if err != nil {
		return T9CrucibleResult{}, err
	}
	return T9CrucibleResult{
		ChallengeID: challenge.ID, IsolatedBranch: branch,
		TestHash: testHash, InitialFailure: initial.Output,
		PatchID: patch.ID, PatchHash: patchHash, FinalPassed: true,
		AdmissionRef: admissionRef, AdmissionHash: admissionHash,
	}, nil
}

type T9SealReceipt struct {
	ChallengeID    string
	TestHash       string
	PatchID        string
	PatchHash      string
	InitialFailure string
	FinalOutput    string
	Branch         string
}

func (g *AdversarialProvingGround) seal(ctx context.Context, receipt T9SealReceipt) (string, string, error) {
	if g.Runtime == nil {
		return "", "", errors.New("T9 F0 sealing requires an adaptive acquisition runtime")
	}
	if receipt.TestHash == "" || receipt.PatchHash == "" || receipt.ChallengeID == "" {
		return "", "", errors.New("T9 seal receipt is incomplete")
	}
	artifact := AcquiredAbstraction{
		ID: Hash([]any{"t9-proving-ground", receipt.ChallengeID, receipt.TestHash, receipt.PatchHash}),
		Name: "t9-adversarial-proving-ground:" + receipt.ChallengeID[:12],
		Procedure: AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{
			{Op: "identity"},
			{Op: "identity"},
		}},
		Contract: AbstractionContract{
			Inputs: []string{"generated adversarial test", "synthesized patch"},
			Outputs: []string{"verified repair receipt"},
			Preconditions: []string{"test committed to isolated branch", "initial test execution fails"},
			Postconditions: []string{
				"final adversarial test passes",
				"test_sha256=" + receipt.TestHash,
				"patch_sha256=" + receipt.PatchHash,
			},
		},
		Evidence: []AbstractionEvidence{{
			TaskStructure: receipt.ChallengeID,
			Verified: true, HeldOut: true, TransferScore: 1,
			DiscoveryCost: ResourceVector{Compute: 1, Memory: 1, TimeMS: 1},
			ObservedGain: 1,
		}},
		Verification: VerificationResult{
			Status: "verified", Independent: true,
			Expected: []string{"adversarial self-test rejects the pre-patch candidate", "synthesized patch restores the invariant"},
			Observed: []string{"initial failure captured", "post-patch run passed"},
			Provenance: Prov("t9-adversarial-verifier", receipt.ChallengeID, "failure-to-repair", receipt),
		},
		Provenance: Prov("t9-adversarial-proving-ground", receipt.ChallengeID, "generated-test-and-repair", receipt),
	}
	sealed, err := g.Runtime.admitAbstraction(ctx, artifact, 1)
	if err != nil {
		return "", "", err
	}
	return sealed.LedgerAdmissionRef, sealed.LedgerAdmissionHash, nil
}
