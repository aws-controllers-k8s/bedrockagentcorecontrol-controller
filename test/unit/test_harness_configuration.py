"""Tests for fail-closed Harness lifecycle configuration."""

from pathlib import Path

import pytest

from harness_configuration import execution_role_arn, resource_namespace


RESOURCE_DIRECTORY = Path(__file__).parents[1] / "e2e" / "resources"


def test_resource_namespace_is_required(monkeypatch):
    monkeypatch.delenv("ACK_E2E_RESOURCE_NAMESPACE", raising=False)

    with pytest.raises(RuntimeError, match="ACK_E2E_RESOURCE_NAMESPACE is required"):
        resource_namespace()


def test_resource_namespace_uses_configured_value(monkeypatch):
    monkeypatch.setenv("ACK_E2E_RESOURCE_NAMESPACE", "ack-test")

    assert resource_namespace() == "ack-test"


def test_execution_role_is_required(monkeypatch):
    monkeypatch.delenv("ACK_E2E_HARNESS_EXECUTION_ROLE_ARN", raising=False)

    with pytest.raises(
        RuntimeError,
        match="ACK_E2E_HARNESS_EXECUTION_ROLE_ARN is required",
    ):
        execution_role_arn("123456789012")


def test_execution_role_must_belong_to_expected_account(monkeypatch):
    monkeypatch.setenv(
        "ACK_E2E_HARNESS_EXECUTION_ROLE_ARN",
        "arn:aws:iam::999999999999:role/wrong-account",
    )

    with pytest.raises(RuntimeError, match="expected account 123456789012"):
        execution_role_arn("123456789012")


def test_execution_role_uses_configured_value(monkeypatch):
    role_arn = "arn:aws:iam::123456789012:role/ack-harness-e2e"
    monkeypatch.setenv("ACK_E2E_HARNESS_EXECUTION_ROLE_ARN", role_arn)

    assert execution_role_arn("123456789012") == role_arn


@pytest.mark.parametrize(
    "resource_name",
    ["harness.yaml", "harness_endpoint.yaml", "harness_invalid.yaml"],
)
def test_harness_manifests_use_configured_namespace_placeholder(resource_name):
    manifest = (RESOURCE_DIRECTORY / resource_name).read_text()

    assert "namespace: $NAMESPACE" in manifest
    assert "namespace: default" not in manifest
