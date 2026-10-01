#!/usr/bin/env python3
import ast
import hashlib
import pathlib
import re
import sys

if len(sys.argv) != 3:
    raise SystemExit("usage: patch_l2c_cv_quarantine_20261001.py <harness> <expected_blob_sha>")

path = pathlib.Path(sys.argv[1])
expected = sys.argv[2]
s = path.read_text()

def blob_sha(text: str) -> str:
    data = text.encode()
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()

actual = blob_sha(s)
if actual != expected:
    raise SystemExit(f"harness blob mismatch: {actual} != {expected}")

def replace_once(anchor: str, replacement: str, label: str) -> None:
    global s
    count = s.count(anchor)
    if count != 1:
        raise SystemExit(f"{label}: expected one anchor, found {count}")
    s = s.replace(anchor, replacement, 1)

candidate_anchor = '''            selection_score = score_core(selection_core, selection)
            out.append({
                "index": index,
                "fingerprint": fp,
                "program": copy.deepcopy(candidate.context_program),
                "program_digest": program_digest,
                "candidate": copy.deepcopy(candidate),
                "train_score": train_score,
                "selection_score": selection_score,
                "complexity": len(json.dumps(candidate.context_program, sort_keys=True, default=str)),
            })
'''
candidate_new = '''            selection_score = score_core(selection_core, selection)
            selection_mode = os.environ.get("ACSIE_L2_SELECTION_MODE", "baseline")
            cv_scores = ()
            if selection_mode in {"cv", "cv_partition"}:
                pre_holdout = tuple(discovery) + tuple(selection)
                fold_count = 4
                fold_scores = []
                for fold in range(fold_count):
                    train_rows = tuple(
                        row for row_index, row in enumerate(pre_holdout)
                        if row_index % fold_count != fold
                    )
                    val_rows = tuple(
                        row for row_index, row in enumerate(pre_holdout)
                        if row_index % fold_count == fold
                    )
                    cv_core = NativeCognitiveCore(seed=750000 + index * fold_count + fold)
                    cv_core.learning_kernel = copy.deepcopy(candidate)
                    cv_core.observe_batch(train_rows)
                    cv_core._adaptive_context_cache.clear()
                    fold_scores.append(score_core(cv_core, val_rows))
                cv_scores = tuple(float(v) for v in fold_scores)
                cv_mean = statistics.mean(cv_scores) if cv_scores else 0.0
                cv_stdev = statistics.pstdev(cv_scores) if len(cv_scores) > 1 else 0.0
                selection_rank = float(cv_mean)
            else:
                cv_mean = None
                cv_stdev = None
                selection_rank = float(selection_score)
            out.append({
                "index": index,
                "fingerprint": fp,
                "program": copy.deepcopy(candidate.context_program),
                "program_digest": program_digest,
                "candidate": copy.deepcopy(candidate),
                "train_score": train_score,
                "selection_score": selection_score,
                "selection_rank": selection_rank,
                "cv_scores": list(cv_scores),
                "cv_mean": cv_mean,
                "cv_stdev": cv_stdev,
                "complexity": len(json.dumps(candidate.context_program, sort_keys=True, default=str)),
            })
'''
replace_once(candidate_anchor, candidate_new, "CV candidate scoring")

helper_anchor = '''    def acquire(stage, base_library, regime, seed_value, require_gain=True):
'''
helper_new = '''    def _selection_program_partition(base_core, program, rows):
        values = tuple(
            bool(base_core._context_program_eval(program, observation))
            for observation, _action, _target in rows
        )
        inverse = tuple(not value for value in values)
        return min(values, inverse)

    def _canonical_partition_preference(program):
        counts = {"neq": 0, "not": 0}
        def visit(node):
            if not isinstance(node, tuple) or not node:
                return
            op = str(node[0])
            if op in counts:
                counts[op] += 1
            for child in node[1:]:
                if isinstance(child, tuple):
                    visit(child)
        visit(program)
        return (
            int(counts["neq"]) + int(counts["not"]),
            int(counts["not"]),
            int(counts["neq"]),
            len(json.dumps(program, sort_keys=True, default=str)),
            json.dumps(program, sort_keys=True, default=str),
        )

    def acquire(stage, base_library, regime, seed_value, require_gain=True):
'''
replace_once(helper_anchor, helper_new, "CV partition helpers")

