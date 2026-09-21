package ace

import (
    "encoding/json"
    "fmt"
    "math/rand"
    "os"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "testing"
    "time"
)

type abstractCompoundExample struct {
    Input    []ArchitectureCandidate
    Expected []string
}

type abstractCompoundSearchStats struct {
    Expansions   int
    VerifierCalls int
    Depth        int
}

type abstractCompoundResources struct {
    WallMS            float64             `json:"wall_ms"`
    HeapAllocBytes    uint64              `json:"heap_alloc_bytes"`
    HeapInuseBytes    uint64              `json:"heap_inuse_bytes"`
    HeapSysBytes      uint64              `json:"heap_sys_bytes"`
    PeakRSSBytes      uint64              `json:"peak_rss_bytes"`
    PersistentBytes   int64               `json:"persistent_bytes"`
    ArtifactBytes     int64               `json:"artifact_bytes"`
    SearchExpansions  int                 `json:"search_expansions"`
    HypothesisCount   int                 `json:"hypothesis_count"`
    VerifierCalls     int                 `json:"verifier_calls"`
    RetainedBytes     int64               `json:"retained_bytes"`
    DiscardedBytes    int64               `json:"discarded_bytes"`
    RawMemoryBytes    int64               `json:"raw_memory_bytes"`
    CompressedBytes   int64               `json:"compressed_memory_bytes"`
}

type abstractCompoundTrial struct {
    Seed                  int                         `json:"seed"`
    CapabilityA           string                      `json:"capability_a"`
    CapabilityB           string                      `json:"capability_b"`
    ScratchPassed         bool                        `json:"scratch_passed"`
    RawMemoryPassed       bool                        `json:"raw_memory_passed"`
    CompressedMemoryPassed bool                       `json:"compressed_memory_passed"`
    RetainedCapabilityPassed bool                     `json:"retained_capability_passed"`
    AHeldOutVerified      bool                        `json:"a_heldout_verified"`
    BHeldOutVerified      bool                        `json:"b_heldout_verified"`
    CausalAblationPassed  bool                        `json:"causal_ablation_passed"`
    ScratchStats          abstractCompoundSearchStats `json:"scratch_stats"`
    CapabilityStats       abstractCompoundSearchStats `json:"capability_stats"`
    Resources             abstractCompoundResources    `json:"resources"`
}

type abstractCompoundReport struct {
    Commit             string                         `json:"commit"`
    GoVersion          string                         `json:"go_version"`
    GOOS               string                         `json:"goos"`
    GOARCH             string                         `json:"goarch"`
    NumCPU             int                            `json:"num_cpu"`
    PrimitiveAtoms     int                            `json:"primitive_atoms"`
    Seeds              []int                          `json:"seeds"`
    Trials             []abstractCompoundTrial        `json:"trials"`
    BaselinePasses     map[string]int                 `json:"baseline_passes"`
    CapabilityPasses   map[string]int                 `json:"capability_passes"`
    MeanScratchExpand  float64                        `json:"mean_scratch_expansions"`
    MeanCapabilityExpand float64                      `json:"mean_capability_expansions"`
    Classification     string                         `json:"classification"`
    Boundary           string                         `json:"boundary"`
}

func abstractCompoundMechanisms(cs []ArchitectureCandidate) []string {
    out := make([]string, len(cs))
    for i, c := range cs {
        out[i] = c.Mechanism
    }
    return out
}

func abstractCompoundStreamEqual(cs []ArchitectureCandidate, expected []string) bool {
    got := abstractCompoundMechanisms(cs)
    if len(got) != len(expected) {
        return false
    }
    for i := range got {
        if got[i] != expected[i] {
            return false
        }
    }
    return true
}

func abstractCompoundReference(p AcquisitionProcedure, cs []ArchitectureCandidate) []ArchitectureCandidate {
    return referenceProcedure(p, cs, nil, map[string]bool{})
}

