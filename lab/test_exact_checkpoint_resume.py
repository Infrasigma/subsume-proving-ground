from __future__ import annotations

import json
import tempfile
from pathlib import Path

import pytest

from lab.acsie_open_ended_self_extending_proving import run_seed


def test_exact_checkpoint_roundtrip_preserves_future_trajectory():
    metadata = {"proving_sha": "checkpoint-test", "acsie_ref": "checkpoint-test"}

    with tempfile.TemporaryDirectory() as tmp:
        checkpoint = Path(tmp) / "state.json"

        uninterrupted = run_seed(
            101,
            4,
            checkpoint_metadata=metadata,
        )

        first_segment = run_seed(
            101,
            4,
            checkpoint_path=str(checkpoint),
            stop_after=2,
            checkpoint_metadata=metadata,
        )
        assert first_segment["completed_generations"] == 2
        assert checkpoint.exists()
        assert not first_segment["finite_open_ended_growth_gate"]

        resumed = run_seed(
            101,
            4,
            checkpoint_path=str(checkpoint),
            resume_from=str(checkpoint),
            checkpoint_metadata=metadata,
        )

        assert resumed["completed_generations"] == 4
        assert resumed["generations_detail"] == uninterrupted["generations_detail"]
        assert resumed["retained_count"] == uninterrupted["retained_count"]
        assert resumed["final_runtime_max_depth"] == uninterrupted["final_runtime_max_depth"]
        assert resumed["final_target_depth"] == uninterrupted["final_target_depth"]
        assert resumed["max_generated_expressions"] == uninterrupted["max_generated_expressions"]
        assert resumed["max_unique_search_states"] == uninterrupted["max_unique_search_states"]


def test_checkpoint_digest_rejects_tampering():
    metadata = {"proving_sha": "checkpoint-test", "acsie_ref": "checkpoint-test"}
    with tempfile.TemporaryDirectory() as tmp:
        checkpoint = Path(tmp) / "state.json"
        run_seed(
            102,
            3,
            checkpoint_path=str(checkpoint),
            checkpoint_metadata=metadata,
        )
        envelope = json.loads(checkpoint.read_text(encoding="utf-8"))
        envelope["generation_next"] = 99
        checkpoint.write_text(json.dumps(envelope), encoding="utf-8")
        with pytest.raises(ValueError, match="checkpoint full blob digest mismatch"):
            run_seed(
                102,
                3,
                resume_from=str(checkpoint),
                checkpoint_metadata=metadata,
            )


def test_checkpoint_runtime_identity_rejects_cross_revision_resume():
    metadata = {"proving_sha": "checkpoint-test-a", "acsie_ref": "runtime-a"}
    with tempfile.TemporaryDirectory() as tmp:
        checkpoint = Path(tmp) / "state.json"
        run_seed(
            103,
            3,
            checkpoint_path=str(checkpoint),
            stop_after=2,
            checkpoint_metadata=metadata,
        )
        with pytest.raises(ValueError, match="runtime identity mismatch"):
            run_seed(
                103,
                3,
                resume_from=str(checkpoint),
                checkpoint_metadata={
                    "proving_sha": "checkpoint-test-b",
                    "acsie_ref": "runtime-a",
                },
            )


def test_checkpoint_semantic_digest_is_deterministic_at_same_boundary():
    metadata = {"proving_sha": "checkpoint-test", "acsie_ref": "checkpoint-test"}

    with tempfile.TemporaryDirectory() as tmp:
        first = Path(tmp) / "first.json"
        second = Path(tmp) / "second.json"

        run_seed(
            104,
            3,
            checkpoint_path=str(first),
            stop_after=2,
            checkpoint_metadata=metadata,
        )
        run_seed(
            104,
            3,
            checkpoint_path=str(second),
            stop_after=2,
            checkpoint_metadata=metadata,
        )

        first_envelope = json.loads(first.read_text(encoding="utf-8"))
        second_envelope = json.loads(second.read_text(encoding="utf-8"))

        assert first_envelope["semantic_state_digest"] == second_envelope["semantic_state_digest"]
        assert first_envelope["state_digest"] == second_envelope["state_digest"]
        assert first_envelope["full_blob_digest"]
        assert second_envelope["full_blob_digest"]


def test_checkpoint_semantic_digest_ignores_forensic_event_order():
    metadata = {"proving_sha": "checkpoint-test", "acsie_ref": "checkpoint-test"}

    with tempfile.TemporaryDirectory() as tmp:
        checkpoint = Path(tmp) / "state.json"
        run_seed(
            105,
            3,
            checkpoint_path=str(checkpoint),
            checkpoint_metadata=metadata,
        )

        envelope = json.loads(checkpoint.read_text(encoding="utf-8"))
        original_semantic = envelope["semantic_state_digest"]
        original_events = list(envelope["learner_state"].get("events", []))
        envelope["learner_state"]["events"] = list(reversed(original_events))

        from lab.exact_checkpoint import _canonical_digest, _payload_without_digests

        payload = _payload_without_digests(envelope)
        envelope["full_blob_digest"] = _canonical_digest(payload)
        checkpoint.write_text(
            json.dumps(envelope, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )

        resumed = run_seed(
            105,
            3,
            resume_from=str(checkpoint),
            checkpoint_metadata=metadata,
        )
        assert resumed["resumed_from_checkpoint"] is True
        assert resumed["last_checkpoint_digest"] == original_semantic

