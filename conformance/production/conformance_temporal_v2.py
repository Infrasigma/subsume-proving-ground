#!/usr/bin/env python3
"""Phase 2.1 temporal fact-semantics production conformance surface."""
import pathlib
import importlib.util

ROOT = pathlib.Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("serl_production_temporal_base", ROOT / "conformance/production/conformance.py")
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)
_original_compile_facts = base.compile_facts
_original_novelty = base.novelty

def canonical_temporal(rule):
    atoms = sorted({base.cj({"predicate": a["predicate"], "args": list(a["args"]) }).decode() for a in rule["atoms"]})
    return {"atoms":[__import__("json").loads(x) for x in atoms],"consequence":rule["consequence"]}

def compile_facts_temporal(history):
    facts = _original_compile_facts(history)
    return sorted(facts, key=base.cj)

def novelty_temporal(a, b):
    value = _original_novelty(a, b)
    return int(value) if value in (0, 1) else value

# Scientific role names are semantic identifiers, so they are not alpha-renamed.
# The prior production alpha-normalizer could rename contrast/target in atoms
# while leaving the fixed consequence term unchanged, invalidating a legitimate
# NONENABLES candidate. Integral novelty is also emitted using the frozen
# numeric boundary rule rather than Python's 0.0/1.0 lexical form.
base.canonical = canonical_temporal
base.compile_facts = compile_facts_temporal
base.novelty = novelty_temporal

if __name__ == "__main__":
    base.main()