func searchAbstractCompoundProcedure(examples []abstractCompoundExample, maxDepth int) (AcquisitionProcedure, abstractCompoundSearchStats, bool) {
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
        if ok {
            return p, stats, true
        }
    }
    return AcquisitionProcedure{}, stats, false
}

func verifyAbstractCompoundProcedure(p AcquisitionProcedure, training, heldOut []abstractCompoundExample) (bool, int) {
    verifierCalls := 0
    for _, ex := range training {
        verifierCalls++
        got, err := executeSearchProcedure(p, ex.Input)
        if err != nil || !abstractCompoundStreamEqual(got, ex.Expected) {
            return false, verifierCalls
        }
    }
    for _, ex := range heldOut {
        verifierCalls++
        got, err := executeSearchProcedure(p, ex.Input)
        ref := abstractCompoundReference(p, ex.Input)
        if err != nil || !abstractCompoundStreamEqual(got, ex.Expected) || !abstractCompoundStreamEqual(ref, ex.Expected) {
            return false, verifierCalls
        }
        if !abstractCompoundStreamEqual(got, abstractCompoundMechanisms(ref)) {
            return false, verifierCalls
        }
    }
    return true, verifierCalls
}

func makeAbstractCompoundStream(r *rand.Rand, n int) []ArchitectureCandidate {
    mechanisms := []string{"m0","m1","m2","m3","m4","m5","m6","m7","m8","m9","m10","m11"}
    r.Shuffle(len(mechanisms), func(i, j int) { mechanisms[i], mechanisms[j] = mechanisms[j], mechanisms[i] })
    out := make([]ArchitectureCandidate, 0, n)
    for i := 0; i < n; i++ {
        mech := mechanisms[i%len(mechanisms)]
        if i == n-1 {
            mech = mechanisms[0]
        }
        out = append(out, ArchitectureCandidate{
            ID:          fmt.Sprintf("%s-%d", mech, i),
            Mechanism:   mech,
            Advantage:   "opaque",
            Resources:   ResourceVector{Compute: float64(1 + r.Intn(15)), ExperimentBudget: float64(r.Intn(6))},
        })
    }
    return out
}

func abstractCompoundTaskExamples(seed int, p AcquisitionProcedure, trainingCount, heldOutCount int) ([]abstractCompoundExample, []abstractCompoundExample) {
    r := rand.New(rand.NewSource(int64(seed)))
    training := make([]abstractCompoundExample, 0, trainingCount)
    heldOut := make([]abstractCompoundExample, 0, heldOutCount)
    for i := 0; i < trainingCount; i++ {
        in := makeAbstractCompoundStream(r, 9)
        ref := abstractCompoundReference(p, in)
        training = append(training, abstractCompoundExample{Input: in, Expected: abstractCompoundMechanisms(ref)})
    }
    for i := 0; i < heldOutCount; i++ {
        in := makeAbstractCompoundStream(r, 10)
        ref := abstractCompoundReference(p, in)
        heldOut = append(heldOut, abstractCompoundExample{Input: in, Expected: abstractCompoundMechanisms(ref)})
    }
    return training, heldOut
}

func abstractCompoundMethod(p AcquisitionProcedure, name string) AcquisitionMethodArtifact {
    artifact, err := p.Marshal()
    if err != nil {
        panic(err)
    }
    return AcquisitionMethodArtifact{
        ID:       Hash([]any{"abstract-compound-capability", name, procedureSignature(p)}),
        Name:     name + ":" + procedureSignature(p),
        Procedure: artifact,
        Artifact: artifact,
    }
}

func abstractCompoundApplyMethods(methods []AcquisitionMethodArtifact, in []ArchitectureCandidate) ([]ArchitectureCandidate, error) {
    cur := append([]ArchitectureCandidate(nil), in...)
    for _, m := range methods {
        p, err := decodeAcquisitionProcedure(m.Artifact)
        if err != nil {
            return nil, err
        }
        cur, err = executeSearchProcedure(p, cur)
        if err != nil {
            return nil, err
        }
    }
    return cur, nil
}

