package ace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

type ReactorExample struct {
	Input    []string `json:"input"`
	Expected []string `json:"expected"`
}

type ReactorTask struct {
	ID                     string         `json:"id"`
	Family                 string         `json:"family"`
	InputKind              string         `json:"input_kind,omitempty"`
	Description            string         `json:"description"`
	Examples               []ReactorExample `json:"examples"`
	MaxSearchDepth         int            `json:"max_search_depth"`
	MinProcedureSteps      int            `json:"min_procedure_steps"`
	AdmitAsAbstraction     bool           `json:"admit_as_abstraction"`
	RequireLatestAdmission bool           `json:"require_latest_admission"`
	RequireAbstractionID   string         `json:"require_abstraction_id,omitempty"`
	MetaKind               string         `json:"meta_kind,omitempty"`
	Autotelic              bool           `json:"autotelic,omitempty"`
	ComplexityScore        int            `json:"complexity_score,omitempty"`
	GapClass               string         `json:"gap_class,omitempty"`
	Budget                 ResourceVector `json:"budget"`
}

func (t ReactorTask) Validate() error {
	if t.ID == "" || t.Family == "" || t.Description == "" {
		return errors.New("reactor task requires id, family, and description")
	}
	if len(t.Examples) < 2 {
		return errors.New("reactor task requires at least two training examples")
	}
	for _, group := range [][]ReactorExample{t.Examples} {
		for _, example := range group {
			if len(example.Input) == 0 || len(example.Input) != len(example.Expected) {
				return errors.New("reactor example requires equal non-empty input and expected streams")
			}
			if t.InputKind == "string" && (len(example.Input) != 1 || len(example.Expected) != 1) {
				return errors.New("string reactor tasks require exactly one input and one expected string")
			}
		}
	}
	if t.InputKind != "" && t.InputKind != "string" && t.InputKind != "architecture-candidate-stream" {
		return fmt.Errorf("unsupported reactor input kind %q", t.InputKind)
	}
	if t.MaxSearchDepth <= 0 {
		return errors.New("reactor task max_search_depth must be positive")
	}
	if t.MinProcedureSteps <= 0 || t.MinProcedureSteps > t.MaxSearchDepth {
		return errors.New("reactor task has invalid procedure depth bounds")
	}
	if t.Autotelic && (t.ComplexityScore < 2 || t.GapClass == "") {
		return errors.New("autotelic task requires non-trivial complexity and a gap class")
	}
	return nil
}

type ReactorTaskLease interface {
	Task() ReactorTask
	Ack() error
	Fail(error) error
}

type ReactorTaskQueue interface {
	Next(context.Context) (ReactorTaskLease, error)
}

type inMemoryReactorLease struct {
	task  ReactorTask
	queue *InMemoryReactorTaskQueue
	once  sync.Once
	err   error
}

func (l *inMemoryReactorLease) Task() ReactorTask { return l.task }

func (l *inMemoryReactorLease) Ack() error {
	l.once.Do(func() {
		l.queue.mu.Lock()
		defer l.queue.mu.Unlock()
		l.queue.acked = append(l.queue.acked, l.task.ID)
	})
	return l.err
}

func (l *inMemoryReactorLease) Fail(err error) error {
	l.once.Do(func() {
		l.queue.mu.Lock()
		defer l.queue.mu.Unlock()
		message := "task failed"
		if err != nil {
			message = err.Error()
		}
		l.queue.failed[l.task.ID] = message
	})
	return l.err
}

type InMemoryReactorTaskQueue struct {
	mu     sync.Mutex
	tasks  []ReactorTask
	next   int
	acked  []string
	failed map[string]string
}

func NewInMemoryReactorTaskQueue(tasks ...ReactorTask) *InMemoryReactorTaskQueue {
	return &InMemoryReactorTaskQueue{
		tasks:  append([]ReactorTask(nil), tasks...),
		failed: map[string]string{},
	}
}

func (q *InMemoryReactorTaskQueue) Next(ctx context.Context) (ReactorTaskLease, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if q.next >= len(q.tasks) {
		return nil, io.EOF
	}
	task := q.tasks[q.next]
	q.next++
	return &inMemoryReactorLease{task: task, queue: q}, nil
}

