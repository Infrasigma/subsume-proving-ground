package acex

import "testing"

func TestV8AdaptiveRepresentationEntityTransferCost(t *testing.T) {
	entity := NewV8CognitiveEntity()
	source, sourceActions, sourceCorrect := adaptiveRoleState("entity-src", 0)
	entity.ObserveOutcome(source, sourceActions[0], source, -1, false)
	entity.ObserveOutcome(source, sourceCorrect, source, 1, false)
	if entity.AdaptiveRoles.ActiveRadius < 2 {
		t.Fatalf("entity did not retain expanded representation: radius=%d", entity.AdaptiveRoles.ActiveRadius)
	}
	sourceKey, sourceKeyOK := rootedNeighborhoodKey(source, sourceCorrect, entity.AdaptiveRoles.ActiveRadius)
	if !sourceKeyOK || entity.AdaptiveRoles.Success[sourceKey] == 0 {
		t.Fatalf("source success key missing: ok=%v key=%q successes=%v", sourceKeyOK, sourceKey, entity.AdaptiveRoles.Success)
	}
	targetProbe, _, targetCorrectProbe := adaptiveRoleState("entity-probe", 1)
	targetKey, targetKeyOK := rootedNeighborhoodKey(targetProbe, targetCorrectProbe, entity.AdaptiveRoles.ActiveRadius)
	if !targetKeyOK || targetKey != sourceKey {
		t.Fatalf("representation did not transfer: source=%q target=%q", sourceKey, targetKey)
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
			t.Logf("stage=%d selected=%q correct=%q adaptiveRadius=%d", stage, a, correct, entity.AdaptiveRoles.ActiveRadius)
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
