#!/usr/bin/env bash
set -euo pipefail

: "${ACSIE_READ_TOKEN:?ACSIE_READ_TOKEN is required}"
: "${ACSIE_REF:?ACSIE_REF is required}"
: "${EXPECTED_HARNESS_BLOB:?EXPECTED_HARNESS_BLOB is required}"
: "${SEED:?SEED is required}"

WORK="/tmp/acsie-hr13-cv-corrected-${SEED}"
ASKPASS="/tmp/acsie-askpass-corrected-${SEED}.sh"
OUT="${GITHUB_WORKSPACE}/evidence/l2c-hr13-cv-partition-corrected/${SEED}"
rm -rf "$WORK" "$ASKPASS" "$OUT"
mkdir -p "$OUT"
trap 'rm -rf "$WORK" "$ASKPASS"' EXIT

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
import ast
import hashlib
import pathlib
import re
import sys

work = pathlib.Path(sys.argv[1])
expected = sys.argv[2]
path = work / "research" / "run_l2_sequential_gate.sh"
s = path.read_text()

def blob_sha(text):
    data = text.encode()
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()

if blob_sha(s) != expected:
    raise SystemExit("harness blob mismatch")

def replace_once(anchor, replacement, label):
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
replace_once(candidate_anchor, candidate_new, "candidate scoring")

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
replace_once(helper_anchor, helper_new, "acquire helper insertion")

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
replace_once(eligible_anchor, eligible_new, "candidate selection")

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
                "selected_index":selected["index"],
'''
replace_once(payload_anchor, payload_new, "selection evidence payload")

# Inject and validate the actual ACSIE harness.
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

selection_anchor = '''        selected = eligible[0] if eligible else None

        if selected is None:
            return {
                "status":"FAILED",
                "failure_boundary":"no_candidate_passed_internal_selection_gate",
                "regime":regime,
                "discovery":{"prior_scores":prior_scores,"known_bank_insufficient":known_bank_insufficient,"lower_bound":lower},
                "candidate_count":len(records),
            }, None
'''
selection_new = '''        partition_class_count = 0
        partition_class_sizes = []
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
            partition_class_count = len(equivalence_classes)
            partition_class_sizes = sorted(
                (len(group) for group in equivalence_classes.values()),
                reverse=True,
            )
        selected = eligible[0] if eligible else None

        if selected is None:
            return {
                "status":"FAILED",
                "failure_boundary":"no_candidate_passed_internal_selection_gate",
                "regime":regime,
                "discovery":{
                    "prior_scores":prior_scores,
                    "known_bank_insufficient":known_bank_insufficient,
                    "lower_bound":lower,
                    "selection_mode":selection_mode,
                    "cv_candidate_count":sum(
                        1 for r in records if r["cv_mean"] is not None
                    ),
                    "cv_statistics_complete":all(
                        r["cv_mean"] is not None
                        and r["cv_stdev"] is not None
                        and len(r["cv_scores"]) == 4
                        for r in records
                    ),
                    "semantic_partition":{
                        "enabled":selection_mode in {"partition","cv_partition"},
                        "class_count":partition_class_count,
                        "class_sizes":partition_class_sizes,
                    },
                },
                "candidate_count":len(records),
            }, None
'''
payload_anchor = '''                "selected_cv_stdev":selected["cv_stdev"],
                "selected_index":selected["index"],
'''
payload_new = '''                "selected_cv_stdev":selected["cv_stdev"],
                "cv_candidate_count":sum(
                    1 for r in records if r["cv_mean"] is not None
                ),
                "cv_statistics_complete":all(
                    r["cv_mean"] is not None
                    and r["cv_stdev"] is not None
                    and len(r["cv_scores"]) == 4
                    for r in records
                ),
                "semantic_partition":{
                    "enabled":selection_mode in {"partition","cv_partition"},
                    "class_count":partition_class_count,
                    "class_sizes":partition_class_sizes,
                },
                "selected_index":selected["index"],
