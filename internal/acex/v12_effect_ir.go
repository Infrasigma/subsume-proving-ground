package acex

import "fmt"

// V12IRToken is a linear capability token. State tokens are never duplicated or
// implicitly reused across an effect boundary.
type V12IRToken struct {
	ID   uint32
	Kind string
}

// V12PureOp describes a side-effect-free operation. Pure operations may
// synthesize/inspect representations but cannot consume linear state.
type V12PureOp struct {
	Kind      string
	Produces  []uint32
}

// V12EffectOp explicitly crosses the effect boundary.
type V12EffectOp struct {
	Kind     string
	Consumes []uint32
	Produces []uint32
}

// V12EffectIR is the smallest effect-safe executable boundary for V11.
// The intended dataflow is:
//   state-before -> consume(state-before, action) -> state-after
// where the action token is produced by a pure symbolic match.
type V12EffectIR struct {
	Version       int
	Pattern       V11DirectedPatternArtifact
	Tokens        []V12IRToken
	PureOps       []V12PureOp
	EffectOps     []V12EffectOp
}

func BuildV12EffectIR(r V11DirectedExecutableRepresentation) (V12EffectIR, error) {
	if !r.Valid || r.Nodes < 2 || r.Root != 0 || len(r.Edges) == 0 {
		return V12EffectIR{}, fmt.Errorf("cannot lower invalid directed representation")
	}
	p := V11DirectedPatternArtifact{
		Version: 1,
		Root:    r.Root,
		Nodes:   r.Nodes,
		Edges:   append([]V11DirectedPatternEdge(nil), r.Edges...),
	}
	ir := V12EffectIR{
		Version: 1,
		Pattern: p,
		Tokens: []V12IRToken{
			{ID: 1, Kind: "state-before"},
			{ID: 2, Kind: "action"},
			{ID: 3, Kind: "state-after"},
		},
		PureOps: []V12PureOp{
			{Kind: "directed-pattern-match", Produces: []uint32{2}},
		},
		EffectOps: []V12EffectOp{
			{Kind: "execute-action", Consumes: []uint32{1, 2}, Produces: []uint32{3}},
		},
	}
	if err := ir.Validate(); err != nil {
		return V12EffectIR{}, err
	}
	return ir, nil
}

func (ir V12EffectIR) Validate() error {
	if ir.Version != 1 {
		return fmt.Errorf("unsupported effect IR version %d", ir.Version)
	}
	if ir.Pattern.Version != 1 || ir.Pattern.Root != 0 || ir.Pattern.Nodes < 2 || ir.Pattern.Nodes > 4 {
		return fmt.Errorf("invalid retained pattern boundary")
	}
	tokenKinds := map[uint32]string{}
	for _, t := range ir.Tokens {
		if t.ID == 0 || t.Kind == "" {
			return fmt.Errorf("invalid IR token")
		}
		if _, exists := tokenKinds[t.ID]; exists {
			return fmt.Errorf("duplicate IR token id %d", t.ID)
		}
		tokenKinds[t.ID] = t.Kind
	}
	if len(ir.PureOps) != 1 || ir.PureOps[0].Kind != "directed-pattern-match" {
		return fmt.Errorf("unexpected pure IR shape")
	}
	if len(ir.PureOps[0].Produces) != 1 || ir.PureOps[0].Produces[0] != 2 {
		return fmt.Errorf("pure match must produce the action token")
	}
	for _, id := range ir.PureOps[0].Produces {
		if _, ok := tokenKinds[id]; !ok {
			return fmt.Errorf("pure op references unknown token %d", id)
		}
	}
	if len(ir.EffectOps) != 1 || ir.EffectOps[0].Kind != "execute-action" {
		return fmt.Errorf("unexpected effect IR shape")
	}
	effect := ir.EffectOps[0]
	if len(effect.Consumes) != 2 || len(effect.Produces) != 1 {
		return fmt.Errorf("effect op must consume two tokens and produce one")
	}
	if effect.Consumes[0] != 1 || effect.Consumes[1] != 2 || effect.Produces[0] != 3 {
		return fmt.Errorf("effect token flow is not linear")
	}
	for _, id := range effect.Consumes {
		if _, ok := tokenKinds[id]; !ok {
			return fmt.Errorf("effect op references unknown token %d", id)
		}
	}
	if _, ok := tokenKinds[effect.Produces[0]]; !ok {
		return fmt.Errorf("effect op produces unknown token %d", effect.Produces[0])
	}
	// A state-before token may be consumed exactly once. The action token is
	// produced by the pure phase and then consumed exactly once at the effect
	// boundary. The produced state-after token cannot be consumed in this IR.
	consumed := map[uint32]bool{}
	for _, id := range effect.Consumes {
		if consumed[id] {
			return fmt.Errorf("linear token %d consumed twice", id)
		}
		consumed[id] = true
	}
	if effect.Produces[0] == 1 || effect.Produces[0] == 2 {
		return fmt.Errorf("effect boundary illegally aliases an input token")
	}
	return nil
}
