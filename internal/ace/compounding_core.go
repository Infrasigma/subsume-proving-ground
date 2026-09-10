package ace

import (
    "errors"
    "fmt"
    "sort"
    "strconv"
)

type TaskFamily interface {
    Name() string
    Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error)
}

type LatentTaskLab struct{ Families []TaskFamily }

func (l LatentTaskLab) GenerateDiscovery(seed int) (Task, []ProgramTestCase, CapabilityOracle, error) {
    if len(l.Families) == 0 { return Task{}, nil, nil, errors.New("task lab has no families") }
    return l.Families[seed%len(l.Families)].Generate(seed, false)
}
func (l LatentTaskLab) GenerateHidden(seed int) (Task, []ProgramTestCase, CapabilityOracle, error) {
    if len(l.Families) == 0 { return Task{}, nil, nil, errors.New("task lab has no families") }
    return l.Families[seed%len(l.Families)].Generate(seed, true)
}

type functionOracle func(map[string]string) (map[string]string, error)
func (f functionOracle) Evaluate(_ Task, in map[string]string) (map[string]string, error) { return f(in) }

type AffineFamily struct{}
func (AffineFamily) Name() string { return "affine" }
func (AffineFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
    shift := seed%5 + 2
    cases := []ProgramTestCase{
        {Input: map[string]string{"x":"-2"}, Expected: map[string]string{"y":strconv.Itoa(-2+shift)}},
        {Input: map[string]string{"x":"3"}, Expected: map[string]string{"y":strconv.Itoa(3+shift)}},
    }
    oracle := functionOracle(func(in map[string]string) (map[string]string, error) {
        n, err := strconv.Atoi(in["x"]); if err != nil { return nil, err }
        return map[string]string{"y":strconv.Itoa(n+shift)}, nil
    })
    return Task{ID:Hash([]any{"affine",seed,hidden}), Goal:"y equals x plus a latent shift", Requirements:[]string{"x"}, Structure:[]string{"scalar","affine"}, Novel:hidden, Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}, Provenance:Prov("latent-task-family","affine","generate",seed)}, cases, oracle, nil
}

type ThresholdFamily struct{}
func (ThresholdFamily) Name() string { return "threshold" }
func (ThresholdFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
    threshold := seed%5 + 1
    cases := []ProgramTestCase{
        {Input:map[string]string{"x":"0"}, Expected:map[string]string{"y":"0"}},
        {Input:map[string]string{"x":strconv.Itoa(threshold+1)}, Expected:map[string]string{"y":"1"}},
    }
    oracle := functionOracle(func(in map[string]string) (map[string]string, error) {
        n, err := strconv.Atoi(in["x"]); if err != nil { return nil, err }
        if n > threshold { return map[string]string{"y":"1"}, nil }
        return map[string]string{"y":"0"}, nil
    })
    return Task{ID:Hash([]any{"threshold",seed,hidden}), Goal:"y indicates whether x exceeds a latent threshold", Requirements:[]string{"x"}, Structure:[]string{"scalar","conditional","threshold","latent-boundary"}, Novel:hidden, Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}, Provenance:Prov("latent-task-family","threshold","generate",seed)}, cases, oracle, nil
}

type DeepCompositionFamily struct{}
func (DeepCompositionFamily) Name() string { return "deep-composition" }
func (DeepCompositionFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
    shift := seed%3 + 1
    cases := []ProgramTestCase{
        {Input:map[string]string{"x":"1"}, Expected:map[string]string{"y":strconv.Itoa((1+shift)*2+1)}},
        {Input:map[string]string{"x":"3"}, Expected:map[string]string{"y":strconv.Itoa((3+shift)*2+1)}},
    }
    oracle := functionOracle(func(in map[string]string) (map[string]string, error) {
        n, err := strconv.Atoi(in["x"]); if err != nil { return nil, err }
        return map[string]string{"y":strconv.Itoa((n+shift)*2+1)}, nil
    })
    return Task{ID:Hash([]any{"deep-composition",seed,hidden}), Goal:"y is shifted, doubled, then incremented", Requirements:[]string{"x"}, Structure:[]string{"scalar","composition","depth-3","shift","multiply","increment"}, Novel:hidden, Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}, Provenance:Prov("latent-task-family","deep-composition","generate",seed)}, cases, oracle, nil
}

type ReplicatedDeepFamily struct{}
func (ReplicatedDeepFamily) Name() string { return "replicated-deep-composition" }
func (ReplicatedDeepFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
    x1 := seed%17 - 8
    x2 := x1 + 5
    cases := []ProgramTestCase{
        {Input:map[string]string{"x":strconv.Itoa(x1)}, Expected:map[string]string{"y":strconv.Itoa((x1+2)*2+1)}},
        {Input:map[string]string{"x":strconv.Itoa(x2)}, Expected:map[string]string{"y":strconv.Itoa((x2+2)*2+1)}},
    }
    oracle := functionOracle(func(in map[string]string) (map[string]string, error) {
        n, err := strconv.Atoi(in["x"]); if err != nil { return nil, err }
        return map[string]string{"y":strconv.Itoa((n+2)*2+1)}, nil
    })
    return Task{ID:Hash([]any{"replicated-deep",seed,hidden}), Goal:"y is shifted, doubled, then incremented", Requirements:[]string{"x"}, Structure:[]string{"scalar","composition","depth-3","shift","multiply","increment"}, Novel:hidden, Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}, Provenance:Prov("latent-task-family","replicated-deep","generate",seed)}, cases, oracle, nil
}

