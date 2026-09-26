#!/usr/bin/env bash
set -euo pipefail

: "${ACSIE_READ_TOKEN:?ACSIE_READ_TOKEN is required}"
: "${ACSIE_REF:?ACSIE_REF is required}"

WORK=/tmp/acsie-private-lab
ASKPASS=/tmp/acsie-askpass.sh
VENV=/tmp/acsie-venv

cleanup() {
  rm -rf "$WORK" "$VENV" "$ASKPASS"
}
trap cleanup EXIT

cat > "$ASKPASS" <<'EOF'
#!/bin/sh
case "$1" in
  *Username*) printf '%s\n' 'x-access-token' ;;
  *) printf '%s\n' "$ACSIE_READ_TOKEN" ;;
esac
EOF
chmod 700 "$ASKPASS"

export GIT_ASKPASS="$ASKPASS"
export GIT_TERMINAL_PROMPT=0

git clone --depth 1 --branch "$ACSIE_REF"   https://github.com/Infrasigma/ACSIE.git "$WORK"

cd "$WORK"

echo "ACSIE_COMMIT=$(git rev-parse HEAD)"
echo "ACSIE_TREE=$(git rev-parse HEAD^{tree})"

python3 -m venv "$VENV"
. "$VENV/bin/activate"
python -m pip install --upgrade pip pytest

python -m compileall -q cognitive_core
PYTHONPATH="$WORK" pytest -q tests/test_core013_h003_integration.py

python - <<'PY'
from cognitive_core import GeneralLearner, CognitiveProcedure
from cognitive_core import continual_learning, open_capability, interactive_learning
l = GeneralLearner()
assert not l.cognitive_evolution_state()["enabled"]
assert CognitiveProcedure is not None
assert continual_learning is not None
assert open_capability is not None
assert interactive_learning is not None
print("current cognitive_core smoke: PASS")
PY

python -m pytest -q tests/test_core015_semantic_operator_discovery.py
python -m pytest -q tests/test_core016_open_semantic_substrate.py
python -m pytest -q tests/test_native_independent_core.py tests/test_native_recursive_intelligence.py
python research/native_independent_final_gate.py

python -m pytest -q tests/test_adaptive_capability_repair.py tests/test_asi_claim_gate.py

python research/native_failure_directed_recovery_v1.py 424242424242 24

echo "ACSIE_PRIVATE_LAB_RESULT=PASS"
