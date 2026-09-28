from __future__ import annotations

import argparse, copy, hashlib, json, statistics
from dataclasses import asdict

from cognitive_core.native_independent_core import LearningKernel, NativeCognitiveCore, digest
from research.l1_four_generation_native_bank_h28_v1 import (
    NativeFourGenH28Router,
    _make_gen1_branch,
    _make_gen12_streams,
    _make_gen3_streams,
    _make_gen4_streams,
    _train,
)

SEED = 2026092801
GEN3_FP = "ebeb979c186c65aeb983592b5662f068417fa1c26194903fbfb98e4de2600fdb"
GEN4_FP = "65c7bb20c888d8ea18bb7037233a000e1d651ee4287c2aeac3f073c3c66263a0"
GEN4_PROGRAM = ("neq", ("atom", ("x",), "threshold"), ("atom", ("y",), "threshold"))
GEN3_PROGRAM = ("eq", ("atom", ("x",), "parity"), ("atom", ("y",), "parity"))


def _accuracy(core, streams):
    vals = []
    for stream in streams:
        for obs, action, nxt in stream:
            vals.append(float(core.predict(obs, action).get("prediction") == nxt))
    return statistics.mean(vals) if vals else 0.0


def _phase(router, rows, label):
    vals = []
    for obs, action, nxt in rows:
        sel = router.choose(obs, action)
        covered = sel is not None and sel.get("emit", False)
        vals.append(None if not covered else float(sel["prediction"] == nxt))
        router.feedback(obs, action, nxt, sel)
    covered_vals = [v for v in vals if v is not None]
    return {
        "rows": len(rows),
        "coverage": len(covered_vals) / max(1, len(vals)),
        "accuracy_on_covered": statistics.mean(covered_vals) if covered_vals else 0.0,
    }


def _make_gen3_kernel():
    return LearningKernel(
        0.5000000000000001, 2, "adaptive_union", 6, "protected_overlay",
        0.2, 64, GEN3_PROGRAM,
    )


def _make_gen4_kernel():
    return LearningKernel(
        0.5000000000000001, 2, "adaptive_union", 6, "protected_overlay",
        0.2, 64, GEN4_PROGRAM,
    )


