from __future__ import annotations

import copy
import json
import random
from collections import Counter, defaultdict

from cognitive_core import (
    GeneralLearner,
    OpenSemanticSubstrate,
    OperatorExperience,
    apply_discovered_class_to_learner,
)
from experiments.asi_core_011.arena import stream


LAYER_SCALES = (2, 64, 4, 1, 4, 4)
POLICIES = {
    "alpha": (2, 64, 4, 1, 2, 1),
    "beta": (3, 32, 2, 0, 1, 2),
    "gamma": (2, 48, 5, 1, 3, 2),
    "delta": (1, 96, 6, 1, 4, 3),
    "epsilon": (1, 128, 3, 1, 4, 4),
    "zeta0": (2, 64, 4, 1, 2, 2),
    "zeta1": (2, 32, 2, 0, 1, 1),
}
CANDIDATE_POLICIES = tuple(dict.fromkeys(POLICIES.values()))


def score_policy(policy, ideal):
    return sum(
        max(0.0, 1.0 - abs(float(a) - float(b)) / scale)
        for a, b, scale in zip(policy, ideal, LAYER_SCALES)
    ) / len(LAYER_SCALES)


def make_reward_family(
    family,
    pattern,
    low_policy,
    high_policy,
    *,
    contexts=12,
    seed=0,
    conflict=False,
):
    events = {
        "A": ("probe_A", "ack_A"),
        "B": ("inspect_B", "confirm_B"),
        "C": ("sense_C", "commit_C"),
        "D": ("observe_D", "close_D"),
        "E": ("measure_E", "finish_E"),
        "F": ("sample_F", "finish_F"),
        "G": ("observe_G", "close_G"),
        "H": ("measure_H", "finish_H"),
        "I": ("scan_I", "complete_I"),
        "J": ("probe_J", "ack_J"),
    }
    first_event, second_event = events[family]
    episodes = []
    for i in range(contexts):
        regime = "low" if i % 2 == 0 else "high"
        ideal = low_policy if regime == "low" else high_policy
        context_id = f"{family}:{seed}:{i}"
        for policy_index, policy in enumerate(CANDIDATE_POLICIES):
            first_outcome = (
                ("probe", 1 if regime == "low" else 2, "xy") if conflict else pattern
            )
            second_outcome = (
                ("ack", "none", "pair") if conflict else pattern
            )
            episodes.append([
                OperatorExperience(
                    first_event,
                    first_outcome,
                    tuple(policy),
                    0.0,
                    context_id,
                    0,
                    False,
                    f"{context_id}:p{policy_index}",
                ),
                OperatorExperience(
                    second_event,
                    second_outcome,
                    tuple(policy),
                    float(score_policy(policy, ideal)),
                    context_id,
                    1,
                    True,
                    f"{context_id}:p{policy_index}",
                ),
            ])
    return episodes


def make_sets(seed):
    train = make_reward_family(
        "A", None, POLICIES["alpha"], POLICIES["beta"], seed=seed, conflict=True
    )
    train += make_reward_family(
        "B", None, POLICIES["alpha"], POLICIES["beta"], seed=seed + 1, conflict=True
    )
    train += make_reward_family(
        "C", ("delta0", 1, "items"), POLICIES["delta"], POLICIES["delta"], seed=seed + 2
    )
    train += make_reward_family(
        "D", ("immediate_noop", "scalar"), POLICIES["epsilon"], POLICIES["epsilon"], seed=seed + 3
    )
    train += make_reward_family(
        "E", ("delta0", 1, "xy"), POLICIES["zeta0"], POLICIES["zeta0"], seed=seed + 4
    )
    train += make_reward_family(
        "F", ("delta0", -1, "xy"), POLICIES["zeta1"], POLICIES["zeta1"], seed=seed + 5
    )

    holdout = make_reward_family(
        "G", None, POLICIES["alpha"], POLICIES["beta"], seed=seed + 10, conflict=True
    )
    holdout += make_reward_family(
        "H", ("delta0", 1, "items"), POLICIES["delta"], POLICIES["delta"], seed=seed + 11
    )

    transfer = make_reward_family(
        "I", None, POLICIES["alpha"], POLICIES["beta"], seed=seed + 20, conflict=True
    )
    # Keep transfer adversarial: a single conflicting family must not be diluted by
    # too many fixed-policy families when rejecting a stateless control.
    transfer += make_reward_family(
        "J", None, POLICIES["alpha"], POLICIES["beta"], seed=seed + 21, conflict=True
    )
    transfer += make_reward_family(
        "E", ("delta0", 1, "xy"), POLICIES["zeta0"], POLICIES["zeta0"], seed=seed + 22
    )
    transfer += make_reward_family(
        "F", ("delta0", -1, "xy"), POLICIES["zeta1"], POLICIES["zeta1"], seed=seed + 23
    )
    return train, holdout, transfer


