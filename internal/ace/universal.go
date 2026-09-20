package ace

import("context";"encoding/json";"errors";"fmt";"strconv";"strings")
type UExpr struct{Kind string `json:"kind"`;Value string `json:"value,omitempty"`;Left *UExpr `json:"left,omitempty"`;Right *UExpr `json:"right,omitempty"`}
type UStmt struct{Kind string `json:"kind"`;Target string `json:"target,omitempty"`;Expr *UExpr `json:"expr,omitempty"`;Cond *UExpr `json:"cond,omitempty"`;Then []UStmt `json:"then,omitempty"`;Else []UStmt `json:"else,omitempty"`;Body []UStmt `json:"body,omitempty"`;Count int `json:"count,omitempty"`}
type UniversalProgram struct{Statements []UStmt `json:"statements"`}
func(e UExpr)eval(env map[string]string)(int,bool,error){switch e.Kind{case"const":n,err:=strconv.Atoi(e.Value);return n,err==nil,err;case"var":v,ok:=env[e.Value];if !ok{return 0,false,fmt.Errorf("missing variable %s",e.Value)};n,err:=strconv.Atoi(v);return n,err==nil,err;case"add","sub","mul":if e.Left==nil||e.Right==nil{return 0,false,errors.New("binary expression missing operand")};a,_,err:=e.Left.eval(env);if err!=nil{return 0,false,err};b,_,err:=e.Right.eval(env);if err!=nil{return 0,false,err};switch e.Kind{case"add":return a+b,true,nil;case"sub":return a-b,true,nil;default:return a*b,true,nil};case"lt","eq","and","or":if e.Left==nil||e.Right==nil{return 0,false,errors.New("boolean expression missing operand")};a,_,err:=e.Left.eval(env);if err!=nil{return 0,false,err};b,_,err:=e.Right.eval(env);if err!=nil{return 0,false,err};switch e.Kind{case"lt":if a<b{return 1,true,nil};case"eq":if a==b{return 1,true,nil};case"and":if a!=0&&b!=0{return 1,true,nil};case"or":if a!=0||b!=0{return 1,true,nil}};return 0,true,nil;case"not":if e.Left==nil{return 0,false,errors.New("not missing operand")};a,_,err:=e.Left.eval(env);if err!=nil{return 0,false,err};if a==0{return 1,true,nil};return 0,true,nil;default:return 0,false,fmt.Errorf("unknown expression %s",e.Kind)}}
func(p UniversalProgram)Run(input map[string]string)(map[string]string,error){env:=map[string]string{};for k,v:=range input{env[k]=v};steps:=0;var exec func([]UStmt)error;exec=func(ss []UStmt)error{for _,s:=range ss{steps++;if steps>1000{return errors.New("program step limit exceeded")};switch s.Kind{case"assign":if s.Expr==nil{return errors.New("assign missing expression")};n,_,err:=s.Expr.eval(env);if err!=nil{return err};env[s.Target]=strconv.Itoa(n);case"if":if s.Cond==nil{return errors.New("if missing condition")};n,_,err:=s.Cond.eval(env);if err!=nil{return err};if n!=0{if err:=exec(s.Then);err!=nil{return err}}else{if err:=exec(s.Else);err!=nil{return err}};case"repeat":if s.Count<0||s.Count>100{return errors.New("invalid repeat count")};for i:=0;i<s.Count;i++{if err:=exec(s.Body);err!=nil{return err}};default:return fmt.Errorf("unknown statement %s",s.Kind)}};return nil};if err:=exec(p.Statements);err!=nil{return nil,err};return env,nil}
func programFits(p UniversalProgram,cases []ProgramTestCase)bool{for _,tc:=range cases{got,err:=p.Run(tc.Input);if err!=nil{return false};for k,v:=range tc.Expected{if got[k]!=v{return false}}};return true}
func serializedProgramFits(p UniversalProgram,cases []ProgramTestCase)bool{b,err:=json.Marshal(p);if err!=nil{return false};var q UniversalProgram;if err=json.Unmarshal(b,&q);err!=nil{return false};return programFits(q,cases)}
func GeneralCapabilitySpecification(t Task,cases []ProgramTestCase)(CapabilitySpecification,error){if len(cases)<2{return CapabilitySpecification{},errors.New("general capability requires at least two examples")};inputs,outputs:=map[string]bool{},map[string]bool{};for _,c:=range cases{for k:=range c.Input{inputs[k]=true};for k:=range c.Expected{outputs[k]=true}};in,out:=make([]string,0,len(inputs)),make([]string,0,len(outputs));for k:=range inputs{in=append(in,k)};for k:=range outputs{out=append(out,k)};return CapabilitySpecification{ID:Hash([]any{"general",t.ID,cases}),DesiredBehaviour:t.Goal,Inputs:in,Outputs:out,Invariants:[]string{"all observed examples preserved"},AcceptanceTests:[]string{"observed-examples","held-out-execution"},ResourceLimits:t.Budget,FailureCriteria:[]string{"wrong output","runtime error"},RegressionConstraints:[]string{"existing capabilities preserved"},KnownExamples:cases,Provenance:Prov("capability-discovery",t.ID,"infer-from-examples",cases)},nil}
type UniversalMechanismSearch struct{}
func(UniversalMechanismSearch)SearchMechanisms(s CapabilitySpecification,b ResourceVector)([]ArchitectureCandidate,error){strategies:=[]string{"universal:straight-line","universal:branching","universal:compositional"};out:=make([]ArchitectureCandidate,0,len(strategies));for _,strategy:=range strategies{out=append(out,ArchitectureCandidate{ID:Hash([]any{s.ID,strategy}),Mechanism:strategy,Interfaces:[]string{"typed-key-value-input","executable-program"},Advantage:"compositional synthesis",Assumptions:"bounded integer/boolean primitives",Resources:b,Tests:s.AcceptanceTests,RegressionRisks:s.RegressionConstraints,Provenance:Prov("architecture-search",s.ID,"strategy",strategy)})};return out,nil}
func cloneExpr(e UExpr)*UExpr{x:=e;if e.Left!=nil{x.Left=cloneExpr(*e.Left)};if e.Right!=nil{x.Right=cloneExpr(*e.Right)};return &x}
// expressionFrontier enumerates exact structural depths while sharing immutable
// child nodes. This avoids exponential deep-copy overhead and recursive frontier growth.
func expressionFrontier(vars []string,maxDepth int)[]UExpr{base:=make([]UExpr,0,len(vars)+5);for _,v:=range vars{base=append(base,UExpr{Kind:"var",Value:v})};for n:=-2;n<=2;n++{base=append(base,UExpr{Kind:"const",Value:strconv.Itoa(n)})};front:=append([]UExpr(nil),base...);prev:=base;for depth:=1;depth<=maxDepth;depth++{next:=make([]UExpr,0,len(prev)*len(prev)*5);for i:=range prev{for j:=range prev{a,b:=&prev[i],&prev[j];next=append(next,UExpr{Kind:"add",Left:a,Right:b},UExpr{Kind:"sub",Left:a,Right:b},UExpr{Kind:"mul",Left:a,Right:b},UExpr{Kind:"lt",Left:a,Right:b},UExpr{Kind:"eq",Left:a,Right:b})}};front=append(front,next...);prev=next};return front}
var ErrSynthesisExpansionLimit = errors.New("universal program synthesis expansion budget exhausted")

