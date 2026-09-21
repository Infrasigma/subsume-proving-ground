package acex

import "testing"

func TestV8AdaptiveRepresentationEntityTransferCost(t *testing.T) {
	entity := NewV8CognitiveEntity()
	source, sourceActions, sourceCorrect := adaptiveRoleState("entity-src", 0)
	for i := 0; i < len(sourceActions)*2; i++ {
		a, err := entity.ObserveAndAct(source, sourceActions)
		if err != nil {
			t.Fatal(err)
		}
		reward := -1.0
		if a == sourceCorrect {
			reward = 1
		}
		entity.ObserveOutcome(source, a, source, reward, false)
		if a == sourceCorrect {
			break
		}
	}
	if entity.AdaptiveRoles.ActiveRadius < 2 {
		t.Fatalf("entity did not retain expanded representation: radius=%d", entity.AdaptiveRoles.ActiveRadius)
	}

	cost := 0
	for stage := 0; stage < 5; stage++ {
		state, actions, correct := adaptiveRoleState("entity-dst-"+string(rune('a'+stage)), 1)
		for i := 0; i < len(actions)*2; i++ {
			a, err := entity.ObserveAndAct(state, actions)
			if err != nil {
				t.Fatal(err)
			}
			cost++
			reward := -1.0
			if a == correct {
				reward = 1
			}
			entity.ObserveOutcome(state, a, state, reward, stage == 4 && a == correct)
			if a == correct {
				break
			}
		}
	}
	t.Logf("adaptive entity target cost=%d radius=%d representation=%s", cost, entity.AdaptiveRoles.ActiveRadius, entity.AdaptiveRoles.InventedRepresentation())
	if cost > 5 {
		t.Fatalf("adaptive representation failed to drive direct entity transfer: cost=%d", cost)
	}
}
