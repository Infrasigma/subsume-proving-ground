package ace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
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
	ArtifactHash string `json:"artifact_hash"`
	KMSSignature protocol.KMSSignedArtifact `json:"kms_signature"`
	LedgerAdmissionRef string `json:"ledger_admission_ref"`
	LedgerAdmissionHash string `json:"ledger_admission_hash"`
	LedgerPreviousAdmissionHash string `json:"ledger_previous_admission_hash"`
	LedgerCreatedAtUnix int64 `json:"ledger_created_at_unix"`
}

type AbstractionLibrary struct {
	Version       uint64
	Abstractions  []AcquiredAbstraction
	TrustedSigners map[string]string `json:"trusted_signers,omitempty"`
}

type canonicalAbstractionArtifact struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Procedure AcquisitionProcedure `json:"procedure"`
	Contract AbstractionContract `json:"contract"`
	Dependencies []string `json:"dependencies"`
	Evidence []AbstractionEvidence `json:"evidence"`
	CostHistory []ResourceVector `json:"cost_history"`
	Verification VerificationResult `json:"verification"`
	Provenance Provenance `json:"provenance"`
}

func (a AcquiredAbstraction) canonicalArtifact() (string, []byte, error) {
	v := canonicalAbstractionArtifact{a.ID,a.Name,a.Procedure,a.Contract,append([]string(nil),a.Dependencies...),append([]AbstractionEvidence(nil),a.Evidence...),append([]ResourceVector(nil),a.CostHistory...),a.Verification,a.Provenance}
	encoded, err := json.Marshal(v)
	if err != nil { return "", nil, err }
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil { return "", nil, err }
	canonical, err := c14n.Canonicalize(value)
	if err != nil { return "", nil, err }
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), canonical, nil
}

func (a AcquiredAbstraction) VerifyAdmission(trustedPublicKeyB64 string) error {
	h, _, err := a.canonicalArtifact()
	if err != nil { return err }
	if a.ArtifactHash == "" || h != a.ArtifactHash { return fmt.Errorf("abstraction artifact hash mismatch") }
	r := protocol.AbstractionAdmissionReceipt{KMSSignedArtifact:a.KMSSignature, LedgerAdmissionRef:a.LedgerAdmissionRef, LedgerAdmissionHash:a.LedgerAdmissionHash, PreviousAdmissionHash:a.LedgerPreviousAdmissionHash, CreatedAtUnix:a.LedgerCreatedAtUnix}
	if r.ArtifactHash != a.ArtifactHash { return fmt.Errorf("KMS signature artifact hash mismatch") }
	return protocol.VerifyAbstractionAdmissionReceipt(r, trustedPublicKeyB64)
}

func (l *AbstractionLibrary) ConfigureTrustedSigner(signerID, publicKeyB64 string) {
	if l.TrustedSigners == nil { l.TrustedSigners = map[string]string{} }
	l.TrustedSigners[signerID] = publicKeyB64
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
	if len(a.Evidence) == 0 { return errors.New("acquired abstraction lacks evidence") }
	trusted := ""; if l.TrustedSigners != nil { trusted = l.TrustedSigners[a.KMSSignature.SignerID] }
	if trusted == "" { return errors.New("acquired abstraction has no trusted KMS signer") }
	if err := a.VerifyAdmission(trusted); err != nil { return err }
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
	return discoverReusableAbstraction(observations, minDistinctStructures, nil)
}

func DiscoverReusableAbstractionWithDiagnostics(observations []AbstractionObservation, minDistinctStructures int, log *DiagnosticLog) (AcquiredAbstraction, error) {
	return discoverReusableAbstraction(observations, minDistinctStructures, log)
}

