import random

from cognitive_core import RecursiveCognitiveCompiler, Trace


def make_family(rng, name, n, mode):
    rows = []
    for i in range(n):
        a = float(rng.randint(1, 6))
        b = float(rng.randint(1, 5))
        c = float(rng.randint(0, 3))
        if mode == "linear":
            y = 2 * b + c
        elif mode == "nested":
            y = 1 + 3 * (2 * b + c)
        else:
            y = 2 * b + c if a >= 4 else 0.0
        rows.append(Trace(
            {"slot_a": a, "slot_b": b, "slot_c": c},
            y, name,
            {"transfer_risk": 0.0, "ambiguity": 0.0},
            f"{name}-{i}",
        ))
    return rows


def make_conflict(rng, n):
    rows = []
    for i in range(n):
        b = float(rng.randint(1, 5))
        c = float(rng.randint(0, 3))
        rows.append(Trace(
            {"slot_a": float(rng.randint(1, 6)), "slot_b": b, "slot_c": c},
            -b + c, "conflict",
            {"transfer_risk": 0.95, "ambiguity": 0.9},
            f"x-{i}",
        ))
    return rows


def run(seed):
    rng = random.Random(seed)
    a = make_family(rng, "A", 18, "linear")
    b = make_family(rng, "B", 18, "linear")
    c = make_family(rng, "C", 14, "linear")
    d = make_family(rng, "D", 14, "nested")
    ood = make_family(rng, "O", 12, "threshold")
    conflict = make_conflict(rng, 12)

    atr, aho = a[:-4], a[-4:]
    btr, bho = b[:-4], b[-4:]
    ctr, cho = c[:-4], c[-4:]
    dtr, dho = d[:-4], d[-4:]

    source, source_hold = atr + btr, aho + bho

    fixed = RecursiveCognitiveCompiler(max_depth=2, seed=seed)
    assert fixed.synthesize_process(dtr, dho, ood) is None

    learner = RecursiveCognitiveCompiler(max_depth=2, seed=seed)
    p1 = learner.invent_primitive(source, source_hold, ctr)
    assert p1 is not None

    p = learner.synthesize_process(ctr, cho, ood)
    assert p is not None
    ev = learner.evaluate(p, ctr, cho, cho, ood, conflict)
    assert ev.accepted
    assert learner.retain(p, ev)

    learner.generation = 1
    p2 = learner.invent_primitive(dtr, dho, dtr)
    assert p2 is not None
    assert any(
        n.get("op") == "macro" and n.get("id") == p1.primitive_id
        for n in learner._walk(p2.expression)
    )

    p_next = learner.synthesize_process(dtr, dho, ood)
    assert p_next is not None
    ev2 = learner.evaluate(p_next, dtr, dho, dho, ood, conflict)
    assert ev2.accepted
    assert learner.retain(p_next, ev2)

    _, rejected = learner.reuse(p_next, conflict)
    assert rejected == len(conflict)

    restored = RecursiveCognitiveCompiler.from_state(learner.export_state())
    assert restored.export_state() == learner.export_state()


def test_core014_recursive_compiler_principal_seed():
    run(0)


def test_core014_recursive_compiler_multi_seed():
    for seed in range(3):
        run(seed)
