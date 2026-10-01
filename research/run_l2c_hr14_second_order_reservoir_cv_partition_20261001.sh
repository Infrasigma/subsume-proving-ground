#!/usr/bin/env bash
set -euo pipefail

: "${ACSIE_READ_TOKEN:?ACSIE_READ_TOKEN is required}"
: "${ACSIE_REF:?ACSIE_REF is required}"
: "${EXPECTED_HARNESS_BLOB:?EXPECTED_HARNESS_BLOB is required}"
: "${SEED:?SEED is required}"

WORK="/tmp/acsie-combined-retention-${SEED}"
ASKPASS="/tmp/acsie-askpass-${SEED}.sh"
VENV="/tmp/acsie-venv-${SEED}"
OUT="${GITHUB_WORKSPACE}/evidence/hr14-second-order-reservoir-cv-partition/${SEED}"

rm -rf "$WORK" "$VENV" "$ASKPASS" "$OUT"
mkdir -p "$OUT"

trap 'rm -rf "$WORK" "$VENV" "$ASKPASS"' EXIT

cat >"$ASKPASS" <<'EOF'
#!/bin/sh
case "$1" in
  *Username*) printf '%s\n' 'x-access-token' ;;
  *) printf '%s\n' "$ACSIE_READ_TOKEN" ;;
esac
EOF
chmod 700 "$ASKPASS"

export GIT_ASKPASS="$ASKPASS"
export GIT_TERMINAL_PROMPT=0

git clone -q --no-checkout https://github.com/Infrasigma/ACSIE.git "$WORK"
git -C "$WORK" checkout --detach --force "$ACSIE_REF"
test "$(git -C "$WORK" rev-parse HEAD)" = "$ACSIE_REF"
SOURCE_TREE="$(git -C "$WORK" rev-parse HEAD^{tree})"
test "$(git -C "$WORK" hash-object research/run_l2_sequential_gate.sh)" = "$EXPECTED_HARNESS_BLOB"

python3 - "$WORK" "$EXPECTED_HARNESS_BLOB" <<'PY'
import hashlib
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
expected = sys.argv[2]
p = work / "research" / "run_l2_sequential_gate.sh"
s = p.read_text()

def blob_sha(text):
    data = text.encode()
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()

actual = blob_sha(s)
if actual != expected:
    raise SystemExit(f"unexpected current-main harness blob: {actual} != {expected}")

old = '''    def route_metrics(branches, retention_rows):
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

new = '''    def route_metrics(branches, retention_rows, quarantine_branch=None):
        local = {item["name"] for item in branches if item["local_only"]}
        router = bank.NativeFourGenH28Router(
            [(item["name"], item["core"]) for item in branches],
            local_only_branches=local,
            quarantine_branch=quarantine_branch,
        )
        groups = [flat_rows(item["calibration"]) for item in branches]
        if quarantine_branch is None:
            calibration_groups = groups
        else:
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

if old not in s:
    raise SystemExit("route_metrics patch anchor not found")
s = s.replace(old, new, 1)

start = s.index("    def retention_gate(before, after, retention_rows):")
end = s.index("    # 1) Reconstruct the frozen L1 state as a precondition only.", start)

replacement = '''    def route_trace(branches, retention_rows, quarantine_branch=None):
        local = {item["name"] for item in branches if item["local_only"]}
        router = bank.NativeFourGenH28Router(
            [(item["name"], item["core"]) for item in branches],
            local_only_branches=local,
            quarantine_branch=quarantine_branch,
        )
        groups = [flat_rows(item["calibration"]) for item in branches]
        if quarantine_branch is None:
            calibration_groups = groups
        else:
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
                    "target_branch": target_name,
                    "selected_branch": choice["branch"] if choice else None,
                    "emit": bool(choice["emit"]) if choice else False,
                    "prediction": choice["prediction"] if choice else None,
                    "state_key": choice["key"] if choice else None,
                })
            trace[target_name] = entries
        return trace

    def retention_gate(before, after, retention_rows):
        baseline = route_metrics(before, retention_rows, quarantine_branch=None)
        post = route_metrics(after, retention_rows, quarantine_branch=after[-1]["name"])
        baseline_trace = route_trace(before, retention_rows, quarantine_branch=None)
        post_trace = route_trace(after, retention_rows, quarantine_branch=after[-1]["name"])
        route_deltas = {}
        for name in baseline_trace:
            b_rows = baseline_trace[name]
            p_rows = post_trace[name]
            if len(b_rows) != len(p_rows):
                route_deltas[name] = [{
                    "error": "trace_length_mismatch",
                    "baseline_length": len(b_rows),
                    "post_length": len(p_rows),
                }]
                continue
            deltas = []
            for b, p in zip(b_rows, p_rows):
                changed = (
                    b["selected_branch"] != p["selected_branch"]
                    or b["emit"] != p["emit"]
                    or b["prediction"] != p["prediction"]
                    or b["state_key"] != p["state_key"]
                )
                if changed:
                    deltas.append({
                        "row_index": b["row_index"],
                        "target_branch": name,
                        "baseline": b,
                        "post": p,
                    })
            route_deltas[name] = deltas

        checks = {}
        for name in baseline:
            b = baseline[name]
            p = post[name]
            route_invariance_pass = len(route_deltas.get(name, ())) == 0
            checks[name] = {
                "accuracy_non_decrease": p["accuracy_on_covered"] + 1e-12 >= b["accuracy_on_covered"],
                "coverage_non_decrease": p["coverage"] + 1e-12 >= b["coverage"],
                "expected_branch_non_decrease": p["selected_expected_branch_rate"] + 1e-12 >= b["selected_expected_branch_rate"],
                "newest_branch_intrusion_zero": p["newest_branch_intrusion"] == 0,
                "route_invariance_pass": route_invariance_pass,
            }
        return {
            "baseline": baseline,
            "post": post,
            "route_invariance": {
                "all_rows_unchanged": all(not route_deltas.get(name) for name in route_deltas),
                "changed_row_count": sum(len(v) for v in route_deltas.values()),
                "route_deltas": route_deltas,
            },
            "checks": checks,
            "all_checks_pass": all(all(v.values()) for v in checks.values()),
        }

'''
s = s[:start] + replacement + s[end:]
p.write_text(s)
print("PATCHED_HARNESS_SHA256=" + hashlib.sha256(s.encode()).hexdigest())
PY


