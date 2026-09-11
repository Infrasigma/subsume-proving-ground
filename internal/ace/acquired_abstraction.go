package ace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

type AbstractionContract struct {
	Inputs         []string
	Outputs        []string
	Preconditions  []string
	Postconditions []string
}

type AbstractionEvidence struct {
	TaskStructure string
	Verified      bool
	HeldOut       bool
	TransferScore float64
	DiscoveryCost ResourceVector
	ObservedGain  float64
}

type AcquiredAbstraction struct {
	ID           string
	Name         string
	Procedure    AcquisitionProcedure
	Contract     AbstractionContract
	Dependencies []string
	Evidence     []AbstractionEvidence
	CostHistory  []ResourceVector
	Verification VerificationResult
	Provenance   Provenance
}

type AbstractionLibrary struct {
	Version      uint64
	Abstractions []AcquiredAbstraction
}

func (l *AbstractionLibrary) Find(id string) (AcquiredAbstraction, bool) {
	for _, a := range l.Abstractions {
		if a.ID == id {
			return a, true
		}
	}
	return AcquiredAbstraction{}, false
}

func (l *AbstractionLibrary) Install(a AcquiredAbstraction) error {
	if a.ID == "" || a.Name == "" {
		return errors.New("incomplete acquired abstraction")
	}
	if len(a.Procedure.Steps) < 2 {
		return errors.New("acquired abstraction must compress a non-trivial composition")
	}
	if _, err := a.Procedure.Marshal(); err != nil {
		return err
	}
	if !a.Verification.Independent || a.Verification.Status != "verified" {
		return errors.New("acquired abstraction lacks independent verification")
	}
	if len(a.Evidence) == 0 {
		return errors.New("acquired abstraction lacks evidence")
	}
	for _, dep := range abstractionDependencies(a.Procedure) {
		if dep == a.ID {
			return errors.New("acquired abstraction cannot depend on itself")
		}
		if _, ok := l.Find(dep); !ok {
			return errors.New("acquired abstraction dependency is not installed")
		}
	}
	if _, ok := l.Find(a.ID); ok {
		return nil
	}
	l.Abstractions = append(l.Abstractions, a)
	l.Version++
	return nil
}

func (l AbstractionLibrary) IDs() []string {
	out := make([]string, 0, len(l.Abstractions))
	for _, a := range l.Abstractions {
		out = append(out, a.ID)
	}
	sort.Strings(out)
	return out
}

type AbstractionObservation struct {
	TaskStructure string
	Procedure     AcquisitionProcedure
	Verified      bool
	HeldOut       bool
	TransferScore float64
	DiscoveryCost ResourceVector
	ObservedGain  float64
}

type AbstractionVerificationCase struct {
	Input    []ArchitectureCandidate
	Expected []string
}

