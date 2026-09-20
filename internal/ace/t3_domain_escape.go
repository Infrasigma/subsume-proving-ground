package ace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	SynthesizedProgramArtifactType = "SynthesizedProgram"
	SynthesizedProgramLanguage     = "ace-bytecode/v1"
	maxSynthInstructions            = 256
	maxSynthRegisters               = 16
	maxSynthMemoryCells             = 1024
	maxSynthInputBytes              = 64 << 10
	maxSynthOutputBytes             = 256 << 10
)

type SynthesizedInstruction struct {
	Op string `json:"op"`
	A  int    `json:"a,omitempty"`
	B  int    `json:"b,omitempty"`
	C  int    `json:"c,omitempty"`
}

type SynthesizedProgram struct {
	Version       int                     `json:"version"`
	Language      string                  `json:"language"`
	Source        string                  `json:"source"`
	Instructions  []SynthesizedInstruction `json:"instructions"`
	Fuel          uint64                  `json:"fuel"`
	MaxInputBytes int                     `json:"max_input_bytes"`
}

func (p SynthesizedProgram) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("synthesized program version must be 1")
	}
	if p.Language != SynthesizedProgramLanguage {
		return fmt.Errorf("unsupported synthesized program language %q", p.Language)
	}
	if len(p.Instructions) == 0 || len(p.Instructions) > maxSynthInstructions {
		return fmt.Errorf("synthesized program instruction count must be 1..%d", maxSynthInstructions)
	}
	if p.Fuel == 0 || p.Fuel > 1_000_000 {
		return errors.New("synthesized program fuel must be 1..1000000")
	}
	if p.MaxInputBytes <= 0 || p.MaxInputBytes > maxSynthInputBytes {
		return fmt.Errorf("synthesized program max input bytes must be 1..%d", maxSynthInputBytes)
	}
	emits, halts := 0, 0
	for i, ins := range p.Instructions {
		check := func(register int) error {
			return validateRegister(register, fmt.Sprintf("instruction %d", i))
		}
		switch ins.Op {
		case "input", "newbuf", "const":
			if err := check(ins.A); err != nil {
				return err
			}
		case "len", "is_vowel", "upper", "lower", "append", "cell_get", "cell_set":
			if err := check(ins.A); err != nil {
				return err
			}
			if err := check(ins.B); err != nil {
				return err
			}
		case "lt", "add", "sub", "char":
			if err := check(ins.A); err != nil {
				return err
			}
			if err := check(ins.B); err != nil {
				return err
			}
			if err := check(ins.C); err != nil {
				return err
			}
		case "jump":
			if ins.A < 0 || ins.A >= len(p.Instructions) {
				return fmt.Errorf("instruction %d jump target out of range", i)
			}
		case "jump_if_false":
			if err := check(ins.A); err != nil {
				return err
			}
			if ins.B < 0 || ins.B >= len(p.Instructions) {
				return fmt.Errorf("instruction %d conditional jump target out of range", i)
			}
		case "emit":
			if err := check(ins.A); err != nil {
				return err
			}
			emits++
		case "halt":
			halts++
		default:
			return fmt.Errorf("unsupported synthesized opcode %q", ins.Op)
		}
	}
	if emits != 1 || halts != 1 {
		return fmt.Errorf("synthesized program must contain exactly one emit and one halt")
	}
	if p.Source != DisassembleSynthesizedProgram(p.Instructions) {
		return errors.New("synthesized program source does not match bytecode")
	}
	return nil
}

func validateRegister(v int, context string) error {
	if v < 0 || v >= maxSynthRegisters {
		return fmt.Errorf("%s register %d out of range", context, v)
	}
	return nil
}

func DisassembleSynthesizedProgram(instructions []SynthesizedInstruction) string {
	var b strings.Builder
	for i, ins := range instructions {
		fmt.Fprintf(&b, "%03d %s %d %d %d\\n", i, ins.Op, ins.A, ins.B, ins.C)
	}
	return b.String()
}

