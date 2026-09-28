from __future__ import annotations

import copy
import json
import statistics
from dataclasses import asdict

from cognitive_core.native_independent_core import LearningKernel, NativeCognitiveCore, digest
from research.export_verified_gen2_state_from_b311_v1 import _reconstruct_verified_state, _make_streams, _train


SEED = 771221


def score(core, streams):
    vals = []
    for stream in streams:
        for obs, action, nxt in stream:
            vals.append(float(core.predict(obs, action).get("prediction") == nxt))
    return statistics.mean(vals) if vals else 0.0


def build_parent(seed):
    gen1_inner = _make_streams(seed + 1000, 6, rule="parity")
    legacy_train = _make_streams(seed + 5000, 4, rule="parity")
    gen2_inner = _make_streams(seed + 3000, 6, rule="sign")
    gen2_outer = _make_streams(seed + 4000, 6, rule="sign")
    legacy_holdout = _make_streams(seed + 6000, 3, rule="parity")

    parent = NativeCognitiveCore(seed=seed)
    d1 = parent.self_improve_learning(gen1_inner)
    _train(parent, legacy_train)
    parent_state = parent.export_state()
    parent_hypotheses = copy.deepcopy(parent.hypotheses)
    d2 = parent.self_improve_learning(gen2_inner)
    _train(parent, gen2_inner)
    frozen = parent.export_state()
    return {
        "parent_state": parent_state,
        "parent_hypotheses": parent_hypotheses,
        "gen2_inner": gen2_inner,
        "gen2_outer": gen2_outer,
        "legacy_holdout": legacy_holdout,
        "d1": asdict(d1),
        "d2": asdict(d2),
        "kernel2": asdict(parent.learning_kernel),
        "frozen_state": frozen,
    }


def clone(state):
    return NativeCognitiveCore.from_state(copy.deepcopy(state))


def run(seed=SEED):
    p = build_parent(seed)
    outer = p["gen2_outer"]
    legacy = p["legacy_holdout"]

    frozen = clone(p["frozen_state"])
    baseline = score(frozen, outer)
    legacy_baseline = score(frozen, legacy)

    variants = {}

    # A: exact frozen state, no modifications.
    a = clone(p["frozen_state"])
    variants["A_frozen_exact"] = {
        "gen2_outer": score(a, outer),
        "legacy_holdout": score(a, legacy),
        "kernel": asdict(a.learning_kernel),
    }

    # B: frozen state, replay the same Gen2 evidence once more.
    b = clone(p["frozen_state"])
    _train(b, p["gen2_inner"])
    variants["B_reobserve_gen2"] = {
        "gen2_outer": score(b, outer),
        "legacy_holdout": score(b, legacy),
        "kernel": asdict(b.learning_kernel),
    }

    # C: reconstruct from pre-Gen2 parent, install the historical frozen kernel,
    # keep legacy hypotheses, then train Gen2 once.
    c = clone(p["parent_state"])
    c.learning_kernel = LearningKernel(**p["kernel2"])
    _train(c, p["gen2_inner"])
    variants["C_parent_plus_historical_kernel"] = {
        "gen2_outer": score(c, outer),
        "legacy_holdout": score(c, legacy),
        "kernel": asdict(c.learning_kernel),
    }

    # D: same as C but protect all pre-Gen2 hypotheses and use a generic
    # protected overlay with the historical kernel parameters otherwise unchanged.
    d = clone(p["parent_state"])
    d.learning_kernel = LearningKernel(**p["kernel2"]) 
    protected = copy.deepcopy(d.hypotheses)
    _train(d, p["gen2_inner"])
    d.protected_hypotheses = protected
    variants["D_posttrain_protected_predecessor"] = {
        "gen2_outer": score(d, outer),
        "legacy_holdout": score(d, legacy),
        "kernel": asdict(d.learning_kernel),
    }

    # E: same as C, but isolate the new mechanism from old mutable hypotheses.
    e = clone(p["parent_state"])
    e.learning_kernel = LearningKernel(**p["kernel2"])
    e.hypotheses = {}
    e.protected_hypotheses = copy.deepcopy(p["parent_hypotheses"])
    e._adaptive_context_cache.clear()
    _train(e, p["gen2_inner"])
    variants["E_clear_current_keep_protected"] = {
        "gen2_outer": score(e, outer),
        "legacy_holdout": score(e, legacy),
        "kernel": asdict(e.learning_kernel),
        "protected_count": len(e.protected_hypotheses),
    }

    return {
        "schema": "ACSIE.gen2-materialization-h29b.v1",
        "seed": seed,
        "scientific_status": "DIAGNOSTIC",
        "historical_decisions": {"gen1": p["d1"], "gen2": p["d2"]},
        "historical_kernel2": p["kernel2"],
        "baseline_frozen": {
            "gen2_outer": baseline,
            "legacy_holdout": legacy_baseline,
        },
        "variants": variants,
        "scope": "mechanism diagnosis only; no qualification",
        "integrity": {
            "task_routing": False,
            "external_model": False,
            "network_dependency": False,
            "holdout_contamination": False,
        },
    }


if __name__ == "__main__":
    r = run()
    print(json.dumps(r, indent=2, sort_keys=True))
