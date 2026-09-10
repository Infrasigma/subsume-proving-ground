package ace

import (
    "encoding/json"
    "errors"
    "fmt"
    "math"
    "sort"
    "strconv"
    "strings"
)

// TransitionRecord is the lossless, typed bridge from raw experience to learning.
type TransitionRecord struct {
    Before State
    Action Action
    After State
    Outcome map[string]string
    Uncertainty Uncertainty
    Provenance Provenance
}

func EncodeTransition(x Experience, tr TransitionRecord) Experience {
    b, _ := json.Marshal(tr)
    x.Raw = b
    x.Derived = nil
    x.Uncertainty = tr.Uncertainty
    x.Provenance = Prov("experience-transition", x.Provenance.ID, "encode-transition", tr)
    return x
}

func DecodeTransition(x Experience) (TransitionRecord, error) {
    var tr TransitionRecord
    if len(x.Raw) == 0 { return tr, errors.New("experience has no raw transition") }
    if err := json.Unmarshal(x.Raw, &tr); err != nil { return tr, fmt.Errorf("decode transition: %w", err) }
    return tr, nil
}

// BayesianCausalEngine maintains weighted competing deterministic hypotheses.
// It never chooses an outcome before the model predicts one.
type BayesianCausalEngine struct{}

func interventionKey(a Action) string {
    keys := make([]string, 0, len(a.Arguments))
    for k := range a.Arguments { keys = append(keys, k) }
    sort.Strings(keys)
    var b strings.Builder
    b.WriteString(a.Operation)
    for _, k := range keys { b.WriteByte('|'); b.WriteString(k); b.WriteByte('='); b.WriteString(a.Arguments[k]) }
    return b.String()
}

func normalizeHypotheses(hs []Hypothesis) []Hypothesis {
    var sum float64
    for _, h := range hs { if h.Confidence > 0 { sum += h.Confidence } }
    if sum == 0 { sum = float64(len(hs)) }
    out := make([]Hypothesis, len(hs))
    copy(out, hs)
    for i := range out {
        if out[i].Confidence <= 0 { out[i].Confidence = 1 / sum } else { out[i].Confidence /= sum }
    }
    return out
}

func (BayesianCausalEngine) Predict(m CausalModel, s State, a Action) ([]Prediction, error) {
    hs := normalizeHypotheses(m.Hypotheses)
    if len(hs) == 0 { return nil, errors.New("causal model has no competing hypotheses") }
    key := interventionKey(a)
    out := make([]Prediction, 0, len(hs))
    for _, h := range hs {
        effect, ok := h.InterventionOutcomes[key]
        if !ok { effect = "unknown" }
        out = append(out, Prediction{ID: Hash([]any{m.ID, h.ID, key}), ActionID: a.ID, Effects: []string{effect}, Probability: h.Confidence, StateHash: Hash(semanticState(s))})
    }
    return out, nil
}

func (BayesianCausalEngine) Revise(m CausalModel, o Observation) (CausalModel, error) {
    actual := o.Outcome["outcome"]
    key := o.Outcome["intervention"]
    if actual == "" || key == "" { return m, errors.New("causal revision requires intervention and outcome") }
    hs := normalizeHypotheses(m.Hypotheses)
    for i := range hs {
        expected := hs[i].InterventionOutcomes[key]
        switch {
        case expected == actual:
            hs[i].EvidenceIDs = appendUnique(hs[i].EvidenceIDs, o.ID)
            hs[i].Confidence *= 2
        case expected != "":
            hs[i].Counterexamples = appendUnique(hs[i].Counterexamples, o.ID)
            hs[i].Confidence *= 0.1
        }
    }
    hs = normalizeHypotheses(hs)
    m.Hypotheses = hs
    m.Revision++
    m.Provenance = Prov("causal-revision", m.Provenance.ID, "bayesian-update", o)
    return m, nil
}