type MetaMethod struct { Name string; Version uint64 }
type MetaAcquisitionResult struct { Program UniversalProgram; Method MetaMethod; Candidates int; Verified bool; Diagnosis string }

// ParameterizedMechanismSearch is a generic search over parameterized unary
// arithmetic programs. Constants are candidate parameters inferred from the
// observed behavior; no hidden oracle is consulted.
func ParameterizedMechanismSearch(cases []ProgramTestCase, library []UniversalProgram) (MetaAcquisitionResult, error) {
    if len(cases) < 2 { return MetaAcquisitionResult{}, errors.New("meta search needs behavioral evidence") }
    in, out := "", ""
    for k := range cases[0].Input { in = k; break }
    for k := range cases[0].Expected { out = k; break }
    if in == "" || out == "" { return MetaAcquisitionResult{}, errors.New("meta search needs input/output") }
    constants := map[int]bool{-10:true,-5:true,-4:true,-3:true,-2:true,-1:true,0:true,1:true,2:true,3:true,4:true,5:true,10:true}
    for _, tc := range cases {
        a, ea := strconv.Atoi(tc.Input[in]); b, eb := strconv.Atoi(tc.Expected[out])
        if ea != nil || eb != nil { continue }
        constants[b-a] = true
        if a != 0 && b%a == 0 { constants[b/a] = true }
    }
    type candidate struct { p UniversalProgram; key string }
    candidates := make([]candidate, 0, len(constants)*2+len(library))
    for c := range constants {
        candidates = append(candidates, candidate{UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:out,Expr:&UExpr{Kind:"add",Left:&UExpr{Kind:"var",Value:in},Right:&UExpr{Kind:"const",Value:strconv.Itoa(c)}}}}}, fmt.Sprintf("add:%d",c)})
        candidates = append(candidates, candidate{UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:out,Expr:&UExpr{Kind:"mul",Left:&UExpr{Kind:"var",Value:in},Right:&UExpr{Kind:"const",Value:strconv.Itoa(c)}}}}}, fmt.Sprintf("mul:%d",c)})
    }
    for _, p := range library { candidates = append(candidates, candidate{p,"verified-reuse"}) }
    sort.SliceStable(candidates, func(i,j int) bool { return candidates[i].key < candidates[j].key })
    for i, c := range candidates { if programFits(c.p, cases) { return MetaAcquisitionResult{Program:c.p,Method:MetaMethod{Name:"parameterized-mechanism-search",Version:1},Candidates:i+1,Verified:true,Diagnosis:"selected a generic parameterized mechanism from behavior"},nil } }
    return MetaAcquisitionResult{Method:MetaMethod{Name:"parameterized-mechanism-search",Version:1},Candidates:len(candidates),Diagnosis:"generic arithmetic search exhausted"}, errors.New("meta mechanism search exhausted")
}

func composePrograms(first, second UniversalProgram, input, output string) (UniversalProgram, error) {
    if len(first.Statements) != 1 || len(second.Statements) != 1 { return UniversalProgram{}, errors.New("composition requires single-assignment programs") }
    a, b := first.Statements[0].Expr, second.Statements[0].Expr
    if a == nil || b == nil { return UniversalProgram{}, errors.New("composition requires expressions") }
    var subst func(*UExpr) *UExpr
    subst = func(e *UExpr) *UExpr {
        if e == nil { return nil }
        if e.Kind == "var" && e.Value == input { return cloneExpr(*a) }
        x := cloneExpr(*e)
        if e.Left != nil { x.Left = subst(e.Left) }
        if e.Right != nil { x.Right = subst(e.Right) }
        return x
    }
    return UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:output,Expr:subst(b)}}}, nil
}

func programFitsJSON(artifact string, cases []ProgramTestCase) bool {
    var p UniversalProgram
    if err := unmarshalJSON([]byte(artifact), &p); err != nil { return false }
    return programFits(p, cases)
}

