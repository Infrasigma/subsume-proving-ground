#!/usr/bin/env python3
import ast
import hashlib
import pathlib
import sys

if len(sys.argv) != 3:
    raise SystemExit("usage: patch_l2c_pr50_route_audit.py <harness> <expected_blob_sha>")

path = pathlib.Path(sys.argv[1])
expected = sys.argv[2]
s = path.read_text()

def blob_sha(text):
    data = text.encode()
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\\0" + data).hexdigest()

actual = blob_sha(s)
if actual != expected:
    raise SystemExit(f"harness blob mismatch: {actual} != {expected}")

def replace_once(anchor, replacement, label):
    global s
    count = s.count(anchor)
    if count != 1:
        raise SystemExit(f"{label}: expected one anchor, found {count}")
    s = s.replace(anchor, replacement, 1)

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
                for (name, item), group in zip(
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
replace_once(route_anchor, route_new, "quarantine wiring")

retention_anchor = '''    def retention_gate(before, after, retention_rows):
        baseline = route_metrics(before, retention_rows)
        post = route_metrics(after, retention_rows)
        checks = {}
        for name in baseline:
            b = baseline[name]
            p = post[name]
            checks[name] = {
                "accuracy_non_decrease": p["accuracy_on_covered"] + 1e-12 >= b["accuracy_on_covered"],
                "coverage_non_decrease": p["coverage"] + 1e-12 >= b["coverage"],
                "expected_branch_non_decrease": p["selected_expected_branch_rate"] + 1e-12 >= b["selected_expected_branch_rate"],
                "newest_branch_intrusion_zero": p["newest_branch_intrusion"] == 0,
            }
        return {
            "baseline":baseline,
            "post":post,
            "checks":checks,
            "all_checks_pass": all(all(v.values()) for v in checks.values()),
        }

'''
retention_new = '''    def _retention_route_trace(branches, retention_rows, quarantine_branch=None):
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
                for (name, item), group in zip(
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
            for row_index, (obs, action, target) in enumerate(rows):
                choice = router.choose(obs, action)
                entries.append({
                    "row_index": row_index,
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
                deltas.append({"error":"trace_length_mismatch"})
            else:
                for b_row, p_row in zip(b, p):
                    if (
                        b_row["selected_branch"] != p_row["selected_branch"]
                        or b_row["emit"] != p_row["emit"]
                        or b_row["prediction"] != p_row["prediction"]
                        or b_row["state_key"] != p_row["state_key"]
                    ):
                        deltas.append({
                            "row_index": b_row["row_index"],
                            "baseline": b_row,
                            "post": p_row,
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
            "baseline": baseline,
            "post": post,
            "route_invariance": {
                "all_rows_unchanged": all(not v for v in route_deltas.values()),
                "changed_row_count": sum(len(v) for v in route_deltas.values()),
                "route_deltas": route_deltas,
            },
            "checks": checks,
            "all_checks_pass": all(all(v.values()) for v in checks.values()),
        }

'''
replace_once(retention_anchor, retention_new, "route invariance")

if "quarantine_branch=quarantine_branch" not in s:
    raise SystemExit("static validation: quarantine wiring missing")
if "route_invariance_pass" not in s:
    raise SystemExit("static validation: route invariance missing")

match_start = s.find("python3 -u - ")
if match_start < 0:
    raise SystemExit("static validation: embedded Python not found")
py_start = s.find("\n", match_start) + 1
py_end = s.find("\nPY\n", py_start)
if py_end < 0:
    raise SystemExit("static validation: Python heredoc terminator missing")
ast.parse(s[py_start:py_end], filename=str(path))

path.write_text(s)
print("PATCH_APPLIED=pr50-explicit-quarantine-wiring+route-invariance-audit")
print("ORIGINAL_HARNESS_BLOB=" + actual)
print("PATCHED_HARNESS_SHA256=" + hashlib.sha256(s.encode()).hexdigest())
