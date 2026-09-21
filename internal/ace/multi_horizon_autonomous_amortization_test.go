package ace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
		"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type amortizationTask struct {
	Train    []abstractCompoundExample
	Holdout  []abstractCompoundExample
	Target   []int
}

type amortizationSearchStats struct {
	Expansions    int `json:"expansions"`
	VerifierCalls int `json:"verifier_calls"`
	WallMS        int64 `json:"wall_ms"`
}

type amortizationHorizon struct {
	N                       int     `json:"n"`
	Tasks                   int     `json:"tasks"`
	ScratchSuccess          int     `json:"scratch_success"`
	RetainedSuccess         int     `json:"retained_success"`
	ScratchExpansions       int     `json:"scratch_expansions"`
	ScratchVerifierCalls    int     `json:"scratch_verifier_calls"`
	RetainedSearchExpansions int    `json:"retained_search_expansions"`
	RetainedVerifierCalls   int     `json:"retained_verifier_calls"`
	ScratchWallMS            int64   `json:"scratch_wall_ms"`
	RetainedWallMS           int64   `json:"retained_wall_ms"`
	ScratchLifetimeCost      int     `json:"scratch_lifetime_cost"`
	RetainedLifetimeCost     int     `json:"retained_lifetime_cost"`
	CostRatio                float64 `json:"cost_ratio"`
}

type amortizationSeedReport struct {
	Seed                    int                   `json:"seed"`
	Horizons                []amortizationHorizon `json:"horizons"`
	AcquisitionExpansions   int                   `json:"acquisition_expansions"`
	AcquisitionVerifierCalls int                  `json:"acquisition_verifier_calls"`
	RetainedBytes           int64                 `json:"retained_bytes"`
	RawMemoryBytes          int64                 `json:"raw_memory_bytes"`
	CompressedMemoryBytes   int64                 `json:"compressed_memory_bytes"`
	PersistentBytes         int64                 `json:"persistent_bytes"`
	ArtifactBytes           int64                 `json:"artifact_bytes"`
	ProcessRestartPassed    bool                  `json:"process_restart_passed"`
	RuntimeTamperRejected   bool                  `json:"runtime_tamper_rejected"`
	ManifestTamperDetected  bool                  `json:"manifest_tamper_detected"`
}

type amortizationReport struct {
	Commit                   string                  `json:"commit"`
	GoVersion                string                  `json:"go_version"`
	GOOS                     string                  `json:"goos"`
	GOARCH                   string                  `json:"goarch"`
	NumCPU                   int                     `json:"num_cpu"`
	Seeds                    []int                   `json:"seeds"`
	Horizons                 []int                   `json:"horizons"`
	PerSeed                  []amortizationSeedReport `json:"per_seed"`
	MeanRatios               map[string]float64      `json:"mean_ratios"`
	CrossoverN               int                     `json:"crossover_n"`
	AllRetainedBeatScratch   bool                    `json:"all_retained_beat_scratch"`
	AllTasksSolvedScratch    bool                    `json:"all_tasks_solved_scratch"`
	AllTasksSolvedRetained   bool                    `json:"all_tasks_solved_retained"`
	RestartPassRate           float64                  `json:"restart_pass_rate"`
	RuntimeTamperRejectRate  float64                  `json:"runtime_tamper_reject_rate"`
	ManifestTamperDetectRate float64                  `json:"manifest_tamper_detect_rate"`
	PeakRSSBytes             uint64                   `json:"peak_rss_bytes"`
	HeapAllocDeltaBytes      uint64                   `json:"heap_alloc_delta_bytes"`
	HeapInuseDeltaBytes      uint64                   `json:"heap_inuse_delta_bytes"`
	HeapSysDeltaBytes        uint64                   `json:"heap_sys_delta_bytes"`
	CPUTimeMS                 int64                    `json:"cpu_time_ms"`
	WallTimeMS                int64                    `json:"wall_time_ms"`
	PersistentBytes          int64                    `json:"persistent_bytes"`
	ArtifactBytes            int64                    `json:"artifact_bytes"`
	Classification            string                  `json:"classification"`
	Boundary                  string                  `json:"boundary"`
}

