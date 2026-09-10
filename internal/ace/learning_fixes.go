package ace

import (
    "errors"
    "fmt"
    "sort"
)

// StableStateHash is the semantic state identity used by learned prediction.
// Instance IDs, provenance, and creation metadata are evidence metadata, not state semantics.
func StableStateHash(s State) string {
    return Hash(struct {
        Entities []Entity
        Relations []Relation
        Events []Event
        Values map[string]string
    }{s.Entities, s.Relations, s.Events, s.Values})
}

// EmpiricalSimulator is a corrected learned simulator that groups transitions by semantic successor.
type EmpiricalSimulator struct{ Experiences []Experience }
func (s EmpiricalSimulator) Simulate(state State, action Action, b ResourceVector) ([]Prediction, error) {
    if err := CheckResources(ResourceVector{TimeMS:1}, b); err != nil { return nil, err }
    counts := map[string]int{}
    total := 0
    for _, x := range s.Experiences {
        tr, err := DecodeTransition(x)
        if err != nil || tr.Action.Operation != action.Operation { continue }
        compatible := true
        for k, v := range tr.Before.Values {
            if got, ok := state.Values[k]; ok && got != v { compatible = false; break }
        }
        if !compatible { continue }
        counts[StableStateHash(tr.After)]++
        total++
    }
    if total == 0 { return nil, errors.New("no learned transition matches action/state") }
    type pair struct{ hash string; n int }
    ps := make([]pair, 0, len(counts))
    for h, n := range counts { ps = append(ps, pair{h,n}) }
    sort.Slice(ps, func(i,j int) bool { return ps[i].n > ps[j].n || (ps[i].n == ps[j].n && ps[i].hash < ps[j].hash) })
    out := make([]Prediction, len(ps))
    for i, p := range ps { out[i] = Prediction{ID:Hash([]any{action.ID,p.hash}),ActionID:action.ID,Probability:float64(p.n)/float64(total),StateHash:p.hash} }
    return out, nil
}

// SafeSearchPlanner performs the same BFS objective without aliasing states through shared maps.
type SafeSearchPlanner struct{ MaxDepth int }
func (p SafeSearchPlanner) Search(t Task, s State, skills []Skill, sim Simulator, b ResourceVector) (Plan,error) {
    if len(skills)==0 { return Plan{}, errors.New("no skills to search") }
    depth:=p.MaxDepth; if depth<=0 { depth=8 }
    type node struct{ state State; actions []Action }
    q:=[]node{{state:cloneState(s)}}
    seen:=map[string]bool{StableStateHash(s):true}
    for d:=0; d<depth && len(q)>0; d++ {
        n:=len(q)
        for i:=0;i<n;i++ {
            cur:=q[0]; q=q[1:]
            if goalSatisfied(t.Goal,cur.state) { return Plan{ID:Hash([]any{t.ID,cur.actions}),Goal:t.Goal,Steps:cur.actions,Cost:ResourceVector{TimeMS:float64(len(cur.actions))},Verification:[]string{"independent-state-verification"},Provenance:Prov("safe-planner",t.ID,"bfs",cur.actions)},nil }
            for _,sk:=range skills {
                if sk.Confidence<=0 || !preconditionsHold(cur.state,sk.Preconditions) { continue }
                for _,a:=range sk.Actions {
                    next:=safeApplyAction(cur.state,a)
                    if sim!=nil { if _,err:=sim.Simulate(cur.state,a,b); err!=nil && len(next.Values)==0 { _=err } }
                    sig:=StableStateHash(next); if seen[sig] { continue }; seen[sig]=true
                    acts:=append(append([]Action(nil),cur.actions...),a)
                    q=append(q,node{state:next,actions:acts})
                }
            }
        }
    }
    return Plan{},errors.New("search exhausted without satisfying goal")
}

func cloneState(s State) State { out:=s; out.Values=map[string]string{}; for k,v:=range s.Values { out.Values[k]=v }; return out }
func safeApplyAction(s State,a Action) State { out:=cloneState(s); out.Version++; for k,v:=range a.Arguments { out.Values[k]=v }; return out }

// InstanceIndependentVerifier compares semantic state, not evidence identity.
type InstanceIndependentVerifier struct{}
func (InstanceIndependentVerifier) Verify(a Action, p Prediction, before, after State) (VerificationResult,error) {
    expected:=safeApplyAction(before,a)
    status:="failed"; if StableStateHash(expected)==StableStateHash(after) { status="verified" }
    observed:=[]string{}
    for k,v:=range after.Values { observed=append(observed,fmt.Sprintf("%s=%s",k,v)) }; sort.Strings(observed)
    return VerificationResult{Status:status,Expected:a.ExpectedEffects,Observed:observed,Independent:true,Provenance:Prov("instance-independent-verifier",a.ID,"semantic-state-compare",after)},nil
}