type UniversalProgramBuilder struct {
	// MaxSynthesisExpansions bounds candidate AST/program construction for a
	// single synthesis phase. Zero means unlimited for legacy callers.
	MaxSynthesisExpansions int
}

type synthesisBudget struct {
	remaining int
}

func newSynthesisBudget(limit int) *synthesisBudget {
	if limit <= 0 {
		return nil
	}
	return &synthesisBudget{remaining: limit}
}

func (b *synthesisBudget) consume() error {
	if b == nil {
		return nil
	}
	if b.remaining <= 0 {
		return ErrSynthesisExpansionLimit
	}
	b.remaining--
	return nil
}

func(UniversalProgramBuilder)Build(c ArchitectureCandidate,s CapabilitySpecification)(ModificationProposal,error){if len(s.KnownExamples)<2||len(s.Inputs)==0||len(s.Outputs)==0{return ModificationProposal{},errors.New("insufficient behavioral evidence")};vars,out:=append([]string{},s.Inputs...),s.Outputs[0];if strings.HasPrefix(c.Mechanism,"universal:branching")||strings.HasPrefix(c.Mechanism,"universal:compositional"){branchExprs:=expressionFrontier(vars,1);for _,v:=range vars{for _,cmp:=range []string{"lt","eq"}{for _,rhs:=range branchExprs{cond:=UExpr{Kind:cmp,Left:&UExpr{Kind:"var",Value:v},Right:cloneExpr(rhs)};for _,te:=range branchExprs{for _,ee:=range branchExprs{p:=UniversalProgram{Statements:[]UStmt{{Kind:"if",Cond:&cond,Then:[]UStmt{{Kind:"assign",Target:out,Expr:cloneExpr(te)}},Else:[]UStmt{{Kind:"assign",Target:out,Expr:cloneExpr(ee)}}}}};if programFits(p,s.KnownExamples){return encodeUniversal(p,s,c)}}}}}}};for _,e:=range expressionFrontier(vars,2){p:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:out,Expr:cloneExpr(e)}}};if programFits(p,s.KnownExamples){return encodeUniversal(p,s,c)}};return ModificationProposal{},errors.New("universal synthesis exhausted search space")}
func encodeUniversal(p UniversalProgram,s CapabilitySpecification,c ArchitectureCandidate)(ModificationProposal,error){b,err:=json.Marshal(p);if err!=nil{return ModificationProposal{},err};var q UniversalProgram;if err:=json.Unmarshal(b,&q);err!=nil||!programFits(q,s.KnownExamples){return ModificationProposal{},errors.New("serialized artifact failed behavioral validation")};return ModificationProposal{ID:Hash([]any{c,s,p}),Capability:s,Candidate:c,Artifact:string(b),Provenance:Prov("mechanism-builder",c.ID,"synthesize-universal-program",p)},nil}
func GenerateAdversarialCases(s CapabilitySpecification)[]ProgramTestCase{out:=make([]ProgramTestCase,0,8);for _,c:=range s.KnownExamples{for k,v:=range c.Input{n,err:=strconv.Atoi(v);if err!=nil{continue};for _,d:=range []int{-1,1}{in:=map[string]string{};for ik,iv:=range c.Input{in[ik]=iv};in[k]=strconv.Itoa(n+d);out=append(out,ProgramTestCase{Input:in})}}};return out}


