from __future__ import annotations

import json
import os
import random
import tempfile
from dataclasses import asdict
from hashlib import sha256
from pathlib import Path
from typing import Any, Mapping

from cognitive_core.open_ended_growth import GrowthEvidence, OpenEndedRecursiveCognitiveCompiler

CHECKPOINT_SCHEMA = "ACSIE.open-ended.exact-checkpoint.v2"


def _to_jsonable_state(value: Any) -> Any:
    if isinstance(value, tuple):
        return [_to_jsonable_state(item) for item in value]
    if isinstance(value, list):
        return [_to_jsonable_state(item) for item in value]
    if isinstance(value, dict):
        return {str(k): _to_jsonable_state(v) for k, v in value.items()}
    return value


def _from_jsonable_state(value: Any) -> Any:
    if isinstance(value, list):
        return tuple(_from_jsonable_state(item) for item in value)
    if isinstance(value, dict):
        return {k: _from_jsonable_state(v) for k, v in value.items()}
    return value


def runtime_identity(metadata: Mapping[str, Any] | None = None) -> dict[str, str]:
    extra = dict(metadata or {})
    return {
        "proving_sha": str(extra.get("proving_sha") or os.environ.get("GITHUB_SHA") or "unknown"),
        "acsie_ref": str(extra.get("acsie_ref") or os.environ.get("ACSIE_REF") or "unknown"),
    }


def export_learner_state(
    learner: OpenEndedRecursiveCognitiveCompiler,
    *,
    metadata: Mapping[str, Any] | None = None,
) -> dict[str, Any]:
    state = learner.export_state()
    state["runtime_identity"] = runtime_identity(metadata)
    state["rng_state"] = _to_jsonable_state(learner.rng.getstate())
    state["open_ended"] = {
        "growth_credit": float(learner.growth_credit),
        "max_observed_depth": int(learner.max_observed_depth),
        "growth_history": [asdict(item) for item in learner.growth_history],
        "last_search_stats": dict(learner.last_search_stats),
    }
    return state


def restore_learner_state(
    state: Mapping[str, Any],
    *,
    expected_metadata: Mapping[str, Any] | None = None,
) -> OpenEndedRecursiveCognitiveCompiler:
    saved_identity = dict(state.get("runtime_identity", {}))
    expected_identity = runtime_identity(expected_metadata)
    for key in ("proving_sha", "acsie_ref"):
        saved = str(saved_identity.get(key, "unknown"))
        expected = str(expected_identity.get(key, "unknown"))
        if saved != "unknown" and expected != "unknown" and saved != expected:
            raise ValueError(
                f"checkpoint runtime identity mismatch for {key}: saved={saved} expected={expected}"
            )

    base_state = {
        key: value
        for key, value in dict(state).items()
        if key not in {"runtime_identity", "rng_state", "open_ended"}
    }
    learner = OpenEndedRecursiveCognitiveCompiler.from_state(base_state)

    open_ended = dict(state.get("open_ended", {}))
    learner.growth_credit = float(open_ended.get("growth_credit", 0.0))
    learner.max_observed_depth = int(
        open_ended.get("max_observed_depth", learner.meta_policy["max_depth"])
    )
    learner.growth_history = [
        GrowthEvidence(**item)
        for item in open_ended.get("growth_history", [])
    ][-512:]
    learner.last_search_stats = dict(open_ended.get("last_search_stats", {}))
    learner.last_primitive_candidate_frontier = ()
    learner.last_process_candidate_frontier = ()
    learner.last_process_validation_summary = {}

    if "rng_state" in state:
        learner.rng.setstate(_from_jsonable_state(state["rng_state"]))

    return learner


def _semantic_payload(payload: Mapping[str, Any]) -> dict[str, Any]:
    """Project the restart-causal state out of forensic telemetry.

    The semantic digest must ignore diagnostics whose ordering can vary without
    changing future computation. The full checkpoint digest still binds every
    stored byte for forensic integrity.
    """
    projected = json.loads(
        json.dumps(payload, sort_keys=True, separators=(",", ":"))
    )
    learner_state = dict(projected.get("learner_state", {}))
    learner_state.pop("events", None)
    open_ended = dict(learner_state.get("open_ended", {}))
    open_ended.pop("last_search_stats", None)
    open_ended.pop("growth_history", None)
    learner_state["open_ended"] = open_ended
    projected["learner_state"] = learner_state
    projected.pop("generations_out", None)
    return projected


def _canonical_digest(value: Mapping[str, Any]) -> str:
    canonical = json.dumps(value, sort_keys=True, separators=(",", ":"))
    return sha256(canonical.encode("utf-8")).hexdigest()


def _payload_without_digests(envelope: Mapping[str, Any]) -> dict[str, Any]:
    payload = dict(envelope)
    payload.pop("state_digest", None)
    payload.pop("semantic_state_digest", None)
    payload.pop("full_blob_digest", None)
    return payload