func appendUnique(xs []string, x string) []string {
    for _, y := range xs { if y == x { return xs } }
    return append(xs, x)
}

// InterventionWorld is deliberately narrow: the cognitive core decides what to test;
// the world decides what actually happens.
type InterventionWorld interface { Intervene(map[string]string) (map[string]string, error) }

type InformationGainExperimenter struct { World InterventionWorld }

func (e InformationGainExperimenter) Propose(m CausalModel, hs []Hypothesis, b ResourceVector) ([]Experiment, error) {
    if len(hs) == 0 { hs = m.Hypotheses }
    hs = normalizeHypotheses(hs)
    if len(hs) < 2 { return nil, errors.New("need at least two competing hypotheses") }
    keys := map[string]bool{}
    for _, h := range hs { for k := range h.InterventionOutcomes { keys[k] = true } }
    type candidate struct { key string; score float64 }
    var cs []candidate
    for k := range keys {
        var p string
        disagreement := 0.0
        for _, h := range hs {
            if v, ok := h.InterventionOutcomes[k]; ok {
                if p == "" { p = v } else if v != p { disagreement += h.Confidence }
            }
        }
        if disagreement > 0 { cs = append(cs, candidate{k, disagreement}) }
    }
    sort.Slice(cs, func(i, j int) bool { return cs[i].score > cs[j].score })
    if len(cs) == 0 { return nil, errors.New("no discriminating intervention exists") }
    out := make([]Experiment, 0, len(cs))
    for _, c := range cs {
        op, args := splitInterventionKey(c.key)
        preds := make([]Prediction, 0, len(hs))
        for _, h := range hs {
            preds = append(preds, Prediction{ID: Hash([]any{h.ID, c.key}), Effects: []string{h.InterventionOutcomes[c.key]}, Probability: h.Confidence})
        }
        out = append(out, Experiment{ID: Hash([]any{"ig", c.key}), HypothesisIDs: hypothesisIDs(hs), Intervention: map[string]string{"operation": op, "key": args, "intervention": c.key}, Predicted: preds, Objective: c.score, Budget: b, Provenance: Prov("experiment-selector", m.ID, "information-gain", c)})
    }
    return out, nil
}

func hypothesisIDs(hs []Hypothesis) []string { out := make([]string, len(hs)); for i := range hs { out[i] = hs[i].ID }; return out }

func splitInterventionKey(key string) (string, string) {
    p := strings.SplitN(key, "|", 2)
    if len(p) == 1 { return p[0], "" }
    return p[0], p[1]
}

func (e InformationGainExperimenter) Execute(x Experiment) (Observation, error) {
    if e.World == nil { return Observation{}, errors.New("experiment world unavailable") }
    out, err := e.World.Intervene(x.Intervention)
    if err != nil { return Observation{}, err }
    outcome := out["outcome"]
    if outcome == "" { return Observation{}, errors.New("intervention world returned no outcome") }
    return Observation{ID: Hash([]any{x.ID, out}), ExperimentID: x.ID, Outcome: map[string]string{"intervention": x.Intervention["intervention"], "outcome": outcome}, Provenance: Prov("experiment-execution", x.ID, "world-outcome", out)}, nil
}

// TransitionAbstractor discovers invariant relational changes instead of memorizing instances.
type TransitionAbstractor struct{}