func expressionFrontierWithContext(ctx context.Context, vars []string, maxDepth int) ([]UExpr, error) {
	return expressionFrontierWithContextBudget(ctx, vars, maxDepth, nil)
}

func expressionFrontierWithContextBudget(ctx context.Context, vars []string, maxDepth int, budget *synthesisBudget) ([]UExpr, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	base := make([]UExpr, 0, len(vars)+7)
	for _, v := range vars {
		if err := budget.consume(); err != nil {
			// Preserve every shallow expression already constructed so callers
			// can verify them before treating the expansion limit as terminal.
			return append([]UExpr(nil), base...), err
		}
		base = append(base, UExpr{Kind: "var", Value: v})
	}
	for _, n := range []int{3, 2, 1, 0, -1, -2, -3} {
		if err := budget.consume(); err != nil {
			return append([]UExpr(nil), base...), err
		}
		base = append(base, UExpr{Kind: "const", Value: strconv.Itoa(n)})
	}
	front := append([]UExpr(nil), base...)
	prev := base
	for depth := 1; depth <= maxDepth; depth++ {
		next := make([]UExpr, 0)
		for i := range prev {
			if err := ctx.Err(); err != nil {
				return front, err
			}
			for j := range prev {
				if (j & 255) == 0 {
					if err := ctx.Err(); err != nil {
						return front, err
					}
				}
				a, b := &prev[i], &prev[j]
				exprs := [...]UExpr{
					{Kind: "add", Left: a, Right: b},
					{Kind: "sub", Left: a, Right: b},
					{Kind: "mul", Left: a, Right: b},
					{Kind: "lt", Left: a, Right: b},
					{Kind: "eq", Left: a, Right: b},
				}
				for _, expr := range exprs {
					if err := budget.consume(); err != nil {
						// The previous complete depth is still a valid bounded
						// search frontier; return it with an explicit exhaustion
						// error instead of returning a malformed partial program.
						return front, err
					}
					next = append(next, expr)
				}
			}
		}
		front = append(front, next...)
		prev = next
	}
	return front, nil
}

func shallowExpressionSet(vars []string) []UExpr {
	out := make([]UExpr, 0, len(vars)+7)
	for _, v := range vars {
		out = append(out, UExpr{Kind: "var", Value: v})
	}
	for _, n := range []int{3, 2, 1, 0, -1, -2, -3} {
		out = append(out, UExpr{Kind: "const", Value: strconv.Itoa(n)})
	}
	return out
}