func (q *InMemoryReactorTaskQueue) Empty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.next >= len(q.tasks)
}

func (q *InMemoryReactorTaskQueue) AcknowledgedIDs() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.acked...)
}

func (q *InMemoryReactorTaskQueue) Failed() map[string]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make(map[string]string, len(q.failed))
	for k, v := range q.failed {
		out[k] = v
	}
	return out
}

type fileReactorLease struct {
	task       ReactorTask
	processing string
	doneDir    string
	failedDir  string
	once       sync.Once
	err        error
}

func (l *fileReactorLease) Task() ReactorTask { return l.task }

func (l *fileReactorLease) Ack() error {
	l.once.Do(func() {
		dst := filepath.Join(l.doneDir, filepath.Base(l.processing))
		l.err = os.Rename(l.processing, dst)
	})
	return l.err
}

func (l *fileReactorLease) Fail(taskErr error) error {
	l.once.Do(func() {
		dst := filepath.Join(l.failedDir, filepath.Base(l.processing))
		if err := os.Rename(l.processing, dst); err != nil {
			l.err = err
			return
		}
		message := ""
		if taskErr != nil {
			message = taskErr.Error()
		}
		l.err = os.WriteFile(dst+".error.txt", []byte(message+"\\n"), 0600)
	})
	return l.err
}

type FileReactorTaskQueue struct {
	Root          string
	DoneDir       string
	FailedDir     string
	ProcessingDir string
	PollInterval  time.Duration
}