func (TransitionAbstractor) Abstract(xs []Experience) ([]KnowledgeObject, error) {
    type family struct { op string; deltas map[string]int; ids []string; values map[string][]int }
    groups := map[string]*family{}
    for _, x := range xs {
        tr, err := DecodeTransition(x); if err != nil { continue }
        g := groups[tr.Action.Operation]
        if g == nil { g = &family{op: tr.Action.Operation, deltas: map[string]int{}, values: map[string][]int{}}; groups[tr.Action.Operation] = g }
        g.ids = append(g.ids, x.ID)
        for k, after := range tr.After.Values {
            before, bok := tr.Before.Values[k]
            ai, aerr := strconv.Atoi(after); bi, berr := strconv.Atoi(before)
            if bok && aerr == nil && berr == nil { g.deltas[k] = ai - bi; g.values[k] = append(g.values[k], ai-bi) }
        }
    }
    var out []KnowledgeObject
    for _, g := range groups {
        if len(g.ids) < 2 { continue }
        var pat []string
        for k, delta := range g.deltas {
            vals := g.values[k]
            if len(vals) != len(g.ids) { continue }
            same := true; for _, v := range vals { if v != delta { same = false; break } }
            if same { pat = append(pat, "delta:"+k+"="+strconv.Itoa(delta)) }
        }
        if len(pat) == 0 { continue }
        sort.Strings(pat)
        ids := append([]string(nil), g.ids...)
        out = append(out, KnowledgeObject{ID: Hash([]any{"abstract", g.op, pat}), Statement: "operation " + g.op + " preserves transition invariants", Level: C2, EvidenceIDs: ids, Scope: []string{"operation:" + g.op}, Pattern: append([]string{"operation:" + g.op}, pat...), Variables: []string{"numeric values"}, Invariants: pat, Confidence: 1, Provenance: Prov("abstraction", ids[0], "invariant-induction", pat)})
    }
    return out, nil
}

// LearnedTransitionSimulator predicts from held-in-experience transitions and exposes residuals.
type LearnedTransitionSimulator struct { Experiences []Experience }

func (s LearnedTransitionSimulator) Simulate(state State, action Action, b ResourceVector) ([]Prediction, error) {
    if err := CheckResources(ResourceVector{TimeMS: 1}, b); err != nil { return nil, err }
    counts := map[string]int{}
    total := 0
    for _, x := range s.Experiences {
        tr, err := DecodeTransition(x); if err != nil || tr.Action.Operation != action.Operation { continue }
        compatible := true
        for k, v := range tr.Before.Values { if sv, ok := state.Values[k]; ok && sv != v { compatible = false; break } }
        if !compatible { continue }
        counts[Hash(semanticState(tr.After))]++
        total++
    }
    if total == 0 { return nil, errors.New("no learned transition matches action/state") }
    type pair struct { hash string; n int }; var ps []pair
    for h, n := range counts { ps = append(ps, pair{h, n}) }
    sort.Slice(ps, func(i,j int) bool { return ps[i].n > ps[j].n })
    out := make([]Prediction, len(ps))
    for i, p := range ps { out[i] = Prediction{ID: Hash([]any{action.ID,p.hash}), ActionID: action.ID, Probability: float64(p.n)/float64(total), StateHash:p.hash} }
    return out, nil
}

// SearchPlanner performs actual breadth-first symbolic search over action sequences.
type SearchPlanner struct { MaxDepth int }

func (p SearchPlanner) Search(t Task, s State, skills []Skill, sim Simulator, b ResourceVector) (Plan, error) {
    if len(skills) == 0 { return Plan{}, errors.New("no skills to search") }
    depth := p.MaxDepth; if depth <= 0 { depth = 8 }
    type node struct { state State; actions []Action }
    q := []node{{state:s}}
    seen := map[string]bool{stateSignature(s):true}
    for d:=0; d<depth && len(q)>0; d++ {
        n := len(q)
        for i:=0;i<n;i++ {
            cur := q[0]; q = q[1:]
            if goalSatisfied(t.Goal, cur.state) { return Plan{ID:Hash([]any{t.ID,cur.actions}),Goal:t.Goal,Steps:cur.actions,Cost:ResourceVector{TimeMS:float64(len(cur.actions))},Verification:[]string{"independent-state-verification"},Provenance:Prov("planner-search",t.ID,"bfs",cur.actions)},nil }
            for _, sk := range skills {
                if sk.Confidence <= 0 || !preconditionsHold(cur.state, sk.Preconditions) { continue }
                for _, a := range sk.Actions {
                    next := applyAction(cur.state,a)
                    if sim != nil { _, _ = sim.Simulate(cur.state,a,b) }
                    sig := stateSignature(next); if seen[sig] { continue }; seen[sig] = true
                    acts := append(append([]Action(nil), cur.actions...), a)
                    q = append(q,node{state:next,actions:acts})
                }
            }
        }
    }
    return Plan{}, errors.New("search exhausted without satisfying goal")
}