func (p SynthesizedProgram) Execute(ctx context.Context, input string) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if len([]byte(input)) > p.MaxInputBytes {
		return "", errors.New("synthesized sandbox rejected oversized input")
	}
	reg := make([]any, maxSynthRegisters)
	memory := make([]int64, maxSynthMemoryCells)
	pc := 0
	var fuel = p.Fuel

	getInt := func(v any) (int64, error) {
		x, ok := v.(int64)
		if !ok {
			return 0, fmt.Errorf("expected integer register, got %T", v)
		}
		return x, nil
	}
	getString := func(v any) (string, error) {
		x, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("expected string register, got %T", v)
		}
		return x, nil
	}
	getBool := func(v any) (bool, error) {
		x, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("expected boolean register, got %T", v)
		}
		return x, nil
	}

	for pc >= 0 && pc < len(p.Instructions) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		if fuel == 0 {
			return "", errors.New("synthesized sandbox fuel exhausted")
		}
		fuel--
		ins := p.Instructions[pc]
		nextPC := pc + 1
		switch ins.Op {
		case "input":
			reg[ins.A] = input
		case "newbuf":
			reg[ins.A] = ""
		case "const":
			reg[ins.A] = int64(ins.B)
		case "len":
			s, err := getString(reg[ins.B])
			if err != nil {
				return "", err
			}
			reg[ins.A] = int64(len([]rune(s)))
		case "lt":
			l, err := getInt(reg[ins.B])
			if err != nil {
				return "", err
			}
			r, err := getInt(reg[ins.C])
			if err != nil {
				return "", err
			}
			reg[ins.A] = l < r
		case "add", "sub":
			l, err := getInt(reg[ins.B])
			if err != nil {
				return "", err
			}
			r, err := getInt(reg[ins.C])
			if err != nil {
				return "", err
			}
			if ins.Op == "add" {
				reg[ins.A] = l + r
			} else {
				reg[ins.A] = l - r
			}
		case "char":
			s, err := getString(reg[ins.B])
			if err != nil {
				return "", err
			}
			idx, err := getInt(reg[ins.C])
			if err != nil {
				return "", err
			}
			rs := []rune(s)
			if idx < 0 || idx >= int64(len(rs)) {
				return "", fmt.Errorf("char index %d out of range", idx)
			}
			reg[ins.A] = string(rs[idx])
		case "is_vowel":
			s, err := getString(reg[ins.B])
			if err != nil {
				return "", err
			}
			rs := []rune(s)
			if len(rs) != 1 {
				return "", errors.New("is_vowel expects a single rune")
			}
			switch rs[0] {
			case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
				reg[ins.A] = true
			default:
				reg[ins.A] = false
			}
		case "upper", "lower":
			s, err := getString(reg[ins.B])
			if err != nil {
				return "", err
			}
			if ins.Op == "upper" {
				reg[ins.A] = strings.ToUpper(s)
			} else {
				reg[ins.A] = strings.ToLower(s)
			}
		case "append":
			dst, err := getString(reg[ins.A])
			if err != nil {
				return "", err
			}
			src, err := getString(reg[ins.B])
			if err != nil {
				return "", err
			}
			if len([]byte(dst))+len([]byte(src)) > maxSynthOutputBytes {
				return "", errors.New("synthesized sandbox output limit exceeded")
			}
			reg[ins.A] = dst + src
		case "cell_get":
			idx, err := getInt(reg[ins.B])
			if err != nil {
				return "", err
			}
			if idx < 0 || idx >= int64(len(memory)) {
				return "", errors.New("synthesized sandbox memory index out of range")
			}
			reg[ins.A] = memory[idx]
		case "cell_set":
			idx, err := getInt(reg[ins.A])
			if err != nil {
				return "", err
			}
			value, err := getInt(reg[ins.B])
			if err != nil {
				return "", err
			}
			if idx < 0 || idx >= int64(len(memory)) {
				return "", errors.New("synthesized sandbox memory index out of range")
			}
			memory[idx] = value
		case "jump":
			nextPC = ins.A
		case "jump_if_false":
			cond, err := getBool(reg[ins.A])
			if err != nil {
				return "", err
			}
			if !cond {
				nextPC = ins.B
			}
		case "emit":
			output, err := getString(reg[ins.A])
			if err != nil {
				return "", err
			}
			return output, nil
		case "halt":
			return "", errors.New("synthesized program halted without emit")
		}
		pc = nextPC
	}
	return "", errors.New("synthesized program terminated without emit")
}

func BuildSynthesizedProgram(instructions []SynthesizedInstruction, fuel uint64, maxInputBytes int) (SynthesizedProgram, error) {
	p := SynthesizedProgram{
		Version:       1,
		Language:      SynthesizedProgramLanguage,
		Source:        DisassembleSynthesizedProgram(instructions),
		Instructions:  append([]SynthesizedInstruction(nil), instructions...),
		Fuel:          fuel,
		MaxInputBytes: maxInputBytes,
	}
	if err := p.Validate(); err != nil {
		return SynthesizedProgram{}, err
	}
	return p, nil
}

