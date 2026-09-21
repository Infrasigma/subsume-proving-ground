package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Status string
const (
	Pass Status = "PASS"
	Fail Status = "FAIL"
	Insufficient Status = "INSUFFICIENT"
)

type Gate struct {
	ID string
	Name string
	Status Status
	TestRegex string
	Evidence string
	Interpretation string
}

type Run struct {
	Schema string
	TimestampUTC string
	Commit string
	Parent string
	Dirty bool
	GoVersion string
	EvaluatorDigest string
	Gates []Gate
	Verdict string
	ScientificNote string
}

func run(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	b, err := c.CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

func gateTest(regex string) (Status, string) {
	out, err := run("go", "test", "-mod=mod", "-run", regex, "./internal/ace", "-count=1", "-timeout", "10m", "-v")
	if err != nil { return Fail, out }
	return Pass, out
}

func shaForPath(path string) string {
	out, err := run("git", "rev-parse", "HEAD:"+path)
	if err != nil { return "UNAVAILABLE" }
	return out
}

func evaluatorDigest() string {
	paths := []string{
		"internal/ace/f0_sealing_test.go",
		"internal/ace/blind_acquisition_gate.go",
		"internal/ace/recursive_cognitive_compounding_test.go",
		"internal/ace/universal.go",
		"internal/ace/bottleneck_diagnosis.go",
	}
	parts := make([]string, 0, len(paths))
	for _, p := range paths { parts = append(parts, p+"="+shaForPath(p)) }
	return strings.Join(parts, "|")
}

func main() {
	head, _ := run("git", "rev-parse", "HEAD")
	parent, _ := run("git", "rev-parse", "HEAD^")
	status, _ := run("git", "status", "--porcelain")
	goVersion, _ := run("go", "version")

	gates := []Gate{
		{ID:"F0", Name:"Immutable evaluator / admission integrity", TestRegex:"^TestExecutionRejectsManuallyInjectedUnsignedAbstraction$", Evidence:"F0 cryptographic admission rejection test", Interpretation:"Evaluator boundary exists at the abstraction-admission layer."},
		{ID:"F1", Name:"Verified experience acquisition", TestRegex:"^TestGeneralAcquisitionUsesIndependentCounterexamples$", Evidence:"Independent counterexample driven acquisition", Interpretation:"Experience can become a retained verified capability."},
		{ID:"F2", Name:"Blind cross-task transfer", TestRegex:"^TestBlindAcquisitionGateShowsFutureAcquisitionGain$", Evidence:"8 evaluator-hidden future tasks; direct baseline versus retained-composition control", Interpretation:"Verified retained capabilities can outperform the direct acquisition frontier on a blind future family."},
		{ID:"F3", Name:"Recursive mechanism compounding", TestRegex:"^TestRecursiveCognitiveMechanismCompounding$", Evidence:"Three generations with predecessor removal controls", Interpretation:"Acquired procedures can become reusable primitives for deeper successor mechanisms."},
		{ID:"F4", Name:"Meta-search", TestRegex:"^TestT4MetacognitiveHotSwapSealsAndRehydrates$", Evidence:"Verified search-heuristic hot swap", Interpretation:"The search controller can be replaced by a verified successor in the current narrow task family."},
		{ID:"F5", Name:"Representation revision", TestRegex:"^TestRepresentationRevisionInventsMissingVariableAndRejectsIrrelevantDistinction$", Evidence:"Residual-reduction representation revision", Interpretation:"The system can select a missing representation feature in a controlled formal environment."},
		{ID:"F6", Name:"Autotelic task generation", TestRegex:"^TestT5AutotelicExpansionFromEmptyQueue$", Evidence:"Capability-boundary driven task generation", Interpretation:"The runtime can generate and admit a next task from its current verified boundary."},
		{ID:"F7", Name:"Cross-domain recursive transfer", TestRegex:"^TestF7CrossDomainStructuralTransfer$", Evidence:"Role-preserving transfer into a structurally different record domain", Interpretation:"A learned relational mechanism transfers across surface representation types in the controlled test."},
		{ID:"F8", Name:"Knowledge consolidation", TestRegex:"^TestF8KnowledgeConsolidationSurvivesEpisodeDeletion$", Evidence:"Rehydrated executable knowledge succeeds after raw episode deletion", Interpretation:"A verified executable abstraction can survive removal of the training episodes in this formal setting."},
		{ID:"F9", Name:"Cognitive substrate replacement", TestRegex:"^TestF9CognitiveSubstrateReplacement$", Evidence:"Retained executable mechanism runs after original search controller removal", Interpretation:"A narrow acquired procedure can function as a replacement execution substrate."},
		{ID:"F10", Name:"Recursive self-improvement of improvement", TestRegex:"^TestF10AutonomousRecursiveAbstractionImprovesDiscoveryCost$", Evidence:"Generic anti-unification of predecessor procedures followed by lower-cost hidden-task discovery", Interpretation:"The experiment tests whether recursive abstraction can make later mechanism discovery cheaper; this remains narrow evidence, not general self-improvement."},
		{ID:"F11", Name:"Resource-normalized improvement", TestRegex:"^TestF11ResourceNormalizedRecursiveImprovement$", Evidence:"Three successor families with measured search, verification, semantic, and artifact costs", Interpretation:"The narrow recursive-abstraction probe now tests whether later mechanism discovery becomes cheaper under a declared resource proxy."},
		{ID:"F12", Name:"Adversarial heterogeneous novelty", TestRegex:"^TestF12AdversarialHeterogeneousNovelty$", Evidence:"Four heterogeneous hidden encodings plus a novel-operator shock requiring behavior absent from the complete frozen baseline", Interpretation:"This gate tests whether recursive transfer survives representation changes and whether the system can autonomously extend its executable semantic language."},
		{ID:"F13", Name:"Open-ended capability growth", TestRegex:"^TestF13EndogenousOpenEndedCapabilityGrowth$", Evidence:"Ten successive capability generations generated from capability state plus generic gap telemetry, with task history deleted from generator input", Interpretation:"This is a bounded formal open-endedness test: capability generation continues without a prewritten future task list."},
		{ID:"F14", Name:"Broad superhuman cognitive battery", TestRegex:"^TestF14BroadCognitiveBattery$", Evidence:"Eight heterogeneous task families with independent hidden cases and no partial-pass promotion", Interpretation:"A broad claim requires one substrate to clear every family; partial scalar competence is insufficient."},
		{ID:"F15", Name:"TRUE ASI", Status:Insufficient, Evidence:"No evidence satisfying F7-F14 exists", Interpretation:"ASI is not established."},
	}

	for i := range gates {
		if gates[i].Status == "" && gates[i].TestRegex != "" {
			gates[i].Status, gates[i].Evidence = gateTest(gates[i].TestRegex)
		}
	}

	verdict := "KILL"
	if gates[len(gates)-1].Status == Pass { verdict = "ASI" }

	data := Run{
		Schema:"asi-forge/v1",
		TimestampUTC:time.Now().UTC().Format(time.RFC3339Nano),
		Commit:head,
		Parent:parent,
		Dirty:strings.TrimSpace(status)!="",
		GoVersion:goVersion,
		EvaluatorDigest:evaluatorDigest(),
		Gates:gates,
		Verdict:verdict,
		ScientificNote:"Only executed independent gates are eligible for PASS. INSUFFICIENT is not a success state.",
	}

	root, _ := run("git", "rev-parse", "--show-toplevel")
	jsonPath := filepath.Join(root, "asi_forge", "artifacts", "ASI_FORGE_RUN.json")
	mdPath := filepath.Join(root, "asi_forge", "artifacts", "ASI_FORGE_RUN.md")
	_ = os.MkdirAll(filepath.Dir(jsonPath), 0755)

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil { panic(err) }
	if err := os.WriteFile(jsonPath, append(b, '\n'), 0644); err != nil { panic(err) }

	var md strings.Builder
	fmt.Fprintf(&md, "# ASI Forge Run\n\nCommit: %s\n\nVerdict: **%s**\n\nEvaluator digest: %s\n\n", head, verdict, data.EvaluatorDigest)
	for _, g := range gates {
		fmt.Fprintf(&md, "## %s — %s\n\nStatus: **%s**\n\nEvidence: %s\n\nInterpretation: %s\n\n", g.ID, g.Name, g.Status, g.Evidence, g.Interpretation)
	}
	if err := os.WriteFile(mdPath, []byte(md.String()), 0644); err != nil { panic(err) }

	fmt.Print(string(b))
	if verdict != "ASI" { os.Exit(2) }
}