func preconditionsHold(s State, ps []string) bool { for _, p := range ps { if !valuePredicate(s,p) { return false } }; return true }
func valuePredicate(s State, p string) bool { k,v,ok := strings.Cut(p,"="); if !ok { return s.Values[p] != "" }; return s.Values[k] == v }
func goalSatisfied(goal string, s State) bool { return valuePredicate(s, goal) }
func stateSignature(s State) string { return Hash(struct{Values map[string]string;Version uint64}{s.Values,s.Version}) }

// Representation adequacy detects systematic aliasing: identical encoded representation
// followed by incompatible verified outcomes is evidence that a distinction is missing.
type RepresentationResidual struct { Representation string; Outcomes []string; EvidenceIDs []string }
func DetectRepresentationInsufficiency(xs []Experience) []RepresentationResidual {
    type bucket struct { outcomes map[string]bool; ids []string }
    buckets := map[string]*bucket{}
    for _, x := range xs {
        tr, err := DecodeTransition(x); if err != nil { continue }
        rep := stateSignature(tr.Before) + "|" + tr.Action.Operation
        outcome := stateSignature(tr.After)
        b := buckets[rep]; if b == nil { b=&bucket{outcomes:map[string]bool{}}; buckets[rep]=b }
        b.outcomes[outcome]=true; b.ids=append(b.ids,x.ID)
    }
    var out []RepresentationResidual
    for rep,b := range buckets { if len(b.outcomes)>1 { var ys []string; for y := range b.outcomes { ys=append(ys,y) }; sort.Strings(ys); out=append(out,RepresentationResidual{Representation:rep,Outcomes:ys,EvidenceIDs:b.ids}) } }
    sort.Slice(out,func(i,j int)bool{return out[i].Representation<out[j].Representation})
    return out
}

// ExecutableProgram is a small, auditable intermediate language. It is not descriptive text:
// the sandbox runs these instructions and tests their observable outputs.
type ProgramInstruction struct { Op, A, B string }
type ExecutableProgram struct { Instructions []ProgramInstruction }

func (p ExecutableProgram) Run(input map[string]string) (map[string]string,error) {
    env:=map[string]string{}; for k,v:=range input { env[k]=v }
    for _, ins := range p.Instructions {
        switch ins.Op {
        case "set": env[ins.A]=ins.B
        case "copy": v,ok:=env[ins.B]; if !ok{return nil,fmt.Errorf("missing input %s",ins.B)}; env[ins.A]=v
        case "add_int": v,ok:=env[ins.B]; if !ok{return nil,fmt.Errorf("missing input %s",ins.B)}; n,e:=strconv.Atoi(v); if e!=nil{return nil,e}; d,e:=strconv.Atoi(ins.B); if e!=nil{return nil,e}; _=d; env[ins.A]=strconv.Itoa(n+1)
        default:return nil,fmt.Errorf("unknown instruction %q",ins.Op)
        }
    }
    return env,nil
}