func SynthesizeDomainEscape(ctx context.Context, task ReactorTask) (SynthesizedProgram, error) {
	program, _, err := SynthesizeDomainEscapeWithHeuristic(ctx, task, nil)
	return program, err
}

func SynthesizeDomainEscapeWithHeuristic(ctx context.Context, task ReactorTask, heuristic *SearchHeuristicProgram) (SynthesizedProgram, HeuristicSearchStats, error) {
	if task.InputKind != "string" {
		return SynthesizedProgram{}, HeuristicSearchStats{}, fmt.Errorf("no synthesized program grammar for input kind %q", task.InputKind)
	}
	if len(task.Examples) < 2 {
		return SynthesizedProgram{}, HeuristicSearchStats{}, errors.New("synthesized program search requires at least two training examples")
	}
	candidateNames := []string{"identity", "reverse", "upper", "lower", "upper-vowels", "lower-vowels", "reverse-upper-vowels"}
	orderedNames, err := orderStringCandidateNames(candidateNames, heuristic)
	if err != nil {
		return SynthesizedProgram{}, HeuristicSearchStats{}, fmt.Errorf("string search heuristic rejected frontier: %w", err)
	}
	builders := map[string]func() SynthesizedProgram{
		"identity":      buildIdentityProgram,
		"reverse":       buildReverseProgram,
		"upper":         buildUpperProgram,
		"lower":         buildLowerProgram,
		"upper-vowels":  buildUpperVowelsProgram,
		"lower-vowels":          buildLowerVowelsProgram,
		"reverse-upper-vowels": buildReverseThenUpperVowelsProgram,
	}
	stats := HeuristicSearchStats{}
	for _, name := range orderedNames {
		if err := ctx.Err(); err != nil {
			return SynthesizedProgram{}, stats, err
		}
		builder, ok := builders[name]
		if !ok {
			return SynthesizedProgram{}, stats, fmt.Errorf("unknown synthesized-program candidate %q", name)
		}
		candidate := builder()
		stats.CandidatesEvaluated++
		if err := candidate.Validate(); err != nil {
			continue
		}
		ok = true
		for _, example := range task.Examples {
			if err := ctx.Err(); err != nil {
				return SynthesizedProgram{}, stats, err
			}
			if len(example.Input) != 1 || len(example.Expected) != 1 {
				ok = false
				break
			}
			got, err := candidate.Execute(ctx, example.Input[0])
			if err != nil || got != example.Expected[0] {
				ok = false
				break
			}
		}
		if ok {
			return candidate, stats, nil
		}
	}
	return SynthesizedProgram{}, stats, errors.New("domain-escape synthesis exhausted bounded generic string program grammar")
}

func buildIdentityProgram() SynthesizedProgram {
	return mustBuildSynthProgram([]SynthesizedInstruction{
		{Op: "input", A: 0},
		{Op: "emit", A: 0},
		{Op: "halt"},
	})
}

func buildUpperProgram() SynthesizedProgram {
	return buildCharMapProgram("upper", "all")
}

func buildLowerProgram() SynthesizedProgram {
	return buildCharMapProgram("lower", "all")
}

func buildUpperVowelsProgram() SynthesizedProgram {
	return buildCharMapProgram("upper", "vowels")
}

func buildLowerVowelsProgram() SynthesizedProgram {
	return buildCharMapProgram("lower", "vowels")
}

func buildReverseThenUpperVowelsProgram() SynthesizedProgram {
	return buildReverseThenCharMapProgram("upper", "vowels")
}

