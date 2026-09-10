#!/usr/bin/env python3
"""Phase 2.1 fact-semantics conformance wrapper.

Uses the existing differential suite unchanged except for the independently
implemented temporal reference surface, then adds explicit semantic assertions
for temporal multiplicity. No endpoint experiment is invoked.
"""
import pathlib
import sys
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))
import conformance.differential as d

ROOT = pathlib.Path(__file__).resolve().parents[1]
d.CLOSURE_REF = ROOT / "conformance/reference/closure_temporal_v2.go"

def temporal_semantics_regression():
    a = "0000000000000001"
    b = "0000000000000002"
    history = [
        {"state":{"location":a},"available_actions":[a],"last_action":None,"last_result":None,"terminated":False},
        {"state":{"location":a},"available_actions":[a,b],"last_action":a,"last_result":{"status":"ACCEPTED"},"terminated":False},
        {"state":{"location":a},"available_actions":[b],"last_action":b,"last_result":{"status":"ACCEPTED"},"terminated":False},
        {"state":{"location":a},"available_actions":[a,b],"last_action":b,"last_result":{"status":"ACCEPTED"},"terminated":False},
        {"state":{"location":a},"available_actions":[a,b],"last_action":a,"last_result":{"status":"ACCEPTED"},"terminated":False},
    ]
    case = {
        "name":"temporal_multiplicity_regression", "seed":10, "history":history,
        "candidate":{"atoms":[{"predicate":"ACTION","args":["target"]}],"consequence":"ENABLES(target)"},
        "attempts":4,"terminal":"SUCCESS",
        "attribution":{"evidence":[]},"novelty_a":[],"novelty_b":[]
    }
    p, pr = d.run([sys.executable, str(d.CLOSURE_PROD)], case)
    r, rr = d.run(["go", "run", str(d.CLOSURE_REF)], case)
    d.compare(p, r, case["name"], pr, rr)
    by_key = {(f["p"], tuple(f["a"])): f["at"] for f in p["facts"]}
    assert by_key[("AVAILABLE", (a,))] == [0,1,3,4], by_key
    assert by_key[("ACTION", (a,))] == [1,4], by_key
    assert p["retrieval"]["status"] == "RETRIEVED"
    assert p["prediction"]["consequence"] == "ENABLES(target)"
    assert p["attribution"] == "UNATTRIBUTABLE"
    print("PASS temporal_fact_multiplicity_semantics")

def main():
    d.legacy()
    d.closure()
    temporal_semantics_regression()
    print("PHASE2_1_CONFORMANCE_TEMPORAL_DIFFERENTIAL_PASS")

if __name__ == "__main__":
    main()