'''
replace_once(selection_anchor, selection_new, "CV partition protocol evidence")
replace_once(payload_anchor, payload_new, "CV selection protocol evidence")

# Static validation of the generated Python payload.
if "\nimport os\n" not in s:
    raise SystemExit("static validation: missing import os")
if "ACSIE_L2_SELECTION_MODE" not in s:
    raise SystemExit("static validation: missing selection mode")
if "cv_scores" not in s or "cv_mean" not in s or "cv_stdev" not in s:
    raise SystemExit("static validation: missing CV statistics")
if "semantic_partition" not in s:
    raise SystemExit("static validation: missing semantic partition evidence")
match = re.search(r"python3 -u - .*?<<['\"]PY['\"]\n(.*?)\nPY\n", s, flags=re.S)
if match is None:
    raise SystemExit("static validation: embedded Python heredoc not found")
ast.parse(match.group(1), filename="run_l2_sequential_gate.sh:embedded-python")
path.write_text(s)
print("STATIC_VALIDATION=PASSED")
print("PATCHED_HARNESS_SHA256=" + hashlib.sha256(s.encode()).hexdigest())
PY

export PYTHONPATH="$WORK"
export ACSIE_L2_SEEDS="$SEED"
export ACSIE_L2_SELECTION_MODE="cv_partition"
export ACSIE_SOURCE_COMMIT="$(git -C "$WORK" rev-parse HEAD)"
export ACSIE_SOURCE_TREE="$SOURCE_TREE"

bash -n "$WORK/research/run_l2_sequential_gate.sh"
python3 - "$WORK/research/run_l2_sequential_gate.sh" <<'PY'
import ast
import re
import sys
from pathlib import Path
text = Path(sys.argv[1]).read_text()
m = re.search(r"python3 -u - .*?<<['\"]PY['\"]\n(.*?)\nPY\n", text, flags=re.S)
if m is None:
    raise SystemExit("runtime preflight: embedded Python missing")
ast.parse(m.group(1), filename="run_l2_sequential_gate.sh:embedded-python")
py = m.group(1)
for required in ("import os", "ACSIE_L2_SELECTION_MODE", "cv_scores", "_selection_program_partition"):
    if required not in py:
        raise SystemExit(f"runtime preflight: missing {required}")
print("RUNTIME_PREFLIGHT=PASSED")
PY
set +e
( cd "$WORK" && bash research/run_l2_sequential_gate.sh >"$OUT/gate.stdout.txt" 2>"$OUT/gate.stderr.txt" )
RC=$?
set -e
printf '%s\n' "$RC" >"$OUT/gate.rc"

if compgen -G "/tmp/acsie-l2-sequential-run/l2a-*.json" >/dev/null; then
  cp /tmp/acsie-l2-sequential-run/l2a-*.json "$OUT/"
else
  cp -f "$WORK"/l2a-*.json "$OUT/" 2>/dev/null || true
fi

python3 - "$OUT" <<'PY'
import json
import pathlib
import sys
out = pathlib.Path(sys.argv[1])
rows = list(out.glob("l2a-*.json"))
if len(rows) != 1:
    raise SystemExit(f"expected exactly one scientific artifact, found {len(rows)}")
d = json.loads(rows[0].read_text())

protocol_violations = []
payloads = []
if isinstance(d.get("l1_precondition"), dict):
    payloads.append(("L1-PRECONDITION", d["l1_precondition"]))
for stage in d.get("stages", []):
    if isinstance(stage, dict):
        payloads.append((stage.get("stage", "unknown"), stage))

for label, payload in payloads:
    disc = payload.get("discovery")
    if not isinstance(disc, dict):
        protocol_violations.append(f"{label}: missing discovery evidence")
        continue
    if disc.get("selection_mode") != "cv_partition":
        protocol_violations.append(
            f"{label}: selection_mode={disc.get('selection_mode')!r}"
        )
    if "cv_candidate_count" not in disc:
        protocol_violations.append(f"{label}: missing cv_candidate_count")
    if "cv_statistics_complete" not in disc:
        protocol_violations.append(f"{label}: missing cv_statistics_complete")
    if disc.get("candidate_count", 0) > 0 and disc.get("cv_statistics_complete") is not True:
        protocol_violations.append(f"{label}: incomplete four-fold CV statistics")
    partition = disc.get("semantic_partition")
    if not isinstance(partition, dict):
        protocol_violations.append(f"{label}: missing semantic_partition evidence")
    elif partition.get("enabled") is not True:
        protocol_violations.append(f"{label}: semantic partition not enabled")

if protocol_violations:
    validation = {
        "schema": "ACSIE.l2c.hr13-cv-partition-corrected.protocol.v1",
        "protocol_status": "INVALID",
        "scientific_status": "NOT_INTERPRETABLE",
        "seed": d.get("seed"),
        "violations": protocol_violations,
    }
    (out / "protocol_validation.json").write_text(
        json.dumps(validation, indent=2, sort_keys=True) + "\n"
    )
    print(json.dumps(validation, indent=2, sort_keys=True))
    raise SystemExit(2)

print(json.dumps({
    "schema": "ACSIE.l2c.hr13-cv-partition-corrected.v1",
    "seed": d.get("seed"),
    "scientific_status": d.get("scientific_status"),
    "failure_boundary": d.get("failure_boundary"),
    "stages": [
        {
            "stage": s.get("stage"),
            "status": s.get("status"),
            "selected_program": (s.get("discovery") or {}).get("selected_program"),
            "selected_cv_mean": (s.get("discovery") or {}).get("selected_cv_mean"),
            "selected_cv_stdev": (s.get("discovery") or {}).get("selected_cv_stdev"),
            "selection_mode": (s.get("discovery") or {}).get("selection_mode"),
        } for s in d.get("stages", [])
    ],
    "integrity": d.get("integrity"),
}, sort_keys=True, indent=2))
PY

if [[ "$RC" -eq 2 ]]; then exit 2; fi
exit 0