type amortizationHelperFixture struct {
	MethodsPath string                  `json:"methods_path"`
	Train       []abstractCompoundExample `json:"train"`
	Input       []ArchitectureCandidate `json:"input"`
}

type amortizationHelperResult struct {
	Methods []string `json:"methods"`
	Output  []string `json:"output"`
}

func searchAndVerifyAbstractCompoundProcedure(examples, heldOut []abstractCompoundExample, maxDepth int) (AcquisitionProcedure, abstractCompoundSearchStats, bool) {
	stats := abstractCompoundSearchStats{Depth: maxDepth}
	candidates := EnumerateAcquisitionProcedures(maxDepth)
	for _, p := range candidates {
		stats.Expansions++
		ok := true
		for _, ex := range examples {
			stats.VerifierCalls++
			got, err := executeSearchProcedure(p, ex.Input)
			if err != nil || !abstractCompoundStreamEqual(got, ex.Expected) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for _, ex := range heldOut {
			stats.VerifierCalls++
			got, err := executeSearchProcedure(p, ex.Input)
			ref := abstractCompoundReference(p, ex.Input)
			if err != nil || !abstractCompoundStreamEqual(got, ex.Expected) || !abstractCompoundStreamEqual(ref, ex.Expected) || !abstractCompoundStreamEqual(got, abstractCompoundMechanisms(ref)) {
				ok = false
				break
			}
		}
		if ok {
			return p, stats, true
		}
	}
	return AcquisitionProcedure{}, stats, false
}

func amortizationCapabilities() []AcquisitionMethodArtifact {
	specs := []struct {
		name string
		p    AcquisitionProcedure
	}{
		{"cap-A", AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "sort-cost"}, {Op: "rotate", Arg: 1}}}},
		{"cap-B", AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "reverse"}, {Op: "dedupe"}}}},
		{"cap-C", AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "rotate", Arg: 1}, {Op: "reverse"}}}},
		{"cap-D", AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "dedupe"}, {Op: "sort-cost"}}}},
		{"cap-E", AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "reverse"}, {Op: "rotate", Arg: 1}}}},
		{"cap-F", AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "sort-cost"}, {Op: "dedupe"}}}},
	}
	out := make([]AcquisitionMethodArtifact, 0, len(specs))
	for _, s := range specs {
		artifact, err := s.p.Marshal()
		if err != nil {
			panic(err)
		}
		out = append(out, AcquisitionMethodArtifact{
			ID: procedureSignature(s.p),
			Name: s.name + ":" + procedureSignature(s.p),
			Procedure: artifact,
			Artifact: artifact,
		})
	}
	return out
}

func amortizationApplyMethods(methods []AcquisitionMethodArtifact, input []ArchitectureCandidate) ([]ArchitectureCandidate, error) {
	return abstractCompoundApplyMethods(methods, input)
}

func amortizationCandidatePairs(methods []AcquisitionMethodArtifact) [][]AcquisitionMethodArtifact {
	out := make([][]AcquisitionMethodArtifact, 0, len(methods)*len(methods))
	for i := range methods {
		for j := range methods {
			out = append(out, []AcquisitionMethodArtifact{methods[i], methods[j]})
		}
	}
	return out
}

func amortizationApplyTraining(methods []AcquisitionMethodArtifact, train []abstractCompoundExample) bool {
	for _, ex := range train {
		got, err := amortizationApplyMethods(methods, ex.Input)
		if err != nil || !abstractCompoundStreamEqual(got, ex.Expected) {
			return false
		}
	}
	return true
}

func amortizationSelectPair(methods []AcquisitionMethodArtifact, task amortizationTask) ([]AcquisitionMethodArtifact, amortizationSearchStats, bool) {
	start := time.Now()
	stats := amortizationSearchStats{}
	seen := map[string]bool{}
	candidates := amortizationCandidatePairs(methods)
	sort.SliceStable(candidates, func(i, j int) bool {
		a := candidates[i][0].ID + "|" + candidates[i][1].ID
		b := candidates[j][0].ID + "|" + candidates[j][1].ID
		return a < b
	})
	for _, candidate := range candidates {
		key := candidate[0].ID + "|" + candidate[1].ID
		if seen[key] {
			continue
		}
		seen[key] = true
		stats.Expansions++
		stats.VerifierCalls++
		if amortizationApplyTraining(candidate, task.Train) {
			stats.WallMS = time.Since(start).Milliseconds()
			return candidate, stats, true
		}
	}
	stats.WallMS = time.Since(start).Milliseconds()
	return nil, stats, false
}