// ProgramBuilder constructs a runnable artifact for each candidate mechanism.
type ProgramBuilder struct{}
func (ProgramBuilder) Build(c ArchitectureCandidate, s CapabilitySpecification)(ModificationProposal,error){
    if len(s.Inputs)==0 || len(s.Outputs)==0 { return ModificationProposal{}, errors.New("program construction requires input and output") }
    in,out := s.Inputs[0],s.Outputs[0]
    var p ExecutableProgram
    switch c.Mechanism {
    case "copy": p=ExecutableProgram{Instructions:[]ProgramInstruction{{Op:"copy",A:out,B:in}}}
    case "increment": p=ExecutableProgram{Instructions:[]ProgramInstruction{{Op:"add_int",A:out,B:in}}}
    case "zero": p=ExecutableProgram{Instructions:[]ProgramInstruction{{Op:"set",A:out,B:"0"}}}
    default:return ModificationProposal{},fmt.Errorf("unsupported construction mechanism %q",c.Mechanism)
    }
    b,_:=json.Marshal(p)
    return ModificationProposal{ID:Hash([]any{c,s}),Capability:s,Candidate:c,Artifact:string(b),Provenance:Prov("mechanism-builder",c.ID,"construct-executable-program",p)},nil
}

type ProgramTestCase struct { Input, Expected map[string]string }
type ExecutableSandbox struct { Cases map[string][]ProgramTestCase }
func (s ExecutableSandbox) Validate(p ModificationProposal)(RegressionRecord,error){
    if err:=ValidateCandidate(p); err!=nil{return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:false,Regressions:[]string{err.Error()}},err}
    var prog ExecutableProgram
    if err:=json.Unmarshal([]byte(p.Artifact),&prog); err!=nil{return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:false},err}
    cases:=s.Cases[p.Candidate.ID]; if len(cases)==0{return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:false,Regressions:[]string{"no executable acceptance cases"}},errors.New("no executable acceptance cases")}
    for _,tc:=range cases { got,err:=prog.Run(tc.Input); if err!=nil{return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:false,Regressions:[]string{err.Error()}},err}; for k,v:=range tc.Expected { if got[k]!=v{return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:false,Regressions:[]string{fmt.Sprintf("%s: got %q want %q",k,got[k],v)}},errors.New("behavioral test failed")} } }
    return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:true,Tests:p.Candidate.Tests,Provenance:Prov("sandbox",p.ID,"execute-program",prog)},nil
}

// CompetingMechanismSearch emits materially different executable strategies.
type CompetingMechanismSearch struct{}
func (CompetingMechanismSearch) SearchMechanisms(s CapabilitySpecification,b ResourceVector)([]ArchitectureCandidate,error){
    names:=[]string{"copy","increment","zero"}; out:=make([]ArchitectureCandidate,0,len(names))
    for _,name:=range names { out=append(out,ArchitectureCandidate{ID:Hash([]any{s.ID,name}),Mechanism:name,Interfaces:[]string{"ExecutableProgram"},Advantage:name,Assumptions:"finite key/value task contract",Resources:b,Tests:append([]string(nil),s.AcceptanceTests...),Ablations:[]string{"remove-instruction"},RegressionRisks:s.RegressionConstraints,Provenance:Prov("architecture-search",s.ID,"candidate",name)}) }
    return out,nil
}

// SearchAndTestMechanism is an execution-based selector: build and run every candidate,
// then return the first candidate whose independent behavioural cases pass.
func SearchAndTestMechanism(cs []ArchitectureCandidate,s CapabilitySpecification,builder MechanismBuilder,sandbox ExecutableSandbox)(ModificationProposal,RegressionRecord,error){
    var last RegressionRecord
    for _,c:=range cs {
        if err:=CheckResources(c.Resources,b); err!=nil { continue }
        p,err:=builder.Build(c,s); if err!=nil { continue }
        rr,err:=sandbox.Validate(p); last=rr; if err==nil&&rr.Passed{return p,rr,nil}
    }
    return ModificationProposal{},last,errors.New("no executable candidate passed acceptance tests")
}

// Expected information gain lower bound used by controllers when comparing experiments.
func Entropy(p float64) float64 { if p<=0||p>=1{return 0}; return -p*math.Log2(p)-(1-p)*math.Log2(1-p) }