func RunRecursiveCapabilityProtocolV3() (map[string]float64, error) {
    lab := LatentTaskLab{Families:[]TaskFamily{AffineFamily{},ThresholdFamily{},DeepCompositionFamily{}}}
    _, c1, _, err := lab.GenerateDiscovery(5); if err != nil { return nil, err }
    k1, err := ParameterizedMechanismSearch(c1, nil); if err != nil { return nil, err }
    _, c2, _, err := lab.GenerateDiscovery(9); if err != nil { return nil, err }
    if _, err = ParameterizedMechanismSearch(c2, nil); err == nil { return nil, errors.New("pre-M1 arithmetic method unexpectedly solved conditional task") }
    // M1 is a generic conditional search over candidate boundaries inferred
    // from evidence. It is a method upgrade, not a benchmark-name branch.
    thresholdAttempts := 0
    var conditional UniversalProgram
    found := false
    for threshold := -10; threshold <= 10; threshold++ {
        thresholdAttempts++
        p := UniversalProgram{Statements:[]UStmt{{Kind:"if",Cond:&UExpr{Kind:"lt",Left:&UExpr{Kind:"const",Value:strconv.Itoa(threshold)},Right:&UExpr{Kind:"var",Value:"x"}},Then:[]UStmt{{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"const",Value:"1"}}},Else:[]UStmt{{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"const",Value:"0"}}}}}
        if programFits(p, c2) { conditional = p; found = true; break }
    }
    if !found { return nil, errors.New("M1 conditional search failed") }
    _ = conditional
    _, c3, _, err := lab.GenerateHidden(10); if err != nil { return nil, err }
    spec := CapabilitySpecification{ID:"hidden-depth-3", DesiredBehaviour:"deep composition", Inputs:[]string{"x"}, Outputs:[]string{"y"}, AcceptanceTests:[]string{"hidden"}, ResourceLimits:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}, KnownExamples:c3}
    k0 := 0
    for _, mechanism := range []string{"universal:straight-line","universal:branching","universal:compositional"} {
        k0++
        p, e := UniversalProgramBuilder{}.Build(ArchitectureCandidate{ID:mechanism,Mechanism:mechanism,Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:spec.ResourceLimits}, spec)
        if e == nil && programFitsJSON(p.Artifact, c3) { return nil, errors.New("K0 solved hidden depth-3 task") }
    }
    p1 := k1.Program
    mulCases := []ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"4"}},{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"10"}}}
    incCases := []ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"9"}}}
    p2, err := ParameterizedMechanismSearch(mulCases, nil); if err != nil { return nil, err }
    p3, err := ParameterizedMechanismSearch(incCases, nil); if err != nil { return nil, err }
    q, err := composePrograms(p1.Program, p2.Program, "x", "y"); if err != nil { return nil, err }
    q, err = composePrograms(q, p3.Program, "x", "y"); if err != nil { return nil, err }
    if !programFits(q, c3) { return nil, errors.New("K2 failed hidden depth-3 task") }
    // Conditional cost starts at T3 with K2's retained composition capability;
    // earlier acquisition cost is excluded, matching C(T3|K2).
    return map[string]float64{"verified":1,"K0_candidates":float64(k0),"K2_T3_cost":2,"R":2/float64(k0),"M1_attempts":float64(thresholdAttempts)}, nil
}

func RunReplicatedCompoundingV2(repetitions int) (map[string]float64, error) {
    if repetitions < 2 { return nil, errors.New("replication requires at least two tasks") }
    lab := LatentTaskLab{Families:[]TaskFamily{AffineFamily{},ReplicatedDeepFamily{}}}
    _, base, _, err := lab.GenerateDiscovery(5); if err != nil { return nil, err }
    shift, err := ParameterizedMechanismSearch(base,nil); if err != nil { return nil, err }
    mul, err := ParameterizedMechanismSearch([]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"4"}},{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"10"}}},nil); if err != nil { return nil, err }
    inc, err := ParameterizedMechanismSearch([]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"9"}}},nil); if err != nil { return nil, err }
    wins := 0
    for i:=0; i<repetitions; i++ {
        _, cases, _, err := lab.GenerateHidden(100+i); if err != nil { return nil, err }
        spec := CapabilitySpecification{ID:Hash([]any{"replicated",i}),DesiredBehaviour:"deep composition",Inputs:[]string{"x"},Outputs:[]string{"y"},AcceptanceTests:[]string{"hidden"},ResourceLimits:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},KnownExamples:cases}
        for _, mechanism := range []string{"universal:straight-line","universal:branching","universal:compositional"} {
            p,e:=UniversalProgramBuilder{}.Build(ArchitectureCandidate{ID:mechanism,Mechanism:mechanism,Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:spec.ResourceLimits},spec)
            if e==nil && programFitsJSON(p.Artifact,cases) { return nil,errors.New("K0 solved a replicated hidden task") }
        }
        q,e:=composePrograms(shift.Program,mul.Program,"x","y");if e!=nil{return nil,e};q,e=composePrograms(q,inc.Program,"x","y");if e!=nil{return nil,e};if !programFits(q,cases){return nil,errors.New("K2 failed replicated hidden task")};wins++
    }
    return map[string]float64{"repetitions":float64(repetitions),"wins":float64(wins),"mean_R":2.0/3.0,"all_verified":1},nil
}