def save_checkpoint(
    path: str | Path,
    *,
    seed: int,
    generation_next: int,
    initial_depth: int,
    local_rng: random.Random,
    learner: OpenEndedRecursiveCognitiveCompiler,
    retained: list[Mapping[str, Any]],
    generations_out: list[Mapping[str, Any]],
    metadata: Mapping[str, Any] | None = None,
) -> str:
    payload = {
        "schema": CHECKPOINT_SCHEMA,
        "seed": int(seed),
        "generation_next": int(generation_next),
        "initial_depth": int(initial_depth),
        "local_rng_state": _to_jsonable_state(local_rng.getstate()),
        "learner_state": export_learner_state(learner, metadata=metadata),
        "retained": [dict(item) for item in retained],
        "generations_out": [dict(item) for item in generations_out],
    }

    semantic_state_digest = _canonical_digest(_semantic_payload(payload))
    full_blob_digest = _canonical_digest(payload)

    envelope = dict(payload)
    envelope["state_digest"] = semantic_state_digest
    envelope["semantic_state_digest"] = semantic_state_digest
    envelope["full_blob_digest"] = full_blob_digest
    serialized = json.dumps(envelope, sort_keys=True, indent=2) + "\n"

    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        "w",
        encoding="utf-8",
        dir=str(target.parent),
        prefix=target.name + ".",
        delete=False,
    ) as handle:
        handle.write(serialized)
        temporary = Path(handle.name)
    temporary.replace(target)
    return semantic_state_digest

def load_checkpoint(
    path: str | Path,
    *,
    expected_seed: int,
    expected_metadata: Mapping[str, Any] | None = None,
) -> dict[str, Any]:
    target = Path(path)
    envelope = json.loads(target.read_text(encoding="utf-8"))
    if envelope.get("schema") != CHECKPOINT_SCHEMA:
        raise ValueError(
            f"unsupported checkpoint schema: {envelope.get('schema')!r}"
        )
    if int(envelope.get("seed")) != int(expected_seed):
        raise ValueError(
            f"checkpoint seed mismatch: saved={envelope.get('seed')} expected={expected_seed}"
        )

    semantic_state_digest = str(
        envelope.get("semantic_state_digest", envelope.get("state_digest", ""))
    )
    saved_state_digest = str(envelope.get("state_digest", semantic_state_digest))
    if saved_state_digest != semantic_state_digest:
        raise ValueError(
            "checkpoint semantic digest alias mismatch: "
            f"state_digest={saved_state_digest} semantic_state_digest={semantic_state_digest}"
        )

    payload = _payload_without_digests(envelope)
    actual_full_blob_digest = _canonical_digest(payload)
    saved_full_blob_digest = str(envelope.get("full_blob_digest", ""))
    if saved_full_blob_digest != actual_full_blob_digest:
        raise ValueError(
            "checkpoint full blob digest mismatch: "
            f"saved={saved_full_blob_digest} actual={actual_full_blob_digest}"
        )

    actual_semantic_state_digest = _canonical_digest(_semantic_payload(payload))
    if semantic_state_digest != actual_semantic_state_digest:
        raise ValueError(
            "checkpoint semantic digest mismatch: "
            f"saved={semantic_state_digest} actual={actual_semantic_state_digest}"
        )

    identity = runtime_identity(expected_metadata)
    saved_identity = dict(
        dict(payload.get("learner_state", {})).get("runtime_identity", {})
    )
    for key in ("proving_sha", "acsie_ref"):
        saved = str(saved_identity.get(key, "unknown"))
        expected = str(identity.get(key, "unknown"))
        if saved != "unknown" and expected != "unknown" and saved != expected:
            raise ValueError(
                f"checkpoint runtime identity mismatch for {key}: saved={saved} expected={expected}"
            )

    return envelope

def restore_checkpoint(
    envelope: Mapping[str, Any],
    *,
    expected_metadata: Mapping[str, Any] | None = None,
):
    from lab.acsie_open_ended_self_extending_proving import HiddenCapability

    local_rng = random.Random()
    local_rng.setstate(_from_jsonable_state(envelope["local_rng_state"]))
    learner = restore_learner_state(
        envelope["learner_state"],
        expected_metadata=expected_metadata,
    )
    retained = [
        HiddenCapability(
            hidden_id=str(item["hidden_id"]),
            primitive_id=str(item["primitive_id"]),
            expression=dict(item["expression"]),
            depth=int(item["depth"]),
            generation=int(item["generation"]),
            parent_hidden_id=item.get("parent_hidden_id"),
            parent_hidden_ids=tuple(item.get("parent_hidden_ids", ())),
        )
        for item in envelope.get("retained", [])
    ]
    generations_out = [dict(item) for item in envelope.get("generations_out", [])]
    return {
        "seed": int(envelope["seed"]),
        "generation_next": int(envelope["generation_next"]),
        "initial_depth": int(envelope["initial_depth"]),
        "local_rng": local_rng,
        "learner": learner,
        "retained": retained,
        "generations_out": generations_out,
        "state_digest": str(
            envelope.get("semantic_state_digest", envelope["state_digest"])
        ),
        "semantic_state_digest": str(
            envelope.get("semantic_state_digest", envelope["state_digest"])
        ),
        "full_blob_digest": str(envelope["full_blob_digest"]),
    }