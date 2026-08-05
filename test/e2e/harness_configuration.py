"""Fail-closed environment configuration for Harness lifecycle tests."""

import os


def resource_namespace():
    namespace = os.getenv("ACK_E2E_RESOURCE_NAMESPACE")
    if not namespace:
        raise RuntimeError(
            "ACK_E2E_RESOURCE_NAMESPACE is required for Harness lifecycle tests"
        )
    return namespace


def execution_role_arn(expected_aws_account_id):
    role_arn = os.getenv("ACK_E2E_HARNESS_EXECUTION_ROLE_ARN")
    if not role_arn:
        raise RuntimeError(
            "ACK_E2E_HARNESS_EXECUTION_ROLE_ARN is required for Harness lifecycle tests"
        )
    expected_prefix = f"arn:aws:iam::{expected_aws_account_id}:role/"
    if not role_arn.startswith(expected_prefix):
        raise RuntimeError(
            "ACK_E2E_HARNESS_EXECUTION_ROLE_ARN must identify a role in "
            f"expected account {expected_aws_account_id}"
        )
    return role_arn