func (b UniversalProgramBuilder) BuildWithContext(ctx context.Context, c ArchitectureCandidate, s CapabilitySpecification) (ModificationProposal, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(s.KnownExamples) < 2 || len(s.Inputs) == 0 || len(s.Outputs) == 0 {
		return ModificationProposal{}, errors.New("insufficient behavioral evidence")
	}
	if err := ctx.Err(); err != nil {
		return ModificationProposal{}, err
	}
	vars, out := append([]string{}, s.Inputs...), s.Outputs[0]

	if strings.HasPrefix(c.Mechanism, "universal:branching") || strings.HasPrefix(c.Mechanism, "universal:compositional") {
		branchBudget := newSynthesisBudget(b.MaxSynthesisExpansions)
		shallow := shallowExpressionSet(vars)
		shallowExhausted := false
	shallowLoop:
		for _, v := range vars {
			for _, cmp := range []string{"lt", "eq"} {
				for _, rhs := range shallow {
					cond := UExpr{Kind: cmp, Left: &UExpr{Kind: "var", Value: v}, Right: cloneExpr(rhs)}
					for _, te := range shallow {
						for _, ee := range shallow {
							if err := branchBudget.consume(); err != nil {
								shallowExhausted = true
								break shallowLoop
							}
							p := UniversalProgram{Statements: []UStmt{{Kind: "if", Cond: &cond, Then: []UStmt{{Kind: "assign", Target: out, Expr: cloneExpr(te)}}, Else: []UStmt{{Kind: "assign", Target: out, Expr: cloneExpr(ee)}}}}}
							if programFits(p, s.KnownExamples) {
								return encodeUniversal(p, s, c)
							}
						}
					}
				}
			}
		}
		branchExprs, frontierErr := expressionFrontierWithContextBudget(ctx, vars, 1, branchBudget)
		if !shallowExhausted && frontierErr == nil {
			branchExhausted := false
		branchLoop:
			for _, v := range vars {
				for _, cmp := range []string{"lt", "eq"} {
					for _, rhs := range branchExprs {
						if err := ctx.Err(); err != nil {
							return ModificationProposal{}, err
						}
						cond := UExpr{Kind: cmp, Left: &UExpr{Kind: "var", Value: v}, Right: cloneExpr(rhs)}
						for _, te := range branchExprs {
							for _, ee := range branchExprs {
								if err := branchBudget.consume(); err != nil {
									branchExhausted = true
									break branchLoop
								}
								p := UniversalProgram{Statements: []UStmt{{Kind: "if", Cond: &cond, Then: []UStmt{{Kind: "assign", Target: out, Expr: cloneExpr(te)}}, Else: []UStmt{{Kind: "assign", Target: out, Expr: cloneExpr(ee)}}}}}
								if programFits(p, s.KnownExamples) {
									return encodeUniversal(p, s, c)
								}
							}
						}
					}
				}
			}
			_ = branchExhausted
		} else if !errors.Is(frontierErr, ErrSynthesisExpansionLimit) {
			return ModificationProposal{}, frontierErr
		}
	}

	straightBudget := newSynthesisBudget(b.MaxSynthesisExpansions)
	exprs, frontierErr := expressionFrontierWithContextBudget(ctx, vars, 2, straightBudget)
	if frontierErr != nil && !errors.Is(frontierErr, ErrSynthesisExpansionLimit) {
		return ModificationProposal{}, frontierErr
	}
	// Always verify the shallow frontier first. This makes a retained derived
	// representation executable without paying for a deeper Cartesian frontier.
	for _, e := range shallowExpressionSet(vars) {
		if err := ctx.Err(); err != nil {
			return ModificationProposal{}, err
		}
		p := UniversalProgram{Statements: []UStmt{{Kind: "assign", Target: out, Expr: cloneExpr(e)}}}
		if programFits(p, s.KnownExamples) {
			return encodeUniversal(p, s, c)
		}
	}
	// Also verify every complete shallow/depth-1 expression already constructed
	// before treating a deeper expansion limit as terminal.
	for _, e := range exprs {
		if err := ctx.Err(); err != nil {
			return ModificationProposal{}, err
		}
		p := UniversalProgram{Statements: []UStmt{{Kind: "assign", Target: out, Expr: cloneExpr(e)}}}
		if programFits(p, s.KnownExamples) {
			return encodeUniversal(p, s, c)
		}
	}
	if frontierErr != nil {
		return ModificationProposal{}, frontierErr
	}
	return ModificationProposal{}, errors.New("universal synthesis exhausted search space")
}