func discoverReusableAbstraction(observations []AbstractionObservation, minDistinctStructures int, log *DiagnosticLog) (AcquiredAbstraction, error) {
	if minDistinctStructures < 2 {
		minDistinctStructures = 2
	}
	type bucket struct {
		procedure    AcquisitionProcedure
		observations []AbstractionObservation
		structures   map[string]bool
	}
	buckets := map[string]*bucket{}
	for i, o := range observations {
		rawID := fmt.Sprintf("observation-%d", i)
		switch {
		case !o.Verified:
			log.Record("C", rawID, "abstraction-observation", o, DiagnosticEvidenceGate, "observation.Verified == true", false, "candidate observation rejected before proposal")
			continue
		case !o.HeldOut:
			log.Record("C", rawID, "abstraction-observation", o, DiagnosticEvidenceGate, "observation.HeldOut == true", false, "candidate observation rejected before proposal")
			continue
		case len(o.Procedure.Steps) < 2:
			log.Record("C", rawID, "abstraction-observation", o, DiagnosticFormalValidity, "len(procedure.Steps) >= 2", false, "procedure is not a non-trivial composition")
			continue
		}
		sig := procedureSignature(o.Procedure)
		log.Record("C", sig, "abstraction-candidate", o.Procedure, DiagnosticProposal, "verified + held-out + non-trivial composition", true, "candidate entered abstraction bucket")
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
	for sig, b := range buckets {
		if len(b.structures) < minDistinctStructures {
			log.Record("C", sig, "abstraction-candidate", b.procedure, DiagnosticEvidenceGate, fmt.Sprintf("distinct_task_structures >= %d", minDistinctStructures), false, fmt.Sprintf("only %d distinct task structures", len(b.structures)))
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
		log.Record("C", "", "abstraction-selection", map[string]any{"candidate_buckets": len(buckets), "min_distinct_structures": minDistinctStructures}, DiagnosticSelection, "exists candidate bucket with sufficient cross-structure evidence", false, "all proposed candidates died at the evidence gate")
		return AcquiredAbstraction{}, errors.New("no reusable abstraction has cross-structure evidence")
	}
	log.Record("C", procedureSignature(best.procedure), "abstraction-candidate", best.procedure, DiagnosticSelection, "score is maximal among candidates passing evidence gate", true, "candidate selected for independent verification")
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
	return verifyAcquiredAbstraction(a, lib, cases, nil)
}

func VerifyAcquiredAbstractionWithDiagnostics(a AcquiredAbstraction, lib *AbstractionLibrary, cases []AbstractionVerificationCase, log *DiagnosticLog) (AcquiredAbstraction, error) {
	return verifyAcquiredAbstraction(a, lib, cases, log)
}

func verifyAcquiredAbstraction(a AcquiredAbstraction, lib *AbstractionLibrary, cases []AbstractionVerificationCase, log *DiagnosticLog) (AcquiredAbstraction, error) {
	if lib == nil {
		log.Record("C", a.ID, "abstraction-candidate", a, DiagnosticFormalValidity, "verification library != nil", false, "independent verification cannot execute")
		return a, errors.New("abstraction verification requires a library")
	}
	if len(cases) < 2 {
		log.Record("C", a.ID, "abstraction-candidate", a, DiagnosticEvidenceGate, "len(verification cases) >= 2", false, "insufficient independent verification cases")
		return a, errors.New("abstraction verification requires at least two independent cases")
	}
	for _, tc := range cases {
		got, err := ExecuteAcquiredAbstraction(a, tc.Input, lib)
		if err != nil {
			log.Record("C", a.ID, "abstraction-candidate", a, DiagnosticFormalValidity, "executable candidate completes without runtime error", false, err.Error())
			return a, err
		}
		want := referenceProcedure(a.Procedure, tc.Input, lib, map[string]bool{})
		if !equalMechanismOrders(got, want) {
			log.Record("C", a.ID, "abstraction-candidate", a, DiagnosticEnvironmental, "executable output == independent reference output", false, "differential execution mismatch")
			return a, errors.New("independent abstraction reference disagrees with executable semantics")
		}
		if len(tc.Expected) > 0 && !equalMechanismOrders(got, namesToCandidates(tc.Expected)) {
			log.Record("C", a.ID, "abstraction-candidate", a, DiagnosticEvidenceGate, "executable output == held-out expected output", false, "held-out behavior mismatch")
			return a, errors.New("independent abstraction verifier rejected expected held-out behavior")
		}
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

type persistedAbstractions struct{Version uint64 `json:"version"`;Abstractions []AcquiredAbstraction `json:"abstractions"`;TrustedSigners map[string]string `json:"trusted_signers,omitempty"`}
type PersistentAbstractionLibrary struct{Path string;Data persistedAbstractions}
func NewPersistentAbstractionLibrary(path string)(*PersistentAbstractionLibrary,error){p:=&PersistentAbstractionLibrary{Path:path};b,err:=os.ReadFile(path);if err==nil{if err=json.Unmarshal(b,&p.Data);err!=nil{return nil,err}}else if !errors.Is(err,os.ErrNotExist){return nil,err};return p,nil}
func(p *PersistentAbstractionLibrary)Save(l *AbstractionLibrary)error{if l==nil{return errors.New("nil abstraction library")};trusted:=map[string]string{};for id,key:=range l.TrustedSigners{trusted[id]=key};p.Data=persistedAbstractions{Version:l.Version,Abstractions:append([]AcquiredAbstraction(nil),l.Abstractions...),TrustedSigners:trusted};if p.Path==""{return nil};if err:=os.MkdirAll(filepath.Dir(p.Path),0755);err!=nil{return err};b,err:=json.MarshalIndent(p.Data,"","  ");if err!=nil{return err};tmp:=p.Path+".tmp";if err=os.WriteFile(tmp,b,0600);err!=nil{return err};return os.Rename(tmp,p.Path)}
func(p *PersistentAbstractionLibrary)Restore(dst *AbstractionLibrary)error{if dst==nil{return errors.New("nil abstraction library destination")};if p.Data.TrustedSigners!=nil{dst.TrustedSigners=map[string]string{};for id,key:=range p.Data.TrustedSigners{dst.TrustedSigners[id]=key}};for _,a:=range p.Data.Abstractions{if err:=dst.Install(a);err!=nil{return err}};return nil}