func NewFileReactorTaskQueue(root string, pollInterval time.Duration) (*FileReactorTaskQueue, error) {
	if root == "" {
		return nil, errors.New("task queue root is required")
	}
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	q := &FileReactorTaskQueue{
		Root:          root,
		DoneDir:       filepath.Join(root, "done"),
		FailedDir:     filepath.Join(root, "failed"),
		ProcessingDir: filepath.Join(root, "processing"),
		PollInterval:  pollInterval,
	}
	for _, dir := range []string{q.Root, q.DoneDir, q.FailedDir, q.ProcessingDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (q *FileReactorTaskQueue) Empty() bool {
	entries, err := os.ReadDir(q.Root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			return false
		}
	}
	return true
}

func (q *FileReactorTaskQueue) Next(ctx context.Context) (ReactorTaskLease, error) {
	for {
		entries, err := os.ReadDir(q.Root)
		if err != nil {
			return nil, err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			src := filepath.Join(q.Root, entry.Name())
			processing := filepath.Join(q.ProcessingDir, entry.Name())
			if err := os.Rename(src, processing); err != nil {
				continue
			}
			raw, err := os.ReadFile(processing)
			if err != nil {
				_ = (&fileReactorLease{processing: processing, failedDir: q.FailedDir}).Fail(err)
				continue
			}
			var task ReactorTask
			if err := json.Unmarshal(raw, &task); err != nil {
				_ = (&fileReactorLease{processing: processing, failedDir: q.FailedDir}).Fail(err)
				continue
			}
			if err := task.Validate(); err != nil {
				_ = (&fileReactorLease{processing: processing, failedDir: q.FailedDir}).Fail(err)
				continue
			}
			return &fileReactorLease{
				task:       task,
				processing: processing,
				doneDir:    q.DoneDir,
				failedDir:  q.FailedDir,
			}, nil
		}
		timer := time.NewTimer(q.PollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

type ReactorVerifier interface {
	Verify(context.Context, ReactorTask, AcquisitionProcedure, *AbstractionLibrary) error
}

type SynthesizedProgramReactorVerifier interface {
	VerifySynthesizedProgram(context.Context, ReactorTask, SynthesizedProgram) error
}

type StaticReactorVerifier struct {
	HiddenByTask map[string][]ReactorExample
}

func (v StaticReactorVerifier) Verify(ctx context.Context, task ReactorTask, procedure AcquisitionProcedure, lib *AbstractionLibrary) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	examples, ok := v.HiddenByTask[task.ID]
	if !ok || len(examples) < 2 {
		return fmt.Errorf("no evaluator-owned hidden fixture for task %q", task.ID)
	}
	if procedureFitsReactorExamples(procedure, examples, lib) {
		return nil
	}
	return errors.New("independent hidden verification rejected candidate")
}

func (v StaticReactorVerifier) VerifySynthesizedProgram(ctx context.Context, task ReactorTask, program SynthesizedProgram) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	examples, ok := v.HiddenByTask[task.ID]
	if !ok || len(examples) < 2 {
		return fmt.Errorf("no evaluator-owned hidden fixture for task %q", task.ID)
	}
	if err := program.Validate(); err != nil {
		return fmt.Errorf("synthesized program rejected before hidden verification: %w", err)
	}
	for _, example := range examples {
		if len(example.Input) != 1 || len(example.Expected) != 1 {
			return errors.New("synthesized hidden fixture must contain one input and one expected output")
		}
		got, err := program.Execute(ctx, example.Input[0])
		if err != nil {
			return fmt.Errorf("synthesized hidden execution failed: %w", err)
		}
		if got != example.Expected[0] {
			return fmt.Errorf("synthesized hidden verification mismatch: got %q want %q", got, example.Expected[0])
		}
	}
	return nil
}

type AbstractionAdmissionSource interface {
	GetAbstractionAdmission(context.Context, string) (protocol.AbstractionAdmissionReceipt, error)
}

type ParameterizedReactorSearchResult struct {
	Procedure           AcquisitionProcedure
	SynthesizedProgram  *SynthesizedProgram
	EvaluatedCandidates int
	Depth               int
	UsedAbstractionID   string
}

func ParameterizedMechanismSearchWithLibrary(ctx context.Context, task ReactorTask, lib *AbstractionLibrary, verifier ReactorVerifier) (ParameterizedReactorSearchResult, error) {
	return ParameterizedMechanismSearchWithHeuristic(ctx, task, lib, verifier, nil)
}

func ParameterizedMechanismSearchWithHeuristic(ctx context.Context, task ReactorTask, lib *AbstractionLibrary, verifier ReactorVerifier, heuristic *SearchHeuristicProgram) (ParameterizedReactorSearchResult, error) {
	if err := task.Validate(); err != nil {
		return ParameterizedReactorSearchResult{}, err
	}
	if lib == nil {
		return ParameterizedReactorSearchResult{}, errors.New("reactor search requires abstraction library")
	}
	if verifier == nil {
		return ParameterizedReactorSearchResult{}, errors.New("reactor search requires an evaluator-owned verifier")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// String tasks are outside the ArchitectureCandidate vocabulary. Dispatch
	// directly to the T3 synthesized-program frontier rather than spending
	// iterations on an inapplicable T2 procedure grammar.
	if task.InputKind == "string" {
		programVerifier, ok := verifier.(SynthesizedProgramReactorVerifier)
		if !ok {
			return ParameterizedReactorSearchResult{}, errors.New("string domain escape requires synthesized-program verification support")
		}
		program, stats, err := SynthesizeDomainEscapeWithHeuristic(ctx, task, heuristic)
		if err != nil {
			return ParameterizedReactorSearchResult{}, fmt.Errorf("domain-escape synthesis failed after primitive exhaustion: %w", err)
		}
		if err := programVerifier.VerifySynthesizedProgram(ctx, task, program); err != nil {
			return ParameterizedReactorSearchResult{}, fmt.Errorf("domain-escape hidden verification rejected candidate: %w", err)
		}
		return ParameterizedReactorSearchResult{
			SynthesizedProgram:  &program,
			EvaluatedCandidates: stats.CandidatesEvaluated,
			Depth:               1,
		}, nil
	}

	evaluated := 0
	for depth := task.MinProcedureSteps; depth <= task.MaxSearchDepth; depth++ {
		procedures := EnumerateAcquisitionProceduresWithLibrary(depth, lib)
		orderedProcedures, err := orderAcquisitionProcedureCandidates(procedures, heuristic)
		if err != nil {
			return ParameterizedReactorSearchResult{}, fmt.Errorf("active search heuristic rejected procedure frontier: %w", err)
		}
		for _, procedure := range orderedProcedures {
			if len(procedure.Steps) != depth {
				continue
			}
			if task.RequireAbstractionID != "" && !procedureCallsAbstraction(procedure, task.RequireAbstractionID) {
				continue
			}
			evaluated++
			if err := ctx.Err(); err != nil {
				return ParameterizedReactorSearchResult{}, err
			}
			if procedureFitsReactorExamples(procedure, task.Examples, lib) &&
				verifier.Verify(ctx, task, procedure, lib) == nil {
				return ParameterizedReactorSearchResult{
					Procedure:           procedure,
					EvaluatedCandidates: evaluated,
					Depth:               depth,
					UsedAbstractionID:   task.RequireAbstractionID,
				}, nil
			}
		}
	}
	return ParameterizedReactorSearchResult{}, fmt.Errorf(
		"parameterized reactor search exhausted depth=%d..%d candidates=%d",
		task.MinProcedureSteps,
		task.MaxSearchDepth,
		evaluated,
	)
}

func procedureCallsAbstraction(p AcquisitionProcedure, id string) bool {
	for _, step := range p.Steps {
		if step.Op == "call" && step.Ref == id {
			return true
		}
	}
	return false
}

func reactorCandidates(labels []string) []ArchitectureCandidate {
	out := make([]ArchitectureCandidate, len(labels))
	for i, label := range labels {
		out[i] = ArchitectureCandidate{
			ID:        Hash([]any{"reactor-candidate", label, i}),
			Mechanism: label,
			Resources: ResourceVector{Compute: float64(i + 1), ExperimentBudget: 1},
		}
	}
	return out
}

func procedureFitsReactorExamples(procedure AcquisitionProcedure, examples []ReactorExample, lib *AbstractionLibrary) bool {
	for _, example := range examples {
		input := reactorCandidates(example.Input)
		got, err := executeSearchProcedureWithLibrary(procedure, input, lib)
		if err != nil {
			return false
		}
		expected := reactorCandidates(example.Expected)
		if !equalMechanismOrders(got, expected) {
			return false
		}
		reference := referenceProcedure(procedure, input, lib, map[string]bool{})
		if !equalMechanismOrders(reference, got) {
			return false
		}
	}
	return true
}

type ReactorTaskResult struct {
	TaskID                  string
	Family                  string
	Solved                  bool
	EvaluatedCandidates     int
	SearchDepth             int
	UsedAbstractionID       string
	DiscoveredAbstractionID string
	AdmissionRef            string
	Error                   string
	Autotelic               bool
}

type ContinuousReactor struct {
	Runtime             *AdaptiveAcquisitionRuntime
	Queue               ReactorTaskQueue
	Admissions          AbstractionAdmissionSource
	PersistentLibrary   *PersistentAbstractionLibrary
	Verifier            ReactorVerifier
	TrustedSignerID     string
	TrustedPublicKeyB64 string
	MaxTasks             int
	AutotelicGenerator   *AutotelicTaskGenerator
	MaxAutotelicTasks    int
	SubstrateEscape      *AxonSubstrateController
	MaxSubstrateEscapes  int
	Logf                func(string, ...any)
}

func (r *ContinuousReactor) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

func (r *ContinuousReactor) Hydrate(ctx context.Context) error {
	if r.Runtime == nil || r.Queue == nil || r.PersistentLibrary == nil || r.Admissions == nil || r.Verifier == nil {
		return errors.New("continuous reactor requires runtime, queue, persistent library, and admission source")
	}
	if r.TrustedSignerID == "" || r.TrustedPublicKeyB64 == "" {
		return errors.New("continuous reactor requires an external trusted signer identity and public key")
	}
	r.Runtime.TrustedKMSPublicKeyB64 = r.TrustedPublicKeyB64
	if r.Runtime.KMSSignerID != "" && r.Runtime.KMSSignerID != r.TrustedSignerID {
		return fmt.Errorf("runtime signer %q differs from reactor trusted signer %q", r.Runtime.KMSSignerID, r.TrustedSignerID)
	}
	r.Runtime.KMSSignerID = r.TrustedSignerID
	if err := r.PersistentLibrary.RestoreWithTrustedAdmissions(
		ctx,
		&r.Runtime.Abstractions,
		map[string]string{r.TrustedSignerID: r.TrustedPublicKeyB64},
		r.Admissions,
	); err != nil {
		return err
	}
	return r.Runtime.ensureActiveSearchHeuristic(ctx, r.PersistentLibrary)
}

func (r *ContinuousReactor) Run(ctx context.Context) ([]ReactorTaskResult, error) {
	if err := r.Hydrate(ctx); err != nil {
		return nil, err
	}
	results := []ReactorTaskResult{}
	autotelicGenerated := 0
	substrateGenerated := 0
	autotelicLimit := r.MaxAutotelicTasks
	if autotelicLimit <= 0 {
		autotelicLimit = 1
	}
	for r.MaxTasks <= 0 || len(results) < r.MaxTasks {
		substrateLimit := r.MaxSubstrateEscapes
		if substrateLimit <= 0 {
			substrateLimit = 1
		}
		if r.SubstrateEscape != nil && substrateGenerated < substrateLimit && autotelicGenerated >= autotelicLimit {
			if queue, ok := r.Queue.(interface{ Empty() bool }); ok && queue.Empty() {
				task, taskErr := r.AutotelicGenerator.GenerateSubstrateEscapeTask(ctx, &r.Runtime.Abstractions, r.Runtime.ActiveSearchHeuristic)
				if taskErr != nil {
					return results, fmt.Errorf("substrate escape task generation failed: %w", taskErr)
				}
				execution, sealed, runErr := r.SubstrateEscape.RunAndSeal(ctx, r.Runtime, task)
				if runErr != nil {
					return results, fmt.Errorf("substrate escape failed: %w", runErr)
				}
				results = append(results, ReactorTaskResult{
					TaskID:                  task.ID,
					Family:                  "t6-substrate-escape",
					Solved:                  true,
					EvaluatedCandidates:     execution.WorkerCount,
					DiscoveredAbstractionID: sealed.ID,
					AdmissionRef:            sealed.LedgerAdmissionRef,
					Autotelic:               true,
				})
				substrateGenerated++
				r.logf("T6_AXON task=%s pivoted=%t workers=%d admission=%s", task.ID, execution.PivotedFromFuel, execution.WorkerCount, sealed.LedgerAdmissionRef)
				return results, nil
			}
		}
		if r.AutotelicGenerator != nil && autotelicGenerated < autotelicLimit {
			if queue, ok := r.Queue.(interface{ Empty() bool }); ok && queue.Empty() {
				bundle, genErr := r.AutotelicGenerator.Generate(ctx, &r.Runtime.Abstractions, r.Runtime.ActiveSearchHeuristic)
				if errors.Is(genErr, ErrNoAutotelicGap) {
					if r.SubstrateEscape != nil && substrateGenerated < substrateLimit {
						task, taskErr := r.AutotelicGenerator.GenerateSubstrateEscapeTask(ctx, &r.Runtime.Abstractions, r.Runtime.ActiveSearchHeuristic)
						if taskErr != nil {
							return results, fmt.Errorf("substrate escape task generation failed: %w", taskErr)
						}
						execution, sealed, runErr := r.SubstrateEscape.RunAndSeal(ctx, r.Runtime, task)
						if runErr != nil {
							return results, fmt.Errorf("substrate escape failed: %w", runErr)
						}
						results = append(results, ReactorTaskResult{
							TaskID:                  task.ID,
							Family:                  "t6-substrate-escape",
							Solved:                  true,
							EvaluatedCandidates:     execution.WorkerCount,
							DiscoveredAbstractionID: sealed.ID,
							AdmissionRef:            sealed.LedgerAdmissionRef,
							Autotelic:               true,
						})
						substrateGenerated++
						r.logf("T6_AXON task=%s pivoted=%t workers=%d admission=%s", task.ID, execution.PivotedFromFuel, execution.WorkerCount, sealed.LedgerAdmissionRef)
						return results, nil
					}
					return results, nil
				}
				if genErr != nil {
					return results, fmt.Errorf("autotelic task generation failed: %w", genErr)
				}
				if bundle.Complexity != bundle.BoundaryScore+1 {
					return results, fmt.Errorf("autotelic complexity gate violated: boundary=%d complexity=%d", bundle.BoundaryScore, bundle.Complexity)
				}
				verifier := newAutotelicHiddenVerifier(r.Verifier, bundle)
				result := r.runOneWithVerifier(ctx, bundle.Task, verifier)
				result.Autotelic = true
				results = append(results, result)
				autotelicGenerated++
				r.logf("T5_AUTOTELIC task=%s gap=%s solved=%t boundary=%d complexity=%d candidates=%d error=%q", bundle.Task.ID, bundle.GapClass, result.Solved, bundle.BoundaryScore, bundle.Complexity, result.EvaluatedCandidates, result.Error)
				if !result.Solved {
					return results, fmt.Errorf("autotelic task %s failed: %s", bundle.Task.ID, result.Error)
				}
				continue
			}
		}

		lease, err := r.Queue.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return results, nil
			}
			return results, err
		}
		task := lease.Task()
		result := r.runOne(ctx, task)
		results = append(results, result)
		if result.Solved {
			if err := lease.Ack(); err != nil {
				return results, fmt.Errorf("ack task %s: %w", task.ID, err)
			}
		} else {
			failErr := errors.New(result.Error)
			if result.Error == "" {
				failErr = errors.New("reactor task failed")
			}
			if err := lease.Fail(failErr); err != nil {
				return results, fmt.Errorf("fail task %s: %w", task.ID, err)
			}
		}
		r.logf(
			"T2_REACTOR task=%s family=%s solved=%t used=%s discovered=%s depth=%d candidates=%d error=%q",
			result.TaskID,
			result.Family,
			result.Solved,
			result.UsedAbstractionID,
			result.DiscoveredAbstractionID,
			result.SearchDepth,
			result.EvaluatedCandidates,
			result.Error,
		)
	}
	return results, nil
}

func latestCapabilityAbstractionID(lib AbstractionLibrary) string {
	for i := len(lib.Abstractions) - 1; i >= 0; i-- {
		if lib.Abstractions[i].ArtifactType == SearchHeuristicArtifactType {
			continue
		}
		return lib.Abstractions[i].ID
	}
	return ""
}

func (r *ContinuousReactor) runOne(ctx context.Context, task ReactorTask) ReactorTaskResult {
	return r.runOneWithVerifier(ctx, task, r.Verifier)
}

func (r *ContinuousReactor) runOneWithVerifier(ctx context.Context, task ReactorTask, verifier ReactorVerifier) ReactorTaskResult {
	result := ReactorTaskResult{TaskID: task.ID, Family: task.Family}
	if err := task.Validate(); err != nil {
		result.Error = err.Error()
		return result
	}
	if task.MetaKind != "" {
		return r.runMetaTask(ctx, task)
	}
	requireID := task.RequireAbstractionID
	if task.RequireLatestAdmission {
		requireID = latestCapabilityAbstractionID(r.Runtime.Abstractions)
		if requireID == "" {
			result.Error = "task requires a previous admitted capability abstraction, but none is installed"
			return result
		}
	}
	if requireID != "" {
		task.RequireAbstractionID = requireID
	}
	searchResult, err := ParameterizedMechanismSearchWithHeuristic(ctx, task, &r.Runtime.Abstractions, verifier, r.Runtime.ActiveSearchHeuristic)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Solved = true
	result.EvaluatedCandidates = searchResult.EvaluatedCandidates
	result.SearchDepth = searchResult.Depth
	result.UsedAbstractionID = searchResult.UsedAbstractionID

	if searchResult.SynthesizedProgram != nil {
		if !task.AdmitAsAbstraction {
			return result
		}
		program := *searchResult.SynthesizedProgram
		artifactIDPrefix := "t3-domain-escape"
		artifactName := "t3-domain-escape:" + task.Family + ":" + task.ID
		if task.Autotelic {
			artifactIDPrefix = "autotelic-domain-escape"
			artifactName = "autotelic:" + task.GapClass
		}
		abstraction := AcquiredAbstraction{
			ID:               Hash([]any{artifactIDPrefix, task.ID, program}),
			Name:             artifactName,
			ArtifactType:     SynthesizedProgramArtifactType,
			SynthesizedProgram: &program,
			Contract: AbstractionContract{
				Inputs:         []string{task.InputKind},
				Outputs:        []string{"string"},
				Preconditions:  []string{"bounded UTF-8 input", "sandbox fuel limit enforced"},
				Postconditions: []string{"deterministic output under independently verified program semantics"},
			},
			Evidence: []AbstractionEvidence{{
				TaskStructure: task.Family,
				Verified:      true,
				HeldOut:       true,
				TransferScore: 1,
				DiscoveryCost: ResourceVector{
					ExperimentBudget: float64(searchResult.EvaluatedCandidates),
					TimeMS:            1,
				},
				ObservedGain: 1,
			}},
			CostHistory: []ResourceVector{{
				ExperimentBudget: float64(searchResult.EvaluatedCandidates),
				TimeMS:            1,
			}},
			Verification: VerificationResult{
				Status:      "verified",
				Independent: true,
				Expected:    []string{"training examples preserved", "evaluator-owned holdout behavior"},
				Observed:    []string{"sandbox execution agreement", "independent hidden verification agreement"},
				Provenance:  Prov("t3-domain-escape-verifier", task.ID, "sandboxed-program-plus-hidden-reference", program),
			},
			Provenance: Prov("t3-domain-escape", task.ID, "synthesized-program-admission", program),
		}
		sealed, err := r.Runtime.admitAbstraction(ctx, abstraction, 1)
		if err != nil {
			result.Solved = false
			result.Error = err.Error()
			return result
		}
		result.DiscoveredAbstractionID = sealed.ID
		result.AdmissionRef = sealed.LedgerAdmissionRef
		if r.PersistentLibrary != nil {
			if err := r.PersistentLibrary.Save(&r.Runtime.Abstractions); err != nil {
				result.Solved = false
				result.Error = fmt.Sprintf("persist admitted T3 program library: %v", err)
				return result
			}
		}
		return result
	}

	if !task.AdmitAsAbstraction {
		return result
	}

	abstraction := AcquiredAbstraction{
		ID:        Hash([]any{"t2-reactor-abstraction", task.ID, searchResult.Procedure}),
		Name:      "t2-reactor:" + task.Family + ":" + task.ID,
		Procedure: searchResult.Procedure,
		Contract: AbstractionContract{
			Inputs:         []string{"ordered-candidate-stream"},
			Outputs:        []string{"ordered-candidate-stream"},
			Preconditions:  []string{"bounded candidate stream"},
			Postconditions: []string{"verified deterministic stream transformation"},
		},
		Evidence: []AbstractionEvidence{{
			TaskStructure: task.Family,
			Verified:      true,
			HeldOut:       true,
			TransferScore: 1,
			DiscoveryCost: ResourceVector{
				ExperimentBudget: float64(searchResult.EvaluatedCandidates),
				TimeMS:            1,
			},
			ObservedGain: 1,
		}},
		CostHistory: []ResourceVector{{
			ExperimentBudget: float64(searchResult.EvaluatedCandidates),
			TimeMS:            1,
		}},
		Verification: VerificationResult{
			Status:      "verified",
			Independent: true,
			Expected:    []string{"train and hidden stream behavior"},
			Observed:    []string{"independent reference interpreter agreement"},
			Provenance:  Prov("t2-reactor-verifier", task.ID, "independent-reference-verification", searchResult.Procedure),
		},
		Provenance: Prov("t2-continuous-reactor", task.ID, "verified-acquisition", searchResult.Procedure),
	}
	sealed, err := r.Runtime.admitAbstraction(ctx, abstraction, 1)
	if err != nil {
		result.Solved = false
		result.Error = err.Error()
		return result
	}
	result.DiscoveredAbstractionID = sealed.ID
	result.AdmissionRef = sealed.LedgerAdmissionRef
	if r.PersistentLibrary != nil {
		if err := r.PersistentLibrary.Save(&r.Runtime.Abstractions); err != nil {
			result.Solved = false
			result.Error = fmt.Sprintf("persist admitted abstraction library: %v", err)
			return result
		}
	}
	return result
}
