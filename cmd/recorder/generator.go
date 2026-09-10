package main

import (
	"fmt"
	"math/rand"

	"github.com/Infrasigma/subsume-proving-ground/evaluator"
	"github.com/Infrasigma/subsume-proving-ground/protocol"
)

const (
	modeIsolated       = "isolated_rules"
	modeCompositional  = "compositional_ood"
	actionMove uint32 = 1
)

type entityKind uint8

const (
	entityAgent entityKind = iota
	entityBlock
	entitySwitch
	entitySinkhole
	tileSize = 6
)

type entity struct {
	kind entityKind
	x    int
	y    int
}

type arena struct {
	entities []entity
}

func buildArena(rng *rand.Rand, mode string, variant int) arena {
	baseX := 12 + rng.Intn(8)
	baseY := 28 + rng.Intn(8)

	if mode == modeIsolated {
		return arena{entities: []entity{
			{kind: entityAgent, x: baseX, y: baseY},
			{kind: entityBlock, x: baseX + 18, y: baseY},
		}}
	}

	// Both compositional chains are constructed so the forced one-tick MOVE
	// places the block directly on its terminal target.
	target := entitySwitch
	if variant == 1 {
		target = entitySinkhole
	}
	return arena{entities: []entity{
		{kind: entityAgent, x: baseX, y: baseY},
		{kind: entityBlock, x: baseX + 6, y: baseY},
		{kind: target, x: baseX + 12, y: baseY},
	}}
}

func (a arena) step() (arena, evaluator.TransitionEvidence) {
	next := arena{entities: make([]entity, len(a.entities))}
	copy(next.entities, a.entities)

	ev := evaluator.TransitionEvidence{}
	agent := -1
	block := -1
	target := -1
	for i := range next.entities {
		switch next.entities[i].kind {
		case entityAgent:
			agent = i
		case entityBlock:
			block = i
		case entitySwitch, entitySinkhole:
			target = i
		}
	}

	if agent < 0 {
		return next, ev
	}

	// Forced MOVE advances the agent and pushes the block when the chain mode
	// has one. The movement fact is accumulated independently of contact facts.
	next.entities[agent].x += 6
	ev.AgentMoved = true

	if block >= 0 && next.entities[block].x == next.entities[agent].x {
		next.entities[block].x += 6
		ev.ElasticContact = true

		if target >= 0 && next.entities[block].x == next.entities[target].x {
			switch next.entities[target].kind {
			case entitySwitch:
				ev.PropertyMutation = true
			case entitySinkhole:
				ev.CreationDeletion = true
				// The sinkhole consumes the block.
				next.entities[block].kind = entityKind(255)
			}
		}
	}

	return next, ev
}

func buildAction(ar arena) protocol.ActionRecord {
	var action protocol.ActionRecord
	action.Type = actionMove
	for _, e := range ar.entities {
		if e.kind == entityAgent {
			action.Params[0] = int32(e.x * 1024 / 64)
			action.Params[1] = int32(e.y * 1024 / 64)
			break
		}
	}
	// MOVE vector in Q10 units; magnitude/duration are fixed for the smoke
	// generator so every example has identical causal timing.
	action.Params[2] = 96
	action.Params[3] = 0
	action.Params[4] = 6 * 1024 / 64
	action.Params[5] = 1
	return action
}

func render(ar arena) [protocol.FrameBytes]byte {
	var frame [protocol.FrameBytes]byte
	put := func(x, y, size int, r, g, b byte) {
		for yy := y; yy < y+size; yy++ {
			if yy < 0 || yy >= 64 {
				continue
			}
			for xx := x; xx < x+size; xx++ {
				if xx < 0 || xx >= 64 {
					continue
				}
				i := (yy*64 + xx) * 3
				frame[i], frame[i+1], frame[i+2] = r, g, b
			}
		}
	}

	for _, e := range ar.entities {
		switch e.kind {
		case entityAgent:
			put(e.x, e.y, tileSize, 32, 96, 255)
		case entityBlock:
			put(e.x, e.y, tileSize, 160, 160, 160)
		case entitySwitch:
			put(e.x, e.y, tileSize, 64, 220, 96)
		case entitySinkhole:
			put(e.x, e.y, tileSize, 24, 24, 24)
		}
	}
	return frame
}

func generateRecord(seed int64, index int, mode string) (protocol.TransitionRecordV8, uint8, error) {
	var record protocol.TransitionRecordV8
	rng := rand.New(rand.NewSource(seed + int64(index)*1000003))
	variant := index & 1
	before := buildArena(rng, mode, variant)
	after, evidence := before.step()
	mask := evidence.ProjectMultiHot()

	if mode == modeIsolated && mask != uint8(1)<<evaluator.ClassAgentMove {
		return record, 0, fmt.Errorf("isolated record %d produced mask 0x%02x", index, mask)
	}
	if mode == modeCompositional {
		expected := uint8(1)<<evaluator.ClassAgentMove | uint8(1)<<evaluator.ClassContactElastic
		if variant == 0 {
			expected |= uint8(1) << evaluator.ClassPropertyMutation
		} else {
			expected |= uint8(1) << evaluator.ClassCreationDeletion
		}
		if mask != expected {
			return record, 0, fmt.Errorf("compositional record %d variant %d produced mask 0x%02x want 0x%02x", index, variant, mask, expected)
		}
	}

	record.TickBefore = uint64(index)
	record.TickAfter = uint64(index + 1)
	record.Action = buildAction(before)
	record.ConsequenceMask = mask
	record.FrameBefore = render(before)
	record.FrameAfter = render(after)
	return record, mask, nil
}

func Generate(path string, count int, mode string, seed int64) error {
	if count <= 0 {
		return fmt.Errorf("count must be positive")
	}
	if mode != modeIsolated && mode != modeCompositional {
		return fmt.Errorf("unsupported mode %q", mode)
	}

	f, err := openOutput(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var classCounts [8]uint32
	records := make([]protocol.TransitionRecordV8, count)
	for i := 0; i < count; i++ {
		r, mask, err := generateRecord(seed, i, mode)
		if err != nil {
			return err
		}
		records[i] = r
		for bit := 0; bit < 8; bit++ {
			if mask&(uint8(1)<<bit) != 0 {
				classCounts[bit]++
			}
		}
	}

	header := protocol.NewDatasetHeaderV8(uint64(seed), uint32(count), classCounts)
	if err := protocol.WriteHeaderV8(f, &header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for i := range records {
		if err := protocol.WriteRecordV8(f, &records[i]); err != nil {
			return fmt.Errorf("write record %d: %w", i, err)
		}
	}
	return nil
}