func amortizationProcedureFingerprint(p AcquisitionProcedure, task amortizationTask) string {
	h := sha256.New()
	b, _ := json.Marshal(p)
	h.Write(b)
	for _, ex := range task.Train {
		for _, c := range ex.Input {
			fmt.Fprintf(h, "|%s|%g|%g|", c.Mechanism, c.Resources.Compute, c.Resources.ExperimentBudget)
		}
		for _, y := range ex.Expected {
			ioWriteString(h, y)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ioWriteString(h interface{ Write([]byte) (int, error) }, s string) {
	_, _ = h.Write([]byte(s))
}

func generateAmortizationTask(seed int, methods []AcquisitionMethodArtifact) amortizationTask {
	rng := deterministicRNG(seed)
	pairs := make([][2]int, 0, len(methods)*len(methods))
	for i := range methods {
		for j := range methods {
			if i != j {
				pairs = append(pairs, [2]int{i, j})
			}
		}
	}
	target := pairs[rng.Intn(len(pairs))]
	buildExample := func(n int) abstractCompoundExample {
		in := makeAmortizationStream(rng.r, n)
		m1, _ := decodeAcquisitionProcedure(methods[target[0]].Artifact)
		m2, _ := decodeAcquisitionProcedure(methods[target[1]].Artifact)
		expected := abstractCompoundReference(m2, abstractCompoundReference(m1, in))
		return abstractCompoundExample{Input: in, Expected: abstractCompoundMechanisms(expected)}
	}
	// A fixed 12-example identification set is the smallest size that makes the
	// deterministic primitive baseline universally solve the 512-task curriculum
	// under the current procedure language; both learners receive it symmetrically.
	train := make([]abstractCompoundExample, 0, 12)
	for n := 11; n <= 22; n++ {
		train = append(train, buildExample(n))
	}
	holdout := make([]abstractCompoundExample, 0, 4)
	for n := 23; n <= 26; n++ {
		holdout = append(holdout, buildExample(n))
	}
	return amortizationTask{Train: train, Holdout: holdout, Target: []int{target[0], target[1]}}
}

func deterministicRNG(seed int) *lockedRand {
	return &lockedRand{r: newRand(seed)}
}

type lockedRand struct {
	r *pseudoRand
}

func (r *lockedRand) Intn(n int) int {
	return r.r.Intn(n)
}

type pseudoRand struct {
	state uint64
}

func newRand(seed int) *pseudoRand {
	x := uint64(seed) + 0x9e3779b97f4a7c15
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return &pseudoRand{state: x}
}

func (r *pseudoRand) next() uint64 {
	x := r.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.state = x
	return x * 2685821657736338717
}

func (r *pseudoRand) Intn(n int) int {
	if n <= 0 {
		panic("invalid Intn")
	}
	return int(r.next() % uint64(n))
}

func makeAmortizationStream(r *pseudoRand, n int) []ArchitectureCandidate {
	mechanisms := []string{"m0","m1","m2","m3","m4","m5","m6","m7","m8","m9","m10","m11"}
	perm := append([]string(nil), mechanisms...)
	for i := len(perm)-1; i > 0; i-- {
		j := r.Intn(i + 1)
		perm[i], perm[j] = perm[j], perm[i]
	}
	out := make([]ArchitectureCandidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ArchitectureCandidate{
			ID: fmt.Sprintf("%s-%d", perm[i%len(perm)], i),
			Mechanism: perm[i%len(perm)],
			Advantage: "opaque",
			Resources: ResourceVector{Compute: float64(1 + r.Intn(15)), ExperimentBudget: float64(r.Intn(6))},
		})
	}
	return out
}

func amortizationCPUTimeMS() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return int64(ru.Utime.Sec*1000 + ru.Utime.Usec/1000 + ru.Stime.Sec*1000 + ru.Stime.Usec/1000)
}

func amortizationRSS() uint64 {
	return abstractCompoundPeakRSS()
}

func amortizationWrite(path string, v any) int64 {
	return abstractCompoundWriteJSON(path, v)
}

func amortizationHashFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func procedureSignatureMust(artifact string) string {
	var p AcquisitionProcedure
	if err := json.Unmarshal([]byte(artifact), &p); err != nil {
		return ""
	}
	return procedureSignature(p)
}

func amortizationHoldoutVerify(methods []AcquisitionMethodArtifact, task amortizationTask) bool {
	got, _, ok := amortizationSelectPair(methods, task)
	if !ok || !gotOk(got) {
		return false
	}
	for _, ex := range task.Holdout {
		out, err := amortizationApplyMethods(got, ex.Input)
		if err != nil || !abstractCompoundStreamEqual(out, ex.Expected) {
			return false
		}
	}
	return true
}

func gotOk(methods []AcquisitionMethodArtifact) bool {
	return len(methods) == 2
}

func amortizationPersist(methods []AcquisitionMethodArtifact, path string) error {
	r, err := NewPersistentMethodRegistry(path)
	if err != nil {
		return err
	}
	return persistedInstall(r, methods...)
}

func amortizationRestartCheck(t *testing.T, registryPath string, task amortizationTask, helperResultPath string) bool {
	fixturePath := registryPath + ".fixture.json"
	fixture := amortizationHelperFixture{
		MethodsPath: registryPath,
		Train: task.Train,
		Input: task.Holdout[0].Input,
	}
	b, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return false
	}
	if err := os.WriteFile(fixturePath, b, 0600); err != nil {
		return false
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAutonomousAmortizationRehydrateHelper$")
	cmd.Env = append(os.Environ(),
		"ACE_AMORTIZATION_HELPER=1",
		"ACE_AMORTIZATION_FIXTURE="+fixturePath,
		"ACE_AMORTIZATION_RESULT="+helperResultPath,
	)
	return cmd.Run() == nil
}

func TestAutonomousAmortizationRehydrateHelper(t *testing.T) {
	if os.Getenv("ACE_AMORTIZATION_HELPER") != "1" {
		return
	}
	fixturePath := os.Getenv("ACE_AMORTIZATION_FIXTURE")
	resultPath := os.Getenv("ACE_AMORTIZATION_RESULT")
	b, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture amortizationHelperFixture
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	registry, err := NewPersistentMethodRegistry(fixture.MethodsPath)
	if err != nil {
		t.Fatal(err)
	}
	methods := registry.Methods()
	selected, _, ok := amortizationSelectPair(methods, amortizationTask{Train: fixture.Train})
	if !ok {
		t.Fatal("rehydrated process could not select a capability composition")
	}
	out, err := amortizationApplyMethods(selected, fixture.Input)
	if err != nil {
		t.Fatal(err)
	}
	result := amortizationHelperResult{
		Methods: []string{selected[0].ID, selected[1].ID},
		Output: abstractCompoundMechanisms(out),
	}
	rb, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, rb, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMultiHorizonAutonomousAmortization(t *testing.T) {
	if os.Getenv("ACE_AMORTIZATION_HELPER") == "1" {
		return
	}
	start := time.Now()
	cpu0 := amortizationCPUTimeMS()
	heap0, inuse0, syss0 := abstractCompoundMemory()
	methods := amortizationCapabilities()
	seeds := []int{3, 11, 23, 41, 59, 71, 83, 97}
	horizons := []int{1, 2, 4, 8, 16, 32, 64}

	global := amortizationReport{
		Commit: abstractCompoundGitHead(),
		GoVersion: runtime.Version(),
		GOOS: runtime.GOOS,
		GOARCH: runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
		Seeds: seeds,
		Horizons: horizons,
		PerSeed: make([]amortizationSeedReport, 0, len(seeds)),
		MeanRatios: map[string]float64{},
		Boundary: "This is a bounded executable-procedure amortization experiment. It does not establish open-ended abstraction, general cross-domain intelligence, recursive self-improvement, AGI, or ASI.",
	}

	for _, seed := range seeds {
		rawRoot := t.TempDir()
		capabilityMethods := make([]AcquisitionMethodArtifact, 0, len(methods))
		acqExpansions := 0
		acqVerifierCalls := 0
		retainedBytes := int64(0)
		rawTrainingBytes := int64(0)

		for _, method := range methods {
			p, err := decodeAcquisitionProcedure(method.Artifact)
			if err != nil {
				t.Fatal(err)
			}
			train, holdout := abstractCompoundTaskExamples(seed*1000+len(capabilityMethods)+1, p, 3, 2)
			learned, stats, ok := searchAndVerifyAbstractCompoundProcedure(train, holdout, 2)
			if !ok {
				t.Fatalf("seed %d: capability acquisition failed after held-out-qualified search", seed)
			}
			verified, calls := verifyAbstractCompoundProcedure(learned, train, holdout)
			if !verified {
				trainFP := amortizationProcedureFingerprint(p, amortizationTask{Train: train})
				holdoutFP := amortizationProcedureFingerprint(p, amortizationTask{Train: holdout})
				t.Fatalf("seed %d: acquired capability failed held-out verification method=%s target=%s learned=%s train_fp=%s holdout_fp=%s verifier_calls=%d", seed, method.Name, procedureSignature(p), procedureSignature(learned), trainFP, holdoutFP, calls)
			}
			rawBytes, err := json.Marshal(struct {
				Train   []abstractCompoundExample `json:"train"`
				Holdout []abstractCompoundExample `json:"holdout"`
			}{Train: train, Holdout: holdout})
			if err != nil {
				t.Fatal(err)
			}
			rawTrainingBytes += int64(len(rawBytes))
			artifact, err := learned.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			capabilityMethods = append(capabilityMethods, AcquisitionMethodArtifact{
				ID: procedureSignature(learned),
				Name: method.Name,
				Procedure: artifact,
				Artifact: artifact,
			})
			acqExpansions += stats.Expansions
			acqVerifierCalls += calls
			retainedBytes += int64(len(artifact))
		}

		tasks := make([]amortizationTask, 64)
		for i := range tasks {
			tasks[i] = generateAmortizationTask(seed*100000+i, capabilityMethods)
		}

		scratch := make([]amortizationSearchStats, len(tasks))
		retained := make([]amortizationSearchStats, len(tasks))
		scratchOK := make([]bool, len(tasks))
		retainedOK := make([]bool, len(tasks))

		for i, task := range tasks {
			s0 := time.Now()
			learned, searchStats, ok := searchAbstractCompoundProcedure(task.Train, 4)
			wallMS := time.Since(s0).Milliseconds()
			scratch[i] = amortizationSearchStats{Expansions: searchStats.Expansions, VerifierCalls: searchStats.VerifierCalls, WallMS: wallMS}
			if ok {
				scratchOK[i] = true
				for _, ex := range task.Holdout {
					out, err := executeSearchProcedure(learned, ex.Input)
					if err != nil || !abstractCompoundStreamEqual(out, ex.Expected) {
						scratchOK[i] = false
						break
					}
				}
			}
			r0 := time.Now()
			selected, stats2, ok2 := amortizationSelectPair(capabilityMethods, task)
			stats2.WallMS = time.Since(r0).Milliseconds()
			retained[i] = stats2
			if ok2 {
				retainedOK[i] = true
				for _, ex := range task.Holdout {
					out, err := amortizationApplyMethods(selected, ex.Input)
					if err != nil || !abstractCompoundStreamEqual(out, ex.Expected) {
						retainedOK[i] = false
						break
					}
				}
			}
		}

		restartPath := filepath.Join(rawRoot, "capabilities.json")
		if err := amortizationPersist(capabilityMethods, restartPath); err != nil {
			t.Fatalf("seed %d persistence failed: %v", seed, err)
		}
		restartResult := filepath.Join(rawRoot, "restart-result.json")
		restartPassed := amortizationRestartCheck(t, restartPath, tasks[63], restartResult)
		if restartPassed {
			b, err := os.ReadFile(restartResult)
			if err != nil {
				restartPassed = false
			} else {
				var result amortizationHelperResult
				if json.Unmarshal(b, &result) != nil || len(result.Output) == 0 {
					restartPassed = false
				} else {
					for _, ex := range tasks[63].Holdout[:1] {
						if !abstractCompoundStreamEqual(stringSliceCandidates(result.Output), ex.Expected) {
							restartPassed = false
						}
					}
				}
			}
		}

		originalHash := amortizationHashFile(restartPath)
		tamperedPath := filepath.Join(rawRoot, "capabilities-tampered.json")
		orig, err := os.ReadFile(restartPath)
		if err != nil {
			t.Fatal(err)
		}
		tampered := strings.Replace(string(orig), "sort-cost", "identity", 1)
		if tampered == string(orig) {
			tampered = string(orig) + "\n"
		}
		if err := os.WriteFile(tamperedPath, []byte(tampered), 0600); err != nil {
			t.Fatal(err)
		}
		runtimeTamperRejected := false
		if registry, err := NewPersistentMethodRegistry(tamperedPath); err == nil {
			dst := InstalledMethodRegistry{}
			runtimeTamperRejected = registry.Restore(&dst) != nil
		}
		manifestTamperDetected := originalHash != amortizationHashFile(tamperedPath)

		rawMemoryBytes := rawTrainingBytes
		compressedMemoryBytes := int64(0)
		for _, method := range capabilityMethods {
			compressedMemoryBytes += int64(len(procedureSignatureMust(method.Artifact)))
		}
		persistentBytes := abstractCompoundFileBytes(rawRoot)
		perSeed := amortizationSeedReport{
			Seed: seed,
			AcquisitionExpansions: acqExpansions,
			AcquisitionVerifierCalls: acqVerifierCalls,
			RetainedBytes: retainedBytes,
			RawMemoryBytes: rawMemoryBytes,
			CompressedMemoryBytes: compressedMemoryBytes,
			ProcessRestartPassed: restartPassed,
			RuntimeTamperRejected: runtimeTamperRejected,
			ManifestTamperDetected: manifestTamperDetected,
			PersistentBytes: persistentBytes,
			ArtifactBytes: retainedBytes,
			Horizons: make([]amortizationHorizon, 0, len(horizons)),
		}

		for _, n := range horizons {
			var se, sv, re, rv int
			var sw, rw int64
			scratchSuccess := 0
			retainedSuccess := 0
			for i := 0; i < n; i++ {
				se += scratch[i].Expansions
				sv += scratch[i].VerifierCalls
				sw += scratch[i].WallMS
				re += retained[i].Expansions
				rv += retained[i].VerifierCalls
				rw += retained[i].WallMS
				if scratchOK[i] {
					scratchSuccess++
				}
				if retainedOK[i] {
					retainedSuccess++
				}
			}
			scratchLifetime := se
			retainedLifetime := acqExpansions + re
			ratio := 0.0
			if scratchLifetime > 0 {
				ratio = float64(retainedLifetime) / float64(scratchLifetime)
			}
			perSeed.Horizons = append(perSeed.Horizons, amortizationHorizon{
				N: n, Tasks: n,
				ScratchSuccess: scratchSuccess,
				RetainedSuccess: retainedSuccess,
				ScratchExpansions: se,
				ScratchVerifierCalls: sv,
				RetainedSearchExpansions: re,
				RetainedVerifierCalls: rv,
				ScratchWallMS: sw,
				RetainedWallMS: rw,
				ScratchLifetimeCost: scratchLifetime,
				RetainedLifetimeCost: retainedLifetime,
				CostRatio: ratio,
			})
		}
		global.PerSeed = append(global.PerSeed, perSeed)
	}

	for _, n := range horizons {
		key := strconv.Itoa(n)
		sum := 0.0
		count := 0
		allBeat := true
		for _, seed := range global.PerSeed {
			for _, h := range seed.Horizons {
				if h.N != n {
					continue
				}
				sum += h.CostRatio
				count++
				if h.CostRatio >= 1.0 {
					allBeat = false
				}
			}
		}
		if count > 0 {
			global.MeanRatios[key] = sum / float64(count)
		}
		if global.CrossoverN == 0 && allBeat {
			global.CrossoverN = n
		}
	}

	global.AllRetainedBeatScratch = global.CrossoverN != 0
	global.AllTasksSolvedScratch = true
	global.AllTasksSolvedRetained = true
	restartPassed := 0
	runtimeTamperRejected := 0
	manifestTamperDetected := 0
	for _, seed := range global.PerSeed {
		if seed.ProcessRestartPassed {
			restartPassed++
		}
		if seed.RuntimeTamperRejected {
			runtimeTamperRejected++
		}
		if seed.ManifestTamperDetected {
			manifestTamperDetected++
		}
		for _, h := range seed.Horizons {
			if h.ScratchSuccess != h.Tasks {
				global.AllTasksSolvedScratch = false
			}
			if h.RetainedSuccess != h.Tasks {
				global.AllTasksSolvedRetained = false
			}
		}
	}
	global.RestartPassRate = float64(restartPassed) / float64(len(global.PerSeed))
	global.RuntimeTamperRejectRate = float64(runtimeTamperRejected) / float64(len(global.PerSeed))
	global.ManifestTamperDetectRate = float64(manifestTamperDetected) / float64(len(global.PerSeed))
	switch {
	case !global.AllTasksSolvedScratch:
		global.Classification = "AMORTIZATION_EXPERIMENT_INVALID_SCRATCH_SOLVER_FAILED_FUTURE_TASKS"
	case !global.AllTasksSolvedRetained:
		global.Classification = "AMORTIZATION_EXPERIMENT_INVALID_RETAINED_SOLVER_FAILED_FUTURE_TASKS"
	case !global.AllRetainedBeatScratch:
		global.Classification = "CAPABILITY_REUSE_REDUCES_FUTURE_SEARCH_BUT_LIFETIME_AMORTIZATION_NOT_DEMONSTRATED"
	case global.RuntimeTamperRejectRate < 1.0:
		global.Classification = "CAPABILITY_AMORTIZATION_DEMONSTRATED_BUT_RUNTIME_TAMPER_INTEGRITY_FAILED"
	default:
		global.Classification = "MULTI_HORIZON_CAPABILITY_AMORTIZATION_DEMONSTRATED"
	}

	elapsed := time.Since(start)
	global.WallTimeMS = elapsed.Milliseconds()
	global.CPUTimeMS = amortizationCPUTimeMS() - cpu0
	heap1, inuse1, syss1 := abstractCompoundMemory()
	global.HeapAllocDeltaBytes = heap1 - heap0
	global.HeapInuseDeltaBytes = inuse1 - inuse0
	global.HeapSysDeltaBytes = syss1 - syss0
	global.PeakRSSBytes = amortizationRSS()
	global.PersistentBytes = 0
	global.ArtifactBytes = 0

	if workspace := os.Getenv("GITHUB_WORKSPACE"); workspace != "" {
		rawRoot := filepath.Join(workspace, "ACE_MULTI_HORIZON_AMORTIZATION")
		_ = os.MkdirAll(rawRoot, 0755)
		global.PersistentBytes = int64(len(global.PerSeed))
		global.ArtifactBytes = amortizationWrite(filepath.Join(workspace, "ACE_MULTI_HORIZON_AMORTIZATION.json"), global)
		_ = os.WriteFile(filepath.Join(rawRoot, "README.txt"), []byte("Evaluator-generated amortization trial data is recorded in the parent JSON report.\n"), 0644)
	}
	t.Logf("ACE_MULTI_HORIZON_AMORTIZATION classification=%s crossover_n=%d mean_ratios=%v restart_pass_rate=%.3f runtime_tamper_reject_rate=%.3f manifest_tamper_detect_rate=%.3f wall_ms=%d cpu_ms=%d peak_rss_bytes=%d",
		global.Classification, global.CrossoverN, global.MeanRatios, global.RestartPassRate, global.RuntimeTamperRejectRate, global.ManifestTamperDetectRate,
		global.WallTimeMS, global.CPUTimeMS, global.PeakRSSBytes)

	if !global.AllTasksSolvedScratch || !global.AllTasksSolvedRetained {
		t.Fatal("amortization task family is not solved by both required learners")
	}
}

func stringSliceCandidates(xs []string) []ArchitectureCandidate {
	out := make([]ArchitectureCandidate, len(xs))
	for i, x := range xs {
		out[i] = ArchitectureCandidate{Mechanism: x}
	}
	return out
}
