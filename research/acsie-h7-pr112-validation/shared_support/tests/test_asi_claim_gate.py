from __future__ import annotations

from research.asi_claim_gate_v1 import qualify


def test_asi_claim_gate_refuses_self_certification():
    result = qualify(
        {
            "schema": "ACSIE.asi-qualification.attestation.v1",
            "source_commit": "abc",
            "independent_evaluator": False,
        },
        "abc",
    )
    assert result["qualified"] is False
    assert result["qualification_status"] == "NOT_QUALIFIED"


def test_asi_claim_gate_requires_all_external_flags():
    attestation = {
        "schema": "ACSIE.asi-qualification.attestation.v1",
        "source_commit": "abc",
        "independent_evaluator": True,
        "final_hidden_suite": True,
        "network_independence_demonstrated": True,
        "external_replication_pass": True,
        "all_pre_registered_thresholds_passed": True,
        "independent_evaluator_id": "external-1",
        "evaluator_manifest_hash": "deadbeef",
    }
    result = qualify(attestation, "abc")
    assert result["qualified"] is True
    assert result["qualification_status"] == "ASI-qualified-under-ACSIE-protocol-v1"