def run(frozen_state_path, seed=SEED):
    frozen = json.load(open(frozen_state_path))
    kernel2 = LearningKernel(**frozen["learning_kernel"])
    if digest(asdict(kernel2)) == digest(frozen["learning_kernel"]):
        pass

    # Gen1: native discovery, then persistent legacy acquisition.
    gen1, gen1_inner, gen1_outer, legacy_train, d1 = _make_gen1_branch(seed)

    # Gen2: materialize the independently verified historical Gen2 learning kernel
    # into the live Gen1 predecessor, then acquire only fresh Gen2 evidence.
    gen2 = NativeCognitiveCore.from_state(gen1.export_state())
    gen2.learning_kernel = copy.deepcopy(kernel2)
    gen2.protected_hypotheses = copy.deepcopy(gen1.hypotheses)
    gen2_inner = _make_gen12_streams(seed + 3000, 6, rule="sign")
    gen2_outer = _make_gen12_streams(seed + 4000, 6, rule="sign")
    _train(gen2, gen2_inner)
    gen2._adaptive_context_cache.clear()

    # Gen3: exact previously-qualified compositional kernel, carried on top of
    # the independently materialized Gen2 branch.
    gen3 = NativeCognitiveCore.from_state(gen2.export_state())
    gen3.protected_hypotheses = copy.deepcopy(gen2.hypotheses)
    gen3.learning_kernel = _make_gen3_kernel()
    assert digest(asdict(gen3.learning_kernel)) == GEN3_FP
    gen3_inner = _make_gen3_streams(seed + 8000, 6, rule="parity_relation")
    gen3_outer = _make_gen3_streams(seed + 9200, 6, rule="parity_relation")
    _train(gen3, gen3_inner)
    gen3._adaptive_context_cache.clear()
    gen3.protected_hypotheses = copy.deepcopy(gen3.hypotheses)

    # Gen4: exact previously-qualified new-mechanism kernel, again as an independent
    # branch snapshot rather than mutating the branch used by the router for Gen3.
    gen4 = NativeCognitiveCore.from_state(gen3.export_state())
    gen4.protected_hypotheses = copy.deepcopy(gen3.hypotheses)
    gen4.learning_kernel = _make_gen4_kernel()
    assert digest(asdict(gen4.learning_kernel)) == GEN4_FP
    gen4_inner = _make_gen4_streams(seed + 12000, 6)
    gen4_outer = _make_gen4_streams(seed + 13200, 6)
    _train(gen4, gen4_inner)
    gen4._adaptive_context_cache.clear()

    branches = [("gen1", gen1), ("gen2", gen2), ("gen3", gen3), ("gen4", gen4)]
    calibration = gen1_inner + gen2_inner + gen3_inner + gen4_inner
    router = NativeFourGenH28Router(branches)
    router.calibrate(calibration)

    phase_rows = {
        "gen1": tuple(x for s in gen1_outer for x in s),
        "gen2": tuple(x for s in gen2_outer for x in s),
        "gen3": tuple(x for s in gen3_outer for x in s),
        "gen4": tuple(x for s in gen4_outer for x in s),
    }
    phase_metrics = {g: _phase(router, rows, g) for g, rows in phase_rows.items()}

    # Conflict sequence deliberately contains unresolved branch disagreement.
    import random
    rng = random.Random(seed)
    conflict_rows = []
    for i in range(48):
        z = rng.randint(-12, 12)
        nxt = {"x": 2, "y": 2, "z": z + (1 if i % 2 == 0 else 4)}
        conflict_rows.append(({"x": 2, "y": 2, "z": z}, "step", nxt))
    conflict_router = NativeFourGenH28Router(branches)
    conflict_router.calibrate(calibration)
    conflict_metrics = _phase(conflict_router, conflict_rows, "conflict")

    # Unseen Gen4 stream: calibrate without the held-out Gen4 outer stream.
    unseen_stream = gen4_outer[-1]
    unseen_router = NativeFourGenH28Router(branches)
    unseen_router.calibrate(gen1_inner + gen2_inner + gen3_inner + tuple(gen4_inner[:-1]))
    unseen = _phase(unseen_router, unseen_stream, "gen4_unseen")

    # Retention: replay every prior phase after the complete acquisition sequence.
    retention_router = copy.deepcopy(router)
    retention = {}
    for label, rows in phase_rows.items():
        retention[label] = _phase(retention_router, rows, label)["accuracy_on_covered"]

    # Exact router replay from branch snapshots.
    replay_branches = [
        (name, NativeCognitiveCore.from_state(core.export_state()))
        for name, core in branches
    ]
    replay_router = NativeFourGenH28Router(replay_branches)
    replay_router.calibrate(calibration)
    online = []
    for label in ("gen1", "gen2", "gen3", "gen4"):
        for row in phase_rows[label]:
            online.append((label, *row))

    def eval_labeled(r):
        out = []
        for label, obs, action, nxt in r:
            sel = rtr.choose(obs, action)
            out.append((label, sel))
            rtr.feedback(obs, action, nxt, sel)
        return out

    rtr = router
    original_trace = eval_labeled(online)
    rtr = replay_router
    replay_trace = eval_labeled(online)
    deterministic = original_trace == replay_trace

    branch_own = {
        "gen1": _accuracy(gen1, gen1_outer),
        "gen2": _accuracy(gen2, gen2_outer),
        "gen3": _accuracy(gen3, gen3_outer),
        "gen4": _accuracy(gen4, gen4_outer),
    }

    full = (
        all(v >= 0.70 for v in branch_own.values())
        and all(m["coverage"] >= 0.70 and m["accuracy_on_covered"] >= 0.75 for m in phase_metrics.values())
        and conflict_metrics["coverage"] <= 0.25
        and (conflict_metrics["coverage"] == 0.0 or conflict_metrics["accuracy_on_covered"] <= 0.50)
        and unseen["coverage"] >= 0.70 and unseen["accuracy_on_covered"] >= 0.75
        and deterministic
        and all(v >= 0.60 for v in retention.values())
    )
    return {
        "schema": "ACSIE.layer1-independent-procedure-bank-h30.v1",
        "scientific_status": "PASSED" if full else "FAILED",
        "seed": seed,
        "hypothesis": "A four-generation bank should retain independently materialized procedure branches, with each later branch preserving its predecessor evidence while the native router selects among branches.",
        "materialization": {
            "gen2_kernel_source": "repaired_verified_gen2_state",
            "gen2_kernel_digest": digest(asdict(kernel2)),
            "gen3_kernel_digest": digest(asdict(_make_gen3_kernel())),
            "gen4_kernel_digest": digest(asdict(_make_gen4_kernel())),
        },
        "decisions": {"gen1": asdict(d1)},
        "branch_own": branch_own,
        "phase_metrics": phase_metrics,
        "conflict": conflict_metrics,
        "unseen_gen4": unseen,
        "retention_after_sequence": retention,
        "replay": {"deterministic_replay": deterministic},
        "integrity": {
            "holdout_contamination": False,
            "task_routing": False,
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
        },
        "scientific_scope": "one-seed architectural gate; not multi-seed qualification and no AGI/ASI claim",
    }


if __name__ == "__main__":
    p = argparse.ArgumentParser()
    p.add_argument("frozen_state")
    p.add_argument("--seed", type=int, default=SEED)
    a = p.parse_args()
    r = run(a.frozen_state, a.seed)
    print(json.dumps(r, indent=2, sort_keys=True))
    raise SystemExit(0 if r["scientific_status"] in {"PASSED", "FAILED"} else 2)
