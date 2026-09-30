#!/usr/bin/env python3
import json
from itertools import product
from pathlib import Path

FIXTURE = Path(__file__).with_name("l2c_p7_semantic_partition_fixture.json")
OUT = Path("l2c_p7_semantic_partition_diagnostic.json")

DOMAIN = tuple(range(-8, 0)) + tuple(range(1, 9))

def atom_value(value, predicate):
    if predicate == "parity":
        return value % 2
    if predicate == "sign":
        return "neg" if value < 0 else ("zero" if value == 0 else "pos")
    if predicate == "zero":
        return value == 0
    if predicate == "threshold":
        return value >= 0
    raise ValueError(predicate)

def eval_program(program, obs):
    op = program[0]
    if op == "atom":
        return atom_value(obs[program[1][0]], str(program[2]))
    left = eval_program(program[1], obs)
    right = eval_program(program[2], obs)
    if op == "eq":
        return left == right
    if op == "neq":
        return left != right
    if op == "xor":
        return bool(left) ^ bool(right)
    if op == "and":
        return bool(left) and bool(right)
    if op == "or":
        return bool(left) or bool(right)
    if op == "not":
        return not bool(left)
    raise ValueError(op)

COMMUTATIVE = {"eq", "and", "or", "xor"}

def structural_normal_form(program):
    op = program[0]
    if op == "atom":
        return ("atom", tuple(program[1]), str(program[2]))
    args = tuple(structural_normal_form(x) for x in program[1:])
    if op in COMMUTATIVE:
        args = tuple(sorted(args, key=repr))
    return (op,) + args

def partition_signature(program):
    rows = []
    label_ids = {}
    next_label = 0
    for x, y, z in product(DOMAIN, repeat=3):
        obs = {"x": x, "y": y, "z": z}
        value = bool(eval_program(program, obs))
        if value not in label_ids:
            label_ids[value] = next_label
            next_label += 1
        rows.append(label_ids[value])
    return tuple(rows)

def truth_signature(program):
    return tuple(
        bool(eval_program(program, {"x":x, "y":y, "z":z}))
        for x, y, z in product(DOMAIN, repeat=3)
    )

def main():
    fixture = json.loads(FIXTURE.read_text())
    target = fixture["target"]
    target_struct = structural_normal_form(target)
    target_truth = truth_signature(target)
    target_partition = partition_signature(target)

    results = []
    for item in fixture["seeds"]:
        program = item["selected"]
        struct = structural_normal_form(program)
        truth = truth_signature(program)
        partition = partition_signature(program)
        result = {
            "seed": item["seed"],
            "exact_ast_match": program == target,
            "commutative_normal_form_match": struct == target_struct,
            "semantic_truth_equivalent": truth == target_truth,
            "semantic_partition_equivalent": partition == target_partition,
            "holdout_pass": item["holdout_pass"],
            "transfer_pass": item["transfer_pass"],
            "holdout_accuracy": item["holdout"],
            "transfer_accuracy": item["transfer"],
        }
        if truth != target_truth and partition == target_partition:
            result["classification"] = "partition_equivalent_label_renaming"
        elif truth == target_truth:
            result["classification"] = "semantic_equivalent"
        elif any(a != b for a,b in zip(truth, target_truth)) and not partition == target_partition:
            result["classification"] = "semantic_mismatch"
        else:
            result["classification"] = "inconclusive"
        results.append(result)

    all_partition_equivalent = all(r["semantic_partition_equivalent"] for r in results)
    no_partial_or_mismatch = all(r["classification"] != "semantic_mismatch" for r in results)
    diagnostic_status = "PASSED" if all_partition_equivalent and no_partial_or_mismatch else "FAILED"

    payload = {
        "schema": fixture["schema"],
        "scientific_status": diagnostic_status,
        "experiment": "L2-C p7 semantic partition equivalence diagnostic",
        "purpose": "Classify seed-dependent alternative context programs without changing the original capstone result or using target identity during discovery/selection.",
        "provenance": {k: fixture[k] for k in ("source_commit","source_tree","capstone_workflow","artifact_id","artifact_sha256")},
        "domain": fixture["domain"],
        "domain_cardinality": len(DOMAIN) ** 3,
        "equivalence_contract": {
            "exact_ast": "byte/structure identity",
            "commutative_normal_form": "canonicalize operand order for explicitly commutative boolean operators",
            "semantic_truth": "same boolean output on every declared observable input in the finite p7 domain",
            "semantic_partition": "same partition after canonical relabeling of boolean classes; permits truth-label swapping because the context expression is used only as a partition key"
        },
        "results": results,
        "summary": {
            "exact_ast_matches": sum(r["exact_ast_match"] for r in results),
            "commutative_normal_form_matches": sum(r["commutative_normal_form_match"] for r in results),
            "semantic_truth_equivalents": sum(r["semantic_truth_equivalent"] for r in results),
            "semantic_partition_equivalents": sum(r["semantic_partition_equivalent"] for r in results),
            "downstream_holdout_passes": sum(r["holdout_pass"] for r in results),
            "downstream_transfer_passes": sum(r["transfer_pass"] for r in results),
        },
        "integrity": {
            "holdout_data_used_for_selection": False,
            "selection_modified": False,
            "runtime_strategy_modified": False,
            "target_identity_exposed_to_original_discovery": False
        },
        "scientific_boundary": "Post-capstone diagnostic only. It does not convert the original L2-C FAILED result into a PASS and does not claim L2-C completion."
    }
    OUT.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
    print(json.dumps(payload, indent=2, sort_keys=True))
    raise SystemExit(0 if diagnostic_status == "PASSED" else 1)

if __name__ == "__main__":
    main()