eligible_anchor = '''        eligible = [
            r for r in records
            if r["train_score"] >= 0.70 and r["selection_score"] >= 0.70
        ]
        eligible.sort(
            key=lambda r: (
                float(r["selection_score"]),
                float(r["train_score"]),
                -float(r["complexity"]),
                -int(r["index"]),
            ),
            reverse=True,
        )
        selected = eligible[0] if eligible else None
'''
eligible_new = '''        selection_mode = os.environ.get("ACSIE_L2_SELECTION_MODE", "baseline")
        eligible = [
            r for r in records
            if r["train_score"] >= 0.70 and (
                r["selection_score"] >= 0.70
                if selection_mode == "baseline"
                else r["selection_rank"] >= 0.70
            )
        ]
        if selection_mode in {"partition", "cv_partition"}:
            pre_holdout_rows = tuple(discovery) + tuple(selection)
            equivalence_classes = {}
            for record in eligible:
                key = _selection_program_partition(
                    base_library[-1]["core"],
                    record["program"],
                    pre_holdout_rows,
                )
                equivalence_classes.setdefault(key, []).append(record)
            eligible = [
                min(group, key=lambda r: _canonical_partition_preference(r["program"]))
                for group in equivalence_classes.values()
            ]
        eligible.sort(
            key=lambda r: (
                float(r["selection_rank"]),
                float(r["train_score"]),
                -float(r["complexity"]),
                -int(r["index"]),
            ),
            reverse=True,
        )
        selected = eligible[0] if eligible else None
'''
replace_once(eligible_anchor, eligible_new, "CV candidate selection")

payload_anchor = '''                "candidate_count":len(records),
                "eligible_count":len(eligible),
                "selected_index":selected["index"],
'''
payload_new = '''                "candidate_count":len(records),
                "eligible_count":len(eligible),
                "selection_mode":selection_mode,
                "selected_selection_score":selected["selection_score"],
                "selected_selection_rank":selected["selection_rank"],
                "selected_cv_mean":selected["cv_mean"],
                "selected_cv_stdev":selected["cv_stdev"],
                "cv_candidate_count":sum(1 for r in records if r["cv_mean"] is not None),
                "cv_statistics_complete":all(
                    r["cv_mean"] is not None
                    and r["cv_stdev"] is not None
                    and len(r["cv_scores"]) == 4
                    for r in records
                ),
                "semantic_partition":{
                    "enabled":selection_mode in {"partition","cv_partition"},
                    "class_count":len({
                        _selection_program_partition(
                            base_library[-1]["core"],
                            r["program"],
                            tuple(discovery) + tuple(selection),
                        )
                        for r in eligible
                    }) if selection_mode in {"partition","cv_partition"} else 0,
                },
                "selected_index":selected["index"],
'''
replace_once(payload_anchor, payload_new, "CV protocol evidence")

replace_once(
'''import copy
import hashlib
import json
import random
''',
'''import copy
import hashlib
import json
import os
import random
''',
"runtime os import",
)

route_anchor = '''    def route_metrics(branches, retention_rows):
        local = {item["name"] for item in branches if item["local_only"]}
        router = bank.NativeFourGenH28Router(
            [(item["name"], item["core"]) for item in branches],
            local_only_branches=local,
        )
        groups = [flat_rows(item["calibration"]) for item in branches]
        router.calibrate(groups)
        local_rows = {
            item["name"]: tuple(item["calibration"])
            for item in branches
            if item["local_only"]
        }
        if local_rows:
            router.calibrate_local(local_rows)
'''
route_new = '''    def route_metrics(branches, retention_rows, quarantine_branch=None):
        local = {item["name"] for item in branches if item["local_only"]}
        router = bank.NativeFourGenH28Router(
            [(item["name"], item["core"]) for item in branches],
            local_only_branches=local,
            quarantine_branch=quarantine_branch,
        )
        groups = [flat_rows(item["calibration"]) for item in branches]
        calibration_groups = groups
        if quarantine_branch is not None:
            calibration_groups = [
                group
                for (name, _item), group in zip(
                    ((item["name"], item) for item in branches),
                    groups,
                )
                if name != quarantine_branch
            ]
        router.calibrate(calibration_groups)
        local_rows = {
            item["name"]: tuple(item["calibration"])
            for item in branches
            if item["local_only"]
        }
        if local_rows:
            router.calibrate_local(local_rows)
'''
replace_once(route_anchor, route_new, "retention route quarantine")

