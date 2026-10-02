from __future__ import annotations
import copy, json, importlib.util, pathlib, sys

spec = importlib.util.spec_from_file_location(
    "h30",
    pathlib.Path(__file__).with_name("l1_independent_procedure_bank_h30_v1.py"),
)
h30 = importlib.util.module_from_spec(spec)
sys.modules["h30"] = h30
spec.loader.exec_module(h30)

Procedure = h30.Procedure
ProcedureBank = h30.ProcedureBank
_make_gen3_kernel = h30._make_gen3_kernel
_make_gen4_kernel = h30._make_gen4_kernel
_make_gen1_branch = h30._make_gen1_branch
_make_gen12_streams = h30._make_gen12_streams
_make_gen3_streams = h30._make_gen3_streams
_make_gen4_streams = h30._make_gen4_streams

from cognitive_core.native_independent_core import LearningKernel

SEED = 2026092801


def build(seed):
    g1_core, g1_inner, g1_outer, _, _ = _make_gen1_branch(seed)
    g2i = _make_gen12_streams(seed + 3000, 6, rule="sign")
    g2o = _make_gen12_streams(seed + 4000, 6, rule="sign")
    g3i = _make_gen3_streams(seed + 8000, 6, rule="parity_relation")
    g3o = _make_gen3_streams(seed + 9200, 6, rule="parity_relation")
    g4i = _make_gen4_streams(seed + 12000, 6)
    g4o = _make_gen4_streams(seed + 13200, 6)
    procs = (
        Procedure("p0", 0, copy.deepcopy(g1_core.learning_kernel)),
        Procedure("p1", 1, LearningKernel(0.5000000000000001, 2, "adaptive_union", 6, "ensemble", 0.2, 64)),
        Procedure("p2", 2, _make_gen3_kernel()),
        Procedure("p3", 3, _make_gen4_kernel()),
    )
    return ProcedureBank(procs), (g1_inner, g2i, g3i, g4i), (g1_outer, g2o, g3o, g4o)


def rank(bank, items):
    return bank._rank(items)


def main():
    bank, inners, outers = build(SEED)
    inner_models = {
        p.name: [bank._train_episode(p, stream, i) for i, stream in enumerate(inners[p.index])]
        for p in bank.procedures
    }
    bank.calibrate(inner_models)

    dump = {}
    for gen_index, (label, streams) in enumerate(zip(("gen1", "gen2", "gen3", "gen4"), outers)):
        episodes = []
        for episode_index, stream in enumerate(streams):
            models = {
                p.name: bank._train_episode(p, stream, 1000 + gen_index * 100 + episode_index * 10 + p.index)
                for p in bank.procedures
            }
            rows = []
            candidate_cache = {
                p.name: bank.candidates(models[p.name]) for p in bank.procedures
            }
            for row_index in range(len(models["p0"].holdout)):
                items = [candidate_cache[p.name][row_index] for p in bank.procedures]
                ranked = rank(bank, items)
                picked = bank.choose(items, feedback=False)
                rows.append({
                    "row": row_index,
                    "target": items[0]["target"],
                    "ranked": ranked,
                    "picked": picked,
                    "correct": bool(picked is not None and picked["prediction"] == items[0]["target"]),
                })
            episodes.append({
                "episode": episode_index,
                "rows": rows,
                "coverage": sum(1 for x in rows if x["picked"] is not None) / len(rows),
                "accuracy_on_covered": (
                    sum(1 for x in rows if x["picked"] is not None and x["correct"])
                    / max(1, sum(1 for x in rows if x["picked"] is not None))
                ),
            })
        dump[label] = episodes

    print(json.dumps({
        "schema": "ACSIE.h38-same-episode-router-row-diagnostic.v1",
        "seed": SEED,
        "phase_dump": dump,
    }, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
