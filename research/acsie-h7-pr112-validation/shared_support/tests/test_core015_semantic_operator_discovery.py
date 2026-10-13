import copy
import random

from cognitive_core import (
    GeneralLearner,
    SemanticOperatorDiscovery,
)


def make_rows(seed, family, n=20):
    rng = random.Random(seed * 97 + sum(map(ord, family)))
    rows = []
    telemetry_key = "uncertainty" if family.startswith("A") else "prediction_error"
    for i in range(n):
        pressure = (0.0, 0.5, 1.0, 2.0)[i % 4]
        rep = rng.choice((2, 3, 4))
        budget = rng.choice((2000, 2500, 3000))
        beam = rng.choice((32, 48, 64))
        rows.append({
            "before": {
                "representation_depth": rep,
                "search_budget": budget,
                "search_beam": beam,
            },
            "after": {
                "representation_depth": rep + (1 if pressure >= 0.5 else 0),
                "search_budget": budget + (1000 if pressure >= 0.5 else 0),
                "search_beam": beam + (16 if pressure >= 0.5 else 0),
            },
            "telemetry": {telemetry_key: pressure},
            "family": family,
            "task_id": f"{family}-{i}",
        })
    return rows


def fixed_single_effect_failure(rows):
    changed = []
    for field in rows[0]["before"]:
        if any(r["before"][field] != r["after"][field] for r in rows):
            changed.append(field)
    # The legacy operator interface can synthesize one scalar output, not a bundled
    # multi-effect cognitive operation. The target requires >=2 coordinated effects.
    return len(changed) >= 2


def run_seed(seed):
    A = make_rows(seed, "A")
    B = make_rows(seed, "B")
    C = make_rows(seed, "A2")
    D = make_rows(seed, "B2")

    train = A[:12] + B[:12]
    holdout = A[12:16] + B[12:16]
    transfer = C[16:] + D[16:]

    assert fixed_single_effect_failure(train)

    learner = SemanticOperatorDiscovery(seed=seed)
    op = learner.discover(train, holdout, transfer)
    assert op is not None
    assert op.status == "validated"
    assert op.kind == "multi_effect_typed_control"
    assert len(op.effects) >= 2

    token = learner.language.token(op.operator_id)
    assert token == f"sop:{op.operator_id}"

    # The raw telemetry vocabulary differs on transfer; the installed operator binds
    # to a typed role rather than a hand-specified field name.
    executed = learner.language.execute(token, transfer[0]["before"], transfer[0]["telemetry"])
    assert executed == transfer[0]["after"]

    # Fresh-process reconstruction of the executable semantic language.
    state = learner.export_state()
    restored = SemanticOperatorDiscovery.from_state(state)
    raw = restored.language.operators[op.operator_id].to_dict()
    assert raw["kind"] == op.kind
    assert raw["status"] == "validated"
    assert tuple(raw.get("parent_operator_ids", ())) == ()

    # Recursive generation: the enlarged language is used as the state basis for
    # another independently discovered semantic operator.
    g1 = []
    for row in C + D:
        base = copy.deepcopy(row["after"])
        pressure = float(next(iter(row["telemetry"].values())))
        g1.append({
            "before": base,
            "after": {
                **base,
                "representation_depth": base["representation_depth"] + (1 if pressure >= 0.5 else 0),
                "search_budget": base["search_budget"] + (500 if pressure >= 0.5 else 0),
                "search_beam": base["search_beam"] + (8 if pressure >= 0.5 else 0),
            },
            "telemetry": dict(row["telemetry"]),
            "family": "recursive",
            "task_id": "R-" + row["task_id"],
        })

    learner.generation = 1
    op2 = learner.discover(g1[:10], g1[10:15], g1[15:])
    assert op2 is not None
    assert op2.generation == 1
    assert op2.parent_operator_ids == (op.operator_id,)
    assert bool(op2.provenance.get("composes_prior_operator"))
    assert op2.operator_id in learner.language.operators
    assert len(op2.effects) >= 2

    token2 = learner.language.token(op2.operator_id)
    # The new token must execute the old operator as a parent, not merely learn from
    # its output during discovery.
    assert learner.language.execute(
        token2, g1[0]["before"], g1[0]["telemetry"]
    ) == g1[0]["after"]

    rebuilt = SemanticOperatorDiscovery.from_state(learner.export_state())
    assert rebuilt.language.execute(
        token2, g1[0]["before"], g1[0]["telemetry"]
    ) == g1[0]["after"]

    # Actual ACSIE representation/search execution boundary.
    core = GeneralLearner()
    core._asi_core_009_state = {
        "search_policy": {"max_depth": 6, "beam": 64, "budget": 3500}
    }
    core.semantic_operator_id = op.operator_id
    core._asi_core_013_state = {
        "semantic_operators": {op.operator_id: op.to_dict()},
        "semantic_language": {"operators": {op.operator_id: op.to_dict()}},
        "generator": {"recursion_depth": 0, "learned": True},
        "events": [],
        "latest_telemetry": {},
    }

    # Boundary check: the learned thresholded operator is allowed to be a no-op
    # when the observed pressure is below its learned activation threshold.
    low_result = core.execute_semantic_operator(
        op.operator_id,
        telemetry=transfer[0]["telemetry"],
        dry_run=False,
    )
    assert low_result["after"]["representation_depth"] == low_result["before"]["representation_depth"]
    assert low_result["after"]["search_budget"] == low_result["before"]["search_budget"]
    assert low_result["after"]["search_beam"] == low_result["before"]["search_beam"]
    assert low_result["installed"] is True
    assert low_result["language_token"] == f"sop:{op.operator_id}"

    # Positive-pressure execution must cross the learned threshold and change all
    # coordinated runtime controls without consulting a hidden evaluator.
    positive_row = next(
        row for row in transfer
        if float(next(iter(row["telemetry"].values()))) >= 0.5
    )
    result = core.execute_semantic_operator(
        op.operator_id,
        telemetry=positive_row["telemetry"],
        dry_run=False,
    )
    assert result["after"]["representation_depth"] != result["before"]["representation_depth"]
    assert result["after"]["search_budget"] != result["before"]["search_budget"]
    assert result["after"]["search_beam"] != result["before"]["search_beam"]
    assert result["installed"] is True
    assert result["language_token"] == f"sop:{op.operator_id}"


def test_core015_principal_multi_seed():
    for seed in range(20):
        run_seed(seed)
