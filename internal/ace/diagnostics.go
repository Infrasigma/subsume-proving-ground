package ace

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type DiagnosticStage string

const (
	DiagnosticProposal       DiagnosticStage = "proposal"
	DiagnosticFormalValidity DiagnosticStage = "formal-validity"
	DiagnosticEnvironmental  DiagnosticStage = "environmental-validity"
	DiagnosticEvidenceGate   DiagnosticStage = "evidence-gate"
	DiagnosticVerification   DiagnosticStage = "independent-verification"
	DiagnosticSelection      DiagnosticStage = "selection"
	DiagnosticAccepted       DiagnosticStage = "accepted"
)

type CandidateDiagnosticEvent struct {
	Sequence      int64          `json:"sequence"`
	Layer         string         `json:"layer"`
	CandidateID   string         `json:"candidate_id,omitempty"`
	CandidateKind string         `json:"candidate_kind,omitempty"`
	CandidateJSON string         `json:"candidate_json,omitempty"`
	Stage         DiagnosticStage `json:"stage"`
	Predicate     string         `json:"predicate"`
	Accepted      bool           `json:"accepted"`
	Detail        string         `json:"detail,omitempty"`
}

type DiagnosticLog struct {
	mu     sync.Mutex
	events []CandidateDiagnosticEvent
}

func (l *DiagnosticLog) Record(layer, candidateID, candidateKind string, candidate any, stage DiagnosticStage, predicate string, accepted bool, detail string) {
	if l == nil { return }
	b, err := json.Marshal(candidate)
	if err != nil { b = []byte(fmt.Sprintf("<unserializable:%v>", err)) }
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, CandidateDiagnosticEvent{
		Sequence: int64(len(l.events)+1), Layer: layer, CandidateID: candidateID,
		CandidateKind: candidateKind, CandidateJSON: string(b), Stage: stage,
		Predicate: predicate, Accepted: accepted, Detail: detail,
	})
}

func (l *DiagnosticLog) Events() []CandidateDiagnosticEvent {
	if l == nil { return nil }
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]CandidateDiagnosticEvent(nil), l.events...)
}


func (l *DiagnosticLog) WriteJSONL(w io.Writer) error {
	for _, e := range l.Events() {
		b, err := json.Marshal(e)
		if err != nil { return err }
		if _, err := w.Write(append(b, '\n')); err != nil { return err }
	}
	return nil
}