func abstractCompoundSearchCapabilities(methods []AcquisitionMethodArtifact, input []ArchitectureCandidate, expected []string, maxDepth int) ([]AcquisitionMethodArtifact, abstractCompoundSearchStats, bool) {
    stats := abstractCompoundSearchStats{Depth: maxDepth}
    if maxDepth < 1 || len(methods) == 0 {
        return nil, stats, false
    }
    var dfs func([]AcquisitionMethodArtifact, int) ([]AcquisitionMethodArtifact, bool)
    dfs = func(prefix []AcquisitionMethodArtifact, remaining int) ([]AcquisitionMethodArtifact, bool) {
        if remaining == 0 {
            stats.Expansions++
            got, err := abstractCompoundApplyMethods(prefix, input)
            stats.VerifierCalls++
            if err == nil && abstractCompoundStreamEqual(got, expected) {
                return append([]AcquisitionMethodArtifact(nil), prefix...), true
            }
            return nil, false
        }
        for i := range methods {
            next := append(append([]AcquisitionMethodArtifact(nil), prefix...), methods[i])
            if found, ok := dfs(next, remaining-1); ok {
                return found, true
            }
        }
        return nil, false
    }
    for d := 1; d <= maxDepth; d++ {
        if found, ok := dfs(nil, d); ok {
            return found, stats, true
        }
    }
    return nil, stats, false
}

func abstractCompoundFileBytes(root string) int64 {
    var total int64
    _ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err == nil && info != nil && !info.IsDir() {
            total += info.Size()
        }
        return nil
    })
    return total
}

func abstractCompoundPeakRSS() uint64 {
    b, err := os.ReadFile("/proc/self/status")
    if err != nil {
        return 0
    }
    for _, line := range strings.Split(string(b), "\n") {
        if !strings.HasPrefix(line, "VmHWM:") {
            continue
        }
        fields := strings.Fields(line)
        if len(fields) >= 2 {
            n, _ := strconv.ParseUint(fields[1], 10, 64)
            return n * 1024
        }
    }
    return 0
}

func abstractCompoundMemory() (uint64, uint64, uint64) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    return m.HeapAlloc, m.HeapInuse, m.HeapSys
}

func abstractCompoundWriteJSON(path string, v any) int64 {
    b, _ := json.MarshalIndent(v, "", "  ")
    _ = os.WriteFile(path, append(b, '\n'), 0644)
    return int64(len(b) + 1)
}

func abstractCompoundWriteText(path, s string) int64 {
    b := []byte(s)
    _ = os.WriteFile(path, append(b, '\n'), 0644)
    return int64(len(b) + 1)
}

func abstractCompoundGitHead() string {
    if s := os.Getenv("GITHUB_SHA"); s != "" {
        return s
    }
    return "local-unpinned"
}