retention_start = '    def retention_gate(before, after, retention_rows):'
retention_end = '    # 1) Reconstruct the frozen L1 state as a precondition only.'
start = s.index(retention_start)
end = s.index(retention_end, start)
replacement = '''    def _retention_route_trace(branches, retention_rows, quarantine_branch=None):
        local = {item["name"] for item in branches if item["local_only"]}
        router = bank.NativeFourGenH28Router(
            [(item["name"], item["core"]) for item in branches],
            local_only_branches=local,
            quarantine_branch=quarantine_branch,
        )
        groups = [flat_rows(item["calibration"]) for item in branches]
        calibration_groups = groups
        if quarantine_branch is not None:
            calibration_groups = [
                group
                for (name, _item), group in zip(
                    ((item["name"], item) for item in branches),
                    groups,
                )
                if name != quarantine_branch
            ]
        router.calibrate(calibration_groups)
        local_rows = {
            item["name"]: tuple(item["calibration"])
            for item in branches
            if item["local_only"]
        }
        if local_rows:
            router.calibrate_local(local_rows)
        trace = {}
        for target_name, rows in retention_rows:
            entries = []
            for index, (obs, action, target) in enumerate(rows):
                choice = router.choose(obs, action)
                entries.append({
                    "row_index": index,
                    "selected_branch": choice["branch"] if choice else None,
                    "emit": bool(choice["emit"]) if choice else False,
                    "prediction": choice["prediction"] if choice else None,
                    "state_key": choice["key"] if choice else None,
                })
            trace[target_name] = entries
        return trace

    def retention_gate(before, after, retention_rows):
        newcomer = after[-1]["name"]
        baseline = route_metrics(before, retention_rows, quarantine_branch=None)
        post = route_metrics(after, retention_rows, quarantine_branch=newcomer)
        baseline_trace = _retention_route_trace(before, retention_rows, quarantine_branch=None)
        post_trace = _retention_route_trace(after, retention_rows, quarantine_branch=newcomer)
        route_deltas = {}
        for name in baseline_trace:
            b = baseline_trace[name]
            p = post_trace[name]
            deltas = []
            if len(b) != len(p):
                deltas.append({"error":"trace_length_mismatch","baseline_length":len(b),"post_length":len(p)})
            else:
                for b_row, p_row in zip(b, p):
                    if (
                        b_row["selected_branch"] != p_row["selected_branch"]
                        or b_row["emit"] != p_row["emit"]
                        or b_row["prediction"] != p_row["prediction"]
                        or b_row["state_key"] != p_row["state_key"]
                    ):
                        deltas.append({
                            "row_index":b_row["row_index"],
                            "baseline":b_row,
                            "post":p_row,
                        })
            route_deltas[name] = deltas

        checks = {}
        for name in baseline:
            b = baseline[name]
            p = post[name]
            checks[name] = {
                "accuracy_non_decrease": p["accuracy_on_covered"] + 1e-12 >= b["accuracy_on_covered"],
                "coverage_non_decrease": p["coverage"] + 1e-12 >= b["coverage"],
                "expected_branch_non_decrease": p["selected_expected_branch_rate"] + 1e-12 >= b["selected_expected_branch_rate"],
                "newest_branch_intrusion_zero": p["newest_branch_intrusion"] == 0,
                "route_invariance_pass": len(route_deltas.get(name, ())) == 0,
            }
        return {
            "baseline":baseline,
            "post":post,
            "route_invariance":{
                "all_rows_unchanged": all(not route_deltas.get(name) for name in route_deltas),
                "changed_row_count": sum(len(v) for v in route_deltas.values()),
                "route_deltas": route_deltas,
            },
            "checks":checks,
            "all_checks_pass": all(all(v.values()) for v in checks.values()),
        }

'''
s = s[:start] + replacement + s[end:]

# Static validation of the generated harness.
if "\nimport os\n" not in s:
    raise SystemExit("static validation: import os missing")
if "ACSIE_L2_SELECTION_MODE" not in s:
    raise SystemExit("static validation: selection hook missing")
if "quarantine_branch" not in s:
    raise SystemExit("static validation: quarantine hook missing")
if "route_invariance" not in s:
    raise SystemExit("static validation: route invariance missing")
match = re.search(r"python3 -u - .*?<<['"]PY['"]\n(.*?)\nPY\n", s, flags=re.S)
if match is None:
    raise SystemExit("static validation: embedded Python block missing")
ast.parse(match.group(1), filename=str(path))
path.write_text(s)
print("PATCH_APPLIED=cv+semantic-partition+newcomer-quarantine+route-invariance")
print("ORIGINAL_HARNESS_BLOB=" + actual)
print("PATCHED_HARNESS_SHA256=" + hashlib.sha256(s.encode()).hexdigest())