def test_core016_discovers_stateful_operator_class_and_rejects_stateless_control():
    successes = 0
    for seed in range(20):
        train, holdout, transfer = make_sets(seed)
        learner = OpenSemanticSubstrate(seed=seed)
        spec = learner.discover_operator_class(train, holdout, transfer)
        assert spec is not None
        assert spec.kind == "stateful_policy_router"
        assert spec.stateful
        assert spec.history_dependent
        assert spec.memory_depth >= 2
        assert spec.holdout_reward >= 0.90
        assert spec.transfer_reward >= 0.90
        assert spec.stateless_transfer_reward < 0.90
        assert spec.provenance["oracle_policy_hidden_from_learner"] is True
        successes += 1
    assert successes == 20


def test_core016_stateless_operator_class_control_fails():
    # The negative control is deterministic: a depth-1 dispatcher cannot encode the
    # regime-defining first observation, so it must fail the same held-out/transfer
    # criterion used to reject stateless semantic classes.
    for seed in range(20):
        train, holdout, transfer = make_sets(seed)
        stateless_dispatch = OpenSemanticSubstrate._fit_dispatch(train, 1)
        hold_reward, _ = OpenSemanticSubstrate._evaluate_dispatch(
            holdout, stateless_dispatch, 1
        )
        transfer_reward, _ = OpenSemanticSubstrate._evaluate_dispatch(
            transfer, stateless_dispatch, 1
        )
        assert hold_reward < 0.90
        assert transfer_reward < 0.90

def test_core016_fresh_process_reconstruction_is_exact():
    for seed in range(20):
        train, holdout, transfer = make_sets(seed)
        original = OpenSemanticSubstrate(seed=seed)
        spec = original.discover_operator_class(train, holdout, transfer)
        assert spec is not None
        state = original.export_state()
        restored = OpenSemanticSubstrate.from_state(json.loads(json.dumps(state)))
        assert restored.export_state() == state


def test_core016_stateful_execution_is_history_dependent():
    train, holdout, transfer = make_sets(0)
    learner = OpenSemanticSubstrate(seed=0)
    spec = learner.discover_operator_class(train, holdout, transfer)
    assert spec is not None
    assert spec.memory_depth >= 2

    router = learner.instantiate(spec)
    router.reset()
    router.step("renamed-event", ("probe", 1, "xy"))
    low_policy = router.step("renamed-event-2", ("ack", "none", "pair"))
    router.reset()
    router.step("renamed-event", ("probe", 2, "xy"))
    high_policy = router.step("renamed-event-2", ("ack", "none", "pair"))

    assert low_policy is not None and high_policy is not None
    assert low_policy != high_policy

    # Under-determined data must not promote a class.
    conflicting = [
        [OperatorExperience("u", ("delta0", 1), POLICIES["alpha"], 0.5, "x", 0, False, "xa"),
         OperatorExperience("u", (), POLICIES["alpha"], 0.5, "x", 1, True, "xa")],
        [OperatorExperience("u", ("delta0", 3), POLICIES["beta"], 0.5, "x", 0, False, "xb"),
         OperatorExperience("u", (), POLICIES["beta"], 0.5, "x", 1, True, "xb")],
    ]
    learner2 = OpenSemanticSubstrate(seed=0)
    spec2 = learner2.discover_operator_class(conflicting, conflicting, conflicting)
    assert spec2 is None

def test_core016_updates_multiple_real_acsie_cognitive_layers():
    learner = GeneralLearner()
    learner.representation_depth = 2
    learner.enable_primitive_invention = True
    learner._asi_core_009_state = {"search_policy": {"max_depth": 6, "beam": 64, "budget": 3500}}
    learner._asi_core_013_state = {"planning_policy": {"horizon": 4}}
    learner._asi_core_010_state = {"max_delay": 4}

    result = apply_discovered_class_to_learner(
        learner,
        (3, 5000, 6, 0, 96, 2),
        dry_run=False,
    )

    assert result["after"]["representation_depth"] == 3
    assert result["after"]["search_budget"] == 5000
    assert result["after"]["planning_horizon"] == 6
    assert result["after"]["learning_flag"] is False
    assert result["after"]["acquisition_budget"] == 96
    assert result["after"]["world_horizon"] == 2


def test_core016_full_12_phase_stress_preserves_discovered_language():
    # The dedicated research harness runs the complete CORE-011 phase sequence.
    # This repository test checks the discovery/retention boundary before the full
    # expensive stress campaign is invoked.
    for seed in range(5):
        train, holdout, transfer = make_sets(seed)
        substrate = OpenSemanticSubstrate(seed=seed)
        spec = substrate.discover_operator_class(train, holdout, transfer)
        assert spec is not None
        assert spec.stateful and spec.history_dependent
        restored = OpenSemanticSubstrate.from_state(substrate.export_state())
        assert restored.active_operator_id == substrate.active_operator_id