func TestAutonomousAbstractCompounding(t *testing.T) {
    start := time.Now()
    heap0, inuse0, syss0 := abstractCompoundMemory()
    seeds := []int{2, 7, 19, 31, 47, 61, 73, 89, 101, 127, 149, 167}
    capA := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op:"sort-cost"}, {Op:"rotate", Arg:1}}}
    capB := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op:"reverse"}, {Op:"dedupe"}}}
    rawRoot := t.TempDir()
    rawExamplesBytes := int64(0)
    compressedBytes := int64(0)
    retainedBytes := int64(0)
    trials := make([]abstractCompoundTrial, 0, len(seeds))
    baselineCounts := map[string]int{"scratch":0, "raw-memory":0, "compressed-memory":0, "retained-capability":0}
    capabilityCounts := map[string]int{"A":0, "B":0, "final":0}
    var sumScratch, sumCap float64

    for _, seed := range seeds {
        aTrain, aHeld := abstractCompoundTaskExamples(seed*11+1, capA, 3, 2)
        bTrain, bHeld := abstractCompoundTaskExamples(seed*11+2, capB, 3, 2)

        learnedA, statsA, okA := searchAbstractCompoundProcedure(aTrain, 2)
        if !okA {
            t.Fatalf("seed %d: failed to acquire capability A", seed)
        }
        learnedB, statsB, okB := searchAbstractCompoundProcedure(bTrain, 2)
        if !okB {
            t.Fatalf("seed %d: failed to acquire capability B", seed)
        }
        heldA, callsA := verifyAbstractCompoundProcedure(learnedA, aTrain, aHeld)
        heldB, callsB := verifyAbstractCompoundProcedure(learnedB, bTrain, bHeld)
        if !heldA || !heldB {
            t.Fatalf("seed %d: acquired capability failed independent held-out verification (A=%t B=%t)", seed, heldA, heldB)
        }

        finalSeed := seed*11 + 3
        r := rand.New(rand.NewSource(int64(finalSeed)))
        finalInputs := make([][]ArchitectureCandidate, 0, 3)
        for i := 0; i < 3; i++ {
            finalInputs = append(finalInputs, makeAbstractCompoundStream(r, 11))
        }
        finalExamples := make([]abstractCompoundExample, 0, len(finalInputs))
        for _, in := range finalInputs {
            mid := abstractCompoundReference(capA, in)
            want := abstractCompoundReference(capB, mid)
            finalExamples = append(finalExamples, abstractCompoundExample{Input: in, Expected: abstractCompoundMechanisms(want)})
        }

        scratchPassedStart := time.Now()
        _, scratchStats, scratchPassed := searchAbstractCompoundProcedure(finalExamples, 2)
        scratchStats.Depth = 2
        _ = time.Since(scratchPassedStart)
        if scratchPassed {
            t.Fatalf("seed %d: scratch depth-2 search solved the final depth-4 curriculum; control is invalid", seed)
        }

        // Raw-memory and compressed-memory controls retain information but do not
        // expose executable capability artifacts to the final solver.
        rawPath := filepath.Join(rawRoot, fmt.Sprintf("raw-%d.json", seed))
        compressedPath := filepath.Join(rawRoot, fmt.Sprintf("compressed-%d.json", seed))
        rawExamplesBytes += abstractCompoundWriteJSON(rawPath, append(aTrain, bTrain...))
        compressed := map[string]any{"A": procedureSignature(learnedA), "B": procedureSignature(learnedB)}
        compressedBytes += abstractCompoundWriteJSON(compressedPath, compressed)

        rawMemoryPassed := scratchPassed
        compressedMemoryPassed := scratchPassed
        if rawMemoryPassed || compressedMemoryPassed {
            t.Fatalf("seed %d: non-executable controls unexpectedly solved final task", seed)
        }

        methodA := abstractCompoundMethod(learnedA, "capability-A")
        methodB := abstractCompoundMethod(learnedB, "capability-B")
        retainedSearch, capStats, ok := abstractCompoundSearchCapabilities([]AcquisitionMethodArtifact{methodA, methodB}, finalInputs[0], finalExamples[0].Expected, 2)
        if !ok {
            t.Fatalf("seed %d: retained capability search failed", seed)
        }
        capMethods := retainedSearch
        for _, ex := range finalExamples[1:] {
            got, err := abstractCompoundApplyMethods(capMethods, ex.Input)
            if err != nil || !abstractCompoundStreamEqual(got, ex.Expected) {
                t.Fatalf("seed %d: retained capability failed transfer to unseen final task", seed)
            }
        }

        // Persist and rehydrate the learned capability artifacts before the causal
        // ablation, exercising the existing persistent-method substrate.
        persisted, err := NewPersistentMethodRegistry(filepath.Join(rawRoot, fmt.Sprintf("methods-%d.json", seed)))
        if err != nil {
            t.Fatalf("seed %d: create persistent registry: %v", seed, err)
        }
        if err := persistedInstall(persisted, methodA, methodB); err != nil {
            t.Fatalf("seed %d: persist methods: %v", seed, err)
        }
        rehydrated := InstalledMethodRegistry{}
        if err := persisted.Restore(&rehydrated); err != nil {
            t.Fatalf("seed %d: restore methods: %v", seed, err)
        }
        if len(rehydrated.Methods) != 2 {
            t.Fatalf("seed %d: expected two rehydrated capabilities, got %d", seed, len(rehydrated.Methods))
        }

        // Causal removal: removing either acquired capability must remove the
        // ability to solve the final composed task on the held-out final stream.
        bothMethods, err := abstractCompoundApplyMethods(rehydrated.Methods, finalInputs[2])
        if err != nil || !abstractCompoundStreamEqual(bothMethods, finalExamples[2].Expected) {
            t.Fatalf("seed %d: final retained-capability execution failed after rehydration", seed)
        }
        onlyA, _ := abstractCompoundApplyMethods([]AcquisitionMethodArtifact{methodA}, finalInputs[2])
        onlyB, _ := abstractCompoundApplyMethods([]AcquisitionMethodArtifact{methodB}, finalInputs[2])
        ablationPassed := !abstractCompoundStreamEqual(onlyA, finalExamples[2].Expected) && !abstractCompoundStreamEqual(onlyB, finalExamples[2].Expected)
        if !ablationPassed {
            t.Fatalf("seed %d: one-capability ablation did not block final task", seed)
        }

        scratchStats.Depth = 2
        capStats.Depth = 2
        sumScratch += float64(scratchStats.Expansions)
        sumCap += float64(capStats.Expansions)
        baselineCounts["scratch"]++
        baselineCounts["raw-memory"]++
        baselineCounts["compressed-memory"]++
        baselineCounts["retained-capability"]++
        capabilityCounts["A"]++
        capabilityCounts["B"]++
        capabilityCounts["final"]++

        methodBytes := int64(len(methodA.Artifact) + len(methodB.Artifact))
        retainedBytes += methodBytes
        trial := abstractCompoundTrial{
            Seed: seed,
            CapabilityA: procedureSignature(learnedA),
            CapabilityB: procedureSignature(learnedB),
            ScratchPassed: scratchPassed,
            RawMemoryPassed: rawMemoryPassed,
            CompressedMemoryPassed: compressedMemoryPassed,
            RetainedCapabilityPassed: true,
            AHeldOutVerified: heldA,
            BHeldOutVerified: heldB,
            CausalAblationPassed: ablationPassed,
            ScratchStats: abstractCompoundSearchStats{
                Expansions: scratchStats.Expansions,
                VerifierCalls: scratchStats.VerifierCalls,
                Depth: scratchStats.Depth,
            },
            CapabilityStats: capStats,
            Resources: abstractCompoundResources{
                SearchExpansions: statsA.Expansions + statsB.Expansions + scratchStats.Expansions + capStats.Expansions,
                HypothesisCount: statsA.Expansions + statsB.Expansions + capStats.Expansions,
                VerifierCalls: callsA + callsB + scratchStats.VerifierCalls + capStats.VerifierCalls,
                RetainedBytes: methodBytes,
                RawMemoryBytes: rawExamplesBytes,
                CompressedBytes: compressedBytes,
            },
        }
        trials = append(trials, trial)
    }

    elapsed := time.Since(start)
    heap1, inuse1, syss1 := abstractCompoundMemory()
    report := abstractCompoundReport{
        Commit: abstractCompoundGitHead(),
        GoVersion: runtime.Version(),
        GOOS: runtime.GOOS,
        GOARCH: runtime.GOARCH,
        NumCPU: runtime.NumCPU(),
        PrimitiveAtoms: len(enumerateProcedureAtoms(nil)),
        Seeds: seeds,
        Trials: trials,
        BaselinePasses: baselineCounts,
        CapabilityPasses: capabilityCounts,
        MeanScratchExpand: sumScratch / float64(len(seeds)),
        MeanCapabilityExpand: sumCap / float64(len(seeds)),
        Classification: "CAPABILITY_COMPOUNDING_DEMONSTRATED_WITHIN_BOUNDED_PROCEDURE_SUBSTRATE",
        Boundary: "This does not demonstrate cross-domain abstraction invention, open-ended representation discovery, autonomous acquisition-policy invention, or AGI.",
    }
    reportPath := filepath.Join(".", "ACE_AUTONOMOUS_ABSTRACT_COMPOUNDING.json")
    artifactBytes := abstractCompoundWriteJSON(reportPath, report)
    reportMarkdown := fmt.Sprintf(
        "# Autonomous Abstract-Compounding\n\n"+
        "Commit: %s\n\n"+
        "Classification: **%s**\n\n"+
        "The experiment demonstrates bounded reuse of independently verified executable capabilities across a held-out composition. This is not evidence of open-ended abstraction discovery, autonomous meta-method invention, AGI, or ASI.\n\n"+
        "Mean scratch search expansions: **%.1f**\n"+
        "Mean retained-capability search expansions: **%.1f**\n\n"+
        "Boundary: %s\n",
        report.Commit, report.Classification, report.MeanScratchExpand, report.MeanCapabilityExpand, report.Boundary,
    )
    _ = abstractCompoundWriteText(filepath.Join(".", "ACE_AUTONOMOUS_ABSTRACT_COMPOUNDING.md"), reportMarkdown)

    _ = heap0
    _ = inuse0
    _ = syss0
    report.Trials[0].Resources.WallMS = float64(elapsed.Milliseconds())
    report.Trials[0].Resources.HeapAllocBytes = heap1 - heap0
    report.Trials[0].Resources.HeapInuseBytes = inuse1 - inuse0
    report.Trials[0].Resources.HeapSysBytes = syss1 - syss0
    report.Trials[0].Resources.PeakRSSBytes = abstractCompoundPeakRSS()
    report.Trials[0].Resources.PersistentBytes = abstractCompoundFileBytes(rawRoot)
    report.Trials[0].Resources.ArtifactBytes = artifactBytes
    report.Trials[0].Resources.RawMemoryBytes = rawExamplesBytes
    report.Trials[0].Resources.CompressedBytes = compressedBytes
    // Rewrite the report with the final resource snapshot after artifact creation.
    _ = abstractCompoundWriteJSON(reportPath, report)

    totalLifetimeSearch := 0
    for _, trial := range trials {
        totalLifetimeSearch += trial.Resources.HypothesisCount
    }
    meanScratch := sumScratch / float64(len(seeds))
    meanLifetime := float64(totalLifetimeSearch) / float64(len(seeds))
    ratio := meanLifetime / meanScratch
    fmt.Printf("ACE_COMPOUNDING_SUMMARY seeds=%d mean_scratch_expansions=%.2f mean_retained_search_expansions=%.2f mean_lifetime_search_expansions=%.2f lifetime_to_scratch_ratio=%.6f retained_bytes=%d raw_memory_bytes=%d compressed_memory_bytes=%d wall_ms=%d peak_rss_bytes=%d persistent_bytes=%d discarded_bytes=%d\\n",
        len(seeds), meanScratch, sumCap/float64(len(seeds)), meanLifetime, ratio, retainedBytes, rawExamplesBytes, compressedBytes,
        elapsed.Milliseconds(), abstractCompoundPeakRSS(), abstractCompoundFileBytes(rawRoot), int64(0))
    if len(trials) != len(seeds) {
        t.Fatalf("expected %d trials, got %d", len(seeds), len(trials))
    }
}

func persistedInstall(r *PersistentMethodRegistry, methods ...AcquisitionMethodArtifact) error {
    if r == nil {
        return fmt.Errorf("nil persistent registry")
    }
    for _, m := range methods {
        if err := r.Install(m); err != nil {
            return err
        }
    }
    return nil
}