# H-R14 selection intervention: discovery-only 4-fold CV + semantic partition.
python3 - "$WORK" <<'PY'
import hashlib
import os
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
p = work / "research" / "run_l2_sequential_gate.sh"
s = p.read_text()

def blob_sha(text):
    data = text.encode()
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()

selection_marker = """            selection_score = score_core(selection_core, selection)
            out.append({
"""
selection_replacement = """            selection_score = score_core(selection_core, selection)
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
                    cv_core = NativeCognitiveCore(
                        seed=750000 + index * fold_count + fold
                    )
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
"""
if selection_marker not in s:
    raise SystemExit("candidate selection anchor not found")
s = s.replace(selection_marker, selection_replacement, 1)

field_marker = """                "selection_score": selection_score,
                "complexity": len(json.dumps(candidate.context_program, sort_keys=True, default=str)),
"""
field_replacement = """                "selection_score": selection_score,
                "selection_rank": selection_rank,
                "cv_scores": list(cv_scores),
                "cv_mean": cv_mean,
                "cv_stdev": cv_stdev,
                "complexity": len(json.dumps(candidate.context_program, sort_keys=True, default=str)),
"""
if field_marker not in s:
    raise SystemExit("candidate field anchor not found")
s = s.replace(field_marker, field_replacement, 1)

acquire_marker = """    def acquire(stage, base_library, regime, seed_value, require_gain=True):
"""
acquire_replacement = """    def _selection_program_partition(base_core, program, rows):
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
"""
if acquire_marker not in s:
    raise SystemExit("acquire function anchor not found")
s = s.replace(acquire_marker, acquire_replacement, 1)

eligible_marker = """        eligible = [
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
"""
eligible_replacement = """        selection_mode = os.environ.get("ACSIE_L2_SELECTION_MODE", "baseline")
        if selection_mode != "cv_partition":
            raise RuntimeError(f"unexpected_selection_mode:{selection_mode}")
        eligible = [
            r for r in records
            if r["train_score"] >= 0.70 and r["selection_rank"] >= 0.70
        ]
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
"""
if eligible_marker not in s:
    raise SystemExit("eligible selection anchor not found")
s = s.replace(eligible_marker, eligible_replacement, 1)

payload_marker = """                "candidate_count":len(records),
                "eligible_count":len(eligible),
                "selected_index":selected["index"],
"""
payload_replacement = """                "candidate_count":len(records),
                "eligible_count":len(eligible),
                "selection_mode":selection_mode,
                "selected_selection_score":selected["selection_score"],
                "selected_selection_rank":selected["selection_rank"],
                "selected_cv_mean":selected["cv_mean"],
                "selected_cv_stdev":selected["cv_stdev"],
                "selected_index":selected["index"],
"""
if payload_marker not in s:
    raise SystemExit("payload selection anchor not found")
s = s.replace(payload_marker, payload_replacement, 1)

p.write_text(s)
print("HR14_SELECTOR_PATCH_APPLIED_SHA256=" + hashlib.sha256(s.encode()).hexdigest())
PY

export ACSIE_L2_SELECTION_MODE="cv_partition"

export PYTHONPATH="$WORK"
export ACSIE_L2_SEEDS="$SEED"
export ACSIE_SOURCE_COMMIT="$(git -C "$WORK" rev-parse HEAD)"
export ACSIE_SOURCE_TREE="$SOURCE_TREE"

set +e
(
  cd "$WORK"
  bash research/run_l2_sequential_gate.sh >"$OUT/gate.stdout.txt" 2>"$OUT/gate.stderr.txt"
)
RC=$?
set -e

printf '%s\n' "$RC" >"$OUT/gate.rc"

if compgen -G /tmp/acsie-l2-sequential-run/l2a-*.json >/dev/null; then
  cp /tmp/acsie-l2-sequential-run/l2a-*.json "$OUT/"
else
  cp -f "$WORK"/l2a-*.json "$OUT/" 2>/dev/null || true
fi

python3 - "$OUT" "$ACSIE_REF" "$SOURCE_TREE" "$SEED" <<'PY'
import json
import pathlib
import sys

out = pathlib.Path(sys.argv[1])
rows = list(out.glob("l2a-*.json"))
if len(rows) != 1:
    raise SystemExit(f"expected exactly one scientific artifact, found {len(rows)}")
d = json.loads(rows[0].read_text())
print(json.dumps({
    "schema": "ACSIE.l2c.hr14.second-order-reservoir-cv-partition.v1",
    "seed": d.get("seed"),
    "scientific_status": d.get("scientific_status"),
    "failure_boundary": d.get("failure_boundary"),
    "source_commit": sys.argv[2],
    "source_tree": sys.argv[3],
    "retention_checks": (d.get("retention_after_p7") or {}).get("checks"),
    "route_invariance": (d.get("retention_after_p7") or {}).get("route_invariance"),
    "integrity": d.get("integrity"),
}, sort_keys=True, indent=2))
PY

# Scientific FAIL is valid evidence; only an execution crash blocks artifact generation.
if [[ "$RC" -eq 2 ]]; then
  exit 2
fi
exit 0