func buildReverseThenCharMapProgram(transform, predicate string) SynthesizedProgram {
	if transform != "upper" && transform != "lower" {
		panic("unsupported synthesized char transform")
	}
	if predicate != "all" && predicate != "vowels" {
		panic("unsupported synthesized char predicate")
	}
	instructions := []SynthesizedInstruction{
		{Op: "input", A: 0},
		{Op: "newbuf", A: 1},
		{Op: "const", A: 2, B: 0},
		{Op: "len", A: 3, B: 0},
		{Op: "const", A: 4, B: 1},
		{Op: "lt", A: 5, B: 2, C: 3},
		{Op: "jump_if_false", A: 5, B: 0},
		{Op: "sub", A: 6, B: 3, C: 4},
		{Op: "sub", A: 6, B: 6, C: 2},
		{Op: "char", A: 7, B: 0, C: 6},
	}
	if predicate == "vowels" {
		instructions = append(instructions,
			SynthesizedInstruction{Op: "is_vowel", A: 8, B: 7},
			SynthesizedInstruction{Op: "jump_if_false", A: 8, B: 0},
		)
	}
	instructions = append(instructions,
		SynthesizedInstruction{Op: transform, A: 7, B: 7},
		SynthesizedInstruction{Op: "append", A: 1, B: 7},
		SynthesizedInstruction{Op: "add", A: 2, B: 2, C: 4},
		SynthesizedInstruction{Op: "jump", A: 5},
		SynthesizedInstruction{Op: "emit", A: 1},
		SynthesizedInstruction{Op: "halt"},
	)
	end := len(instructions) - 2
	instructions[6].B = end
	if predicate == "vowels" {
		appendIndex := -1
		for i, ins := range instructions {
			if ins.Op == "append" {
				appendIndex = i
				break
			}
		}
		if appendIndex < 0 {
			panic("synthesized reverse char-map program missing append")
		}
		for i, ins := range instructions {
			if ins.Op == "jump_if_false" && ins.A == 8 {
				instructions[i].B = appendIndex
			}
		}
	}
	return mustBuildSynthProgram(instructions)
}

func buildReverseProgram() SynthesizedProgram {
	instructions := []SynthesizedInstruction{
		{Op: "input", A: 0},
		{Op: "newbuf", A: 1},
		{Op: "const", A: 2, B: 0},
		{Op: "len", A: 3, B: 0},
		{Op: "const", A: 4, B: 1},
		{Op: "lt", A: 5, B: 2, C: 3},
		{Op: "jump_if_false", A: 5, B: 13},
		{Op: "sub", A: 6, B: 3, C: 4},
		{Op: "sub", A: 6, B: 6, C: 2},
		{Op: "char", A: 7, B: 0, C: 6},
		{Op: "append", A: 1, B: 7},
		{Op: "add", A: 2, B: 2, C: 4},
		{Op: "jump", A: 5},
		{Op: "emit", A: 1},
		{Op: "halt"},
	}
	return mustBuildSynthProgram(instructions)
}

func buildCharMapProgram(transform, predicate string) SynthesizedProgram {
	if transform != "upper" && transform != "lower" {
		panic("unsupported synthesized char transform")
	}
	if predicate != "all" && predicate != "vowels" {
		panic("unsupported synthesized char predicate")
	}
	instructions := []SynthesizedInstruction{
		{Op: "input", A: 0},
		{Op: "newbuf", A: 1},
		{Op: "const", A: 2, B: 0},
		{Op: "len", A: 3, B: 0},
		{Op: "const", A: 4, B: 1},
		{Op: "lt", A: 5, B: 2, C: 3},
		{Op: "jump_if_false", A: 5, B: 0},
		{Op: "char", A: 6, B: 0, C: 2},
	}
	if predicate == "vowels" {
		instructions = append(instructions,
			SynthesizedInstruction{Op: "is_vowel", A: 7, B: 6},
			SynthesizedInstruction{Op: "jump_if_false", A: 7, B: 0},
		)
	}
	instructions = append(instructions,
		SynthesizedInstruction{Op: transform, A: 6, B: 6},
		SynthesizedInstruction{Op: "append", A: 1, B: 6},
		SynthesizedInstruction{Op: "add", A: 2, B: 2, C: 4},
		SynthesizedInstruction{Op: "jump", A: 5},
		SynthesizedInstruction{Op: "emit", A: 1},
		SynthesizedInstruction{Op: "halt"},
	)
	end := len(instructions) - 2
	instructions[6].B = end
	if predicate == "vowels" {
		appendIndex := -1
		for i, ins := range instructions {
			if ins.Op == "append" {
				appendIndex = i
				break
			}
		}
		if appendIndex < 0 {
			panic("synthesized char-map program missing append")
		}
		for i, ins := range instructions {
			if ins.Op == "jump_if_false" && ins.A == 7 {
				instructions[i].B = appendIndex
			}
		}
	}
	return mustBuildSynthProgram(instructions)
}

func mustBuildSynthProgram(instructions []SynthesizedInstruction) SynthesizedProgram {
	p, err := BuildSynthesizedProgram(instructions, 50_000, maxSynthInputBytes)
	if err != nil {
		panic(err)
	}
	return p
}
