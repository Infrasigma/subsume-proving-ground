#!/usr/bin/env bash
set -euo pipefail

: "${ACSIE_READ_TOKEN:?ACSIE_READ_TOKEN is required}"
: "${ACSIE_REF:?ACSIE_REF is required}"
: "${EXPECTED_HARNESS_BLOB:?EXPECTED_HARNESS_BLOB is required}"
: "${SEED:?SEED is required}"

WORK="/tmp/acsie-p7-frontier-diagnostic-${SEED}"
ASKPASS="/tmp/acsie-askpass-p7-${SEED}.sh"
OUT="${GITHUB_WORKSPACE}/evidence/p7-frontier-diagnostic/${SEED}"

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
import hashlib
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
expected_blob = sys.argv[2]
p = work / "research" / "run_l2_sequential_gate.sh"
s = p.read_text()

def blob_sha(text):
    data = text.encode()
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()

actual = blob_sha(s)
if actual != expected_blob:
    raise SystemExit(f"unexpected harness blob: {actual} != {expected_blob}")

anchor = '''        records = candidate_records(
            base_library[-1]["core"],
            discovery,
            selection,
            program_digests(base_library),
        )
'''

inject = '''        records = candidate_records(
            base_library[-1]["core"],
            discovery,
            selection,
            program_digests(base_library),
        )

        # Post-hoc diagnostic only: never influences candidate generation, selection,
        # thresholds, holdout, transfer, or retention. It asks whether the evaluator's
        # known p7 target-equivalent structure survived the generated frontier.
        if rule_id == "l2_r7_xyz_composed":
            target_program = (
                "xor",
                ("eq", ("atom", ("x",), "parity"), ("atom", ("z",), "parity")),
                ("eq", ("atom", ("y",), "sign"), ("atom", ("z",), "sign")),
            )
            exact = [
                r for r in records
                if r["program"] == target_program
            ]
            behavioral = [
                r for r in records
                if float(r["train_score"]) >= 0.999999
                and float(r["selection_score"]) >= 0.999999
            ]
            top = sorted(
                (
                    {
                        "index": r["index"],
                        "train_score": r["train_score"],
                        "selection_score": r["selection_score"],
                        "complexity": r["complexity"],
                        "program": r["program"],
                        "program_digest": r["program_digest"],
                    }
                    for r in records
                ),
                key=lambda r: (
                    float(r["selection_score"]),
                    float(r["train_score"]),
                    -float(r["complexity"]),
                    -int(r["index"]),
                ),
                reverse=True,
            )[:25]
            diagnostic = {
                "schema": "ACSIE.l2c.p7-frontier-diagnostic.v1",
                "seed": seed,
                "stage": stage_name,
                "rule_id": rule_id,
                "candidate_count_after_dedup": len(records),
                "exact_target_candidate_count": len(exact),
                "behaviorally_target_equivalent_candidate_count": len(behavioral),
                "exact_target_candidates": exact,
                "behaviorally_target_equivalent_candidates": [
                    {
                        "index": r["index"],
                        "train_score": r["train_score"],
                        "selection_score": r["selection_score"],
                        "program": r["program"],
                        "program_digest": r["program_digest"],
                    }
                    for r in behavioral
                ],
                "top_candidates": top,
                "selected_by_existing_selector": top[0] if top else None,
                "selection_layer_implicated": bool(behavioral) and not bool(
                    exact and selected is not None and selected["program"] == target_program
                ),
                "holdout_or_transfer_used_for_diagnostic": False,
                "target_identity_used_to_change_runtime_behavior": False,
            }
            (root / f"p7-frontier-diagnostic-{seed}.json").write_text(
                json.dumps(diagnostic, indent=2, sort_keys=True) + chr(10)
            )
'''

if anchor not in s:
    raise SystemExit("diagnostic insertion anchor not found")
s = s.replace(anchor, inject, 1)
p.write_text(s)
print("PATCHED")
PY

export PYTHONPATH="$WORK"
export ACSIE_L2_SEEDS="$SEED"
export ACSIE_SOURCE_COMMIT="$ACSIE_REF"
export ACSIE_SOURCE_TREE="$SOURCE_TREE"

set +e
(
  cd "$WORK"
  bash research/run_l2_sequential_gate.sh >"$OUT/gate.stdout.txt" 2>"$OUT/gate.stderr.txt"
)
RC=$?
set -e
printf '%s\n' "$RC" >"$OUT/gate.rc"

diag="/tmp/acsie-l2-sequential-run/p7-frontier-diagnostic-${SEED}.json"
if [[ -f "$diag" ]]; then
  cp "$diag" "$OUT/"
fi

python3 - "$OUT" "$ACSIE_REF" "$SOURCE_TREE" "$SEED" <<'PY'
import json
import pathlib
import sys
out = pathlib.Path(sys.argv[1])
rows = list(out.glob("p7-frontier-diagnostic-*.json"))
if len(rows) != 1:
    raise SystemExit(f"expected exactly one diagnostic artifact, found {len(rows)}")
d = json.loads(rows[0].read_text())
print(json.dumps({
    "schema": d.get("schema"),
    "seed": d.get("seed"),
    "candidate_count_after_dedup": d.get("candidate_count_after_dedup"),
    "exact_target_candidate_count": d.get("exact_target_candidate_count"),
    "behaviorally_target_equivalent_candidate_count": d.get("behaviorally_target_equivalent_candidate_count"),
    "selected_by_existing_selector": d.get("selected_by_existing_selector"),
    "selection_layer_implicated": d.get("selection_layer_implicated"),
    "source_commit": sys.argv[2],
    "source_tree": sys.argv[3],
}, indent=2, sort_keys=True))
PY

[[ "$RC" -ne 2 ]]

# Diagnostic execution trigger synchronized after workflow checkout plumbing activation.
