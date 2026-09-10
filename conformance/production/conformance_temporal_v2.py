#!/usr/bin/env python3
"""Phase 2.1 temporal fact-semantics production conformance surface."""
import pathlib
import importlib.util

ROOT = pathlib.Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("serl_production_temporal_base", ROOT / "conformance/production/conformance.py")
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)
_original_compile_facts = base.compile_facts

# The frozen roles are semantic roles (target/intervention/contrast/context),
# not anonymous variables. Canonicalization therefore sorts/deduplicates atoms
# without renaming those semantic role identifiers. The prior production
# alpha-normalizer could rename `contrast`/`target` while leaving the fixed
# consequence term unchanged, turning a valid NONENABLES rule into an invalid
# candidate.
def canonical_temporal(rule):
    atoms = sorted({base.cj({"predicate": a["predicate"], "args": list(a["args"]) }).decode() for a in rule["atoms"]})
    return {"atoms":[__import__("json").loads(x) for x in atoms],"consequence":rule["consequence"]}

def compile_facts_temporal(history):
    facts = _original_compile_facts(history)
    return sorted(facts, key=base.cj)

base.canonical = canonical_temporal
base.compile_facts = compile_facts_temporal

if __name__ == "__main__":
    base.main()