func abstractionDependencies(p AcquisitionProcedure) []string {
	seen := map[string]bool{}
	for _, s := range p.Steps {
		if s.Op == "call" && s.Ref != "" {
			seen[s.Ref] = true
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func DiscoverReusableAbstraction(observations []AbstractionObservation, minDistinctStructures int) (AcquiredAbstraction, error) {
	if minDistinctStructures < 2 {
		minDistinctStructures = 2
	}
	type bucket struct {
		procedure    AcquisitionProcedure
		observations []AbstractionObservation
		structures   map[string]bool
	}
	buckets := map[string]*bucket{}
	for _, o := range observations {
		if !o.Verified || !o.HeldOut || len(o.Procedure.Steps) < 2 {
			continue
		}
		sig := procedureSignature(o.Procedure)
		b := buckets[sig]
		if b == nil {
			b = &bucket{procedure: o.Procedure, structures: map[string]bool{}}
			buckets[sig] = b
		}
		b.observations = append(b.observations, o)
		if o.TaskStructure != "" {
			b.structures[o.TaskStructure] = true
		}
	}
	var best *bucket
	bestScore := -1.0
	for _, b := range buckets {
		if len(b.structures) < minDistinctStructures {
			continue
		}
		score := 0.0
		for _, o := range b.observations {
			score += o.ObservedGain / (1 + o.DiscoveryCost.Compute + o.DiscoveryCost.Memory + o.DiscoveryCost.TimeMS + o.DiscoveryCost.ExperimentBudget)
		}
		if best == nil || score > bestScore || (score == bestScore && procedureSignature(b.procedure) < procedureSignature(best.procedure)) {
			best = b
			bestScore = score
		}
	}
	if best == nil {
		return AcquiredAbstraction{}, errors.New("no reusable abstraction has cross-structure evidence")
	}
	first := best.observations[0]
	for _, o := range best.observations[1:] {
		if o.DiscoveryCost.Compute < first.DiscoveryCost.Compute {
			first = o
		}
	}
	a := AcquiredAbstraction{
		ID:        Hash([]any{"acquired-abstraction", procedureSignature(best.procedure)}),
		Name:      "acquired-abstraction:" + procedureSignature(best.procedure),
		Procedure: best.procedure,
		Contract: AbstractionContract{
			Inputs:         []string{"architecture-candidate-stream"},
			Outputs:        []string{"architecture-candidate-stream"},
			Preconditions:  []string{"bounded candidate stream", "all referenced abstractions installed"},
			Postconditions: []string{"deterministic executable transformation"},
		},
		Dependencies: abstractionDependencies(best.procedure),
		Verification: VerificationResult{Status: "pending", Independent: false},
		Provenance:   Prov("abstraction-acquisition", first.TaskStructure, "cross-structure-composition", best.procedure),
	}
	for _, o := range best.observations {
		a.Evidence = append(a.Evidence, AbstractionEvidence{TaskStructure:o.TaskStructure,Verified:o.Verified,HeldOut:o.HeldOut,TransferScore:o.TransferScore,DiscoveryCost:o.DiscoveryCost,ObservedGain:o.ObservedGain})
		a.CostHistory = append(a.CostHistory, o.DiscoveryCost)
	}
	return a, nil
}

func VerifyAcquiredAbstraction(a AcquiredAbstraction, lib *AbstractionLibrary, cases []AbstractionVerificationCase) (AcquiredAbstraction, error) {
	if lib == nil {
		return a, errors.New("abstraction verification requires a library")
	}
	if len(cases) < 2 {
		return a, errors.New("abstraction verification requires at least two independent cases")
	}
	for _, tc := range cases {
		got, err := ExecuteAcquiredAbstraction(a, tc.Input, lib)
		if err != nil { return a, err }
		want := referenceProcedure(a.Procedure, tc.Input, lib, map[string]bool{})
		if !equalMechanismOrders(got, want) { return a, errors.New("independent abstraction reference disagrees with executable semantics") }
		if len(tc.Expected) > 0 && !equalMechanismOrders(got, namesToCandidates(tc.Expected)) { return a, errors.New("independent abstraction verifier rejected expected held-out behavior") }
	}
	a.Verification = VerificationResult{Status:"verified",Independent:true,Expected:[]string{"held-out procedure behavior when supplied","independent reference agreement"},Observed:[]string{"independent reference interpreter agreement"},Provenance:Prov("independent-abstraction-verifier",a.ID,"differential-reference-execution",cases)}
	return a, nil
}

func referenceProcedure(p AcquisitionProcedure, cs []ArchitectureCandidate, lib *AbstractionLibrary, stack map[string]bool) []ArchitectureCandidate {
	cur := append([]ArchitectureCandidate(nil), cs...)
	for _, s := range p.Steps {
		switch s.Op {
		case "identity":
		case "reverse":
			next := make([]ArchitectureCandidate, len(cur)); for i := range cur { next[len(cur)-1-i] = cur[i] }; cur = next
		case "rotate":
			if len(cur)==0 { continue }; n:=s.Arg%len(cur); if n<0 {n+=len(cur)}; next:=append([]ArchitectureCandidate(nil),cur[n:]...); next=append(next,cur[:n]...); cur=next
		case "dedupe":
			seen:=map[string]bool{}; next:=make([]ArchitectureCandidate,0,len(cur)); for _,c:=range cur {if !seen[c.Mechanism] {seen[c.Mechanism]=true;next=append(next,c)}}; cur=next
		case "sort-cost":
			for i:=1;i<len(cur);i++ {v:=cur[i];j:=i-1;for j>=0&&cur[j].Resources.Compute+cur[j].Resources.ExperimentBudget>v.Resources.Compute+v.Resources.ExperimentBudget {cur[j+1]=cur[j];j--};cur[j+1]=v}
		case "take":
			if s.Arg<1||s.Arg>len(cur) {return nil};cur=append([]ArchitectureCandidate(nil),cur[:s.Arg]...)
		case "call":
			if stack[s.Ref] {return nil};a,ok:=lib.Find(s.Ref);if !ok{return nil};stack[s.Ref]=true;cur=referenceProcedure(a.Procedure,cur,lib,stack);delete(stack,s.Ref)
		}
	}
	return cur
}

func equalMechanismOrders(a,b []ArchitectureCandidate)bool{if len(a)!=len(b){return false};for i:=range a{if a[i].Mechanism!=b[i].Mechanism{return false}};return true}
func namesToCandidates(names []string)[]ArchitectureCandidate{out:=make([]ArchitectureCandidate,len(names));for i,name:=range names{out[i]=ArchitectureCandidate{Mechanism:name}};return out}

func enumerateProcedureAtoms(lib *AbstractionLibrary) []ProcedureStep {atoms:=[]ProcedureStep{{Op:"identity"},{Op:"reverse"},{Op:"dedupe"},{Op:"sort-cost"},{Op:"take",Arg:1},{Op:"rotate",Arg:1}};if lib!=nil{for _,id:=range lib.IDs(){atoms=append(atoms,ProcedureStep{Op:"call",Ref:id})}};return atoms}
func ProcedureLibrarySearchCost(maxSteps int,lib *AbstractionLibrary)int{if maxSteps<1{return 0};n:=len(enumerateProcedureAtoms(lib));total:=0;power:=1;for d:=1;d<=maxSteps;d++{power*=n;total+=power};return total}
func ExecuteAcquiredAbstraction(a AcquiredAbstraction,cs []ArchitectureCandidate,lib *AbstractionLibrary)([]ArchitectureCandidate,error){if lib==nil{return nil,errors.New("abstraction execution requires a library")};return executeSearchProcedureWithLibrary(a.Procedure,cs,lib)}

type persistedAbstractions struct{Version uint64 `json:"version"`;Abstractions []AcquiredAbstraction `json:"abstractions"`}
type PersistentAbstractionLibrary struct{Path string;Data persistedAbstractions}
func NewPersistentAbstractionLibrary(path string)(*PersistentAbstractionLibrary,error){p:=&PersistentAbstractionLibrary{Path:path};b,err:=os.ReadFile(path);if err==nil{if err=json.Unmarshal(b,&p.Data);err!=nil{return nil,err}}else if !errors.Is(err,os.ErrNotExist){return nil,err};return p,nil}
func(p *PersistentAbstractionLibrary)Save(l *AbstractionLibrary)error{if l==nil{return errors.New("nil abstraction library")};p.Data=persistedAbstractions{Version:l.Version,Abstractions:append([]AcquiredAbstraction(nil),l.Abstractions...)};if p.Path==""{return nil};if err:=os.MkdirAll(filepath.Dir(p.Path),0755);err!=nil{return err};b,err:=json.MarshalIndent(p.Data,"","  ");if err!=nil{return err};tmp:=p.Path+".tmp";if err=os.WriteFile(tmp,b,0600);err!=nil{return err};return os.Rename(tmp,p.Path)}
func(p *PersistentAbstractionLibrary)Restore(dst *AbstractionLibrary)error{if dst==nil{return errors.New("nil abstraction library destination")};for _,a:=range p.Data.Abstractions{if err:=dst.Install(a);err!=nil{return err}};return nil}
