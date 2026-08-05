# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the Harness API."""

import os
import time

import pytest
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from botocore.exceptions import ClientError

from e2e import (
    CRD_GROUP,
    CRD_VERSION,
    load_bedrockagentcorecontrol_resource,
    service_marker,
)
from e2e.harness_configuration import execution_role_arn, resource_namespace
from e2e.replacement_values import REPLACEMENT_VALUES

HARNESS_RESOURCE_PLURAL = "harnesses"
SYNC_WAIT_PERIODS = 30
UPDATE_WAIT_AFTER_SECONDS = 10


def _model_id():
    model_id = os.getenv("ACK_E2E_HARNESS_MODEL_ID")
    if not model_id:
        pytest.skip("ACK_E2E_HARNESS_MODEL_ID is required for Harness tests")
    return model_id


def _wait_for_aws_status(client, harness_id, expected="READY"):
    for _ in range(SYNC_WAIT_PERIODS):
        harness = client.get_harness(harnessId=harness_id)["harness"]
        if harness["status"] == expected:
            return harness
        time.sleep(10)
    pytest.fail(f"Harness {harness_id} did not reach {expected}")


def _wait_for_runtime_status(client, runtime_id, expected="READY"):
    for _ in range(SYNC_WAIT_PERIODS):
        runtime = client.get_agent_runtime(agentRuntimeId=runtime_id)
        if runtime["status"] == expected:
            return runtime
        time.sleep(10)
    pytest.fail(f"AgentRuntime {runtime_id} did not reach {expected}")


def _wait_for_not_found(operation, resource_description, **kwargs):
    last_error = None
    for _ in range(SYNC_WAIT_PERIODS):
        try:
            operation(**kwargs)
        except ClientError as error:
            last_error = error
            if error.response["Error"]["Code"] == "ResourceNotFoundException":
                return
        time.sleep(10)
    pytest.fail(
        f"{resource_description} still exists in AWS after deletion; "
        f"last error: {last_error}"
    )


@pytest.fixture(scope="module")
def simple_harness(bedrockagentcorecontrol_client, expected_aws_account_id):
    harness_name = random_suffix_name("acktestharness", 32, delimiter="")
    namespace = resource_namespace()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["HARNESS_NAME"] = harness_name
    replacements["NAMESPACE"] = namespace
    replacements["ROLE_ARN"] = execution_role_arn(expected_aws_account_id)
    replacements["MODEL_ID"] = _model_id()

    resource_data = load_bedrockagentcorecontrol_resource(
        "harness",
        additional_replacements=replacements,
    )
    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        HARNESS_RESOURCE_PLURAL,
        harness_name,
        namespace=namespace,
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr)

    current = k8s.get_resource(ref)
    harness_id = current["status"]["harnessID"]
    runtime_id = current["status"]["agentRuntimeID"]
    _, deleted = k8s.delete_custom_resource(ref, wait_periods=10, period_length=15)
    assert deleted, "Harness CR was not deleted"
    _wait_for_not_found(
        bedrockagentcorecontrol_client.get_harness,
        f"Harness {harness_id}",
        harnessId=harness_id,
    )
    _wait_for_not_found(
        bedrockagentcorecontrol_client.get_agent_runtime,
        f"managed AgentRuntime {runtime_id}",
        agentRuntimeId=runtime_id,
    )


@service_marker
@pytest.mark.canary
class TestHarness:
    def test_create_and_observe(
        self,
        simple_harness,
        bedrockagentcorecontrol_client,
        expected_aws_account_id,
    ):
        ref, _ = simple_harness

        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS
        )
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        assert cr["status"]["status"] == "READY"
        assert cr["status"]["ackResourceMetadata"]["arn"]
        assert (
            cr["status"]["ackResourceMetadata"]["ownerAccountID"]
            == expected_aws_account_id
        )
        assert cr["status"]["harnessID"]
        assert cr["status"]["harnessVersion"]
        assert cr["status"]["agentRuntimeARN"]
        assert cr["status"]["agentRuntimeID"]
        assert cr["status"]["agentRuntimeName"]

        aws_harness = bedrockagentcorecontrol_client.get_harness(
            harnessId=cr["status"]["harnessID"]
        )["harness"]
        assert aws_harness["harnessName"] == cr["spec"]["harnessName"]
        assert aws_harness["status"] == "READY"
        aws_runtime = _wait_for_runtime_status(
            bedrockagentcorecontrol_client, cr["status"]["agentRuntimeID"]
        )
        assert aws_runtime["agentRuntimeArn"] == cr["status"]["agentRuntimeARN"]
        assert aws_runtime["agentRuntimeName"] == cr["status"]["agentRuntimeName"]
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=cr["status"]["ackResourceMetadata"]["arn"]
        )["tags"]
        assert aws_tags["ack-e2e"] == "harness"

    def test_update(self, simple_harness, bedrockagentcorecontrol_client):
        ref, _ = simple_harness
        before = k8s.get_resource(ref)
        old_version = before["status"]["harnessVersion"]

        k8s.patch_custom_resource(ref, {"spec": {"maxIterations": 4}})
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS
        )

        after = k8s.get_resource(ref)
        aws_harness = _wait_for_aws_status(
            bedrockagentcorecontrol_client, after["status"]["harnessID"]
        )
        assert after["status"]["harnessVersion"] != old_version
        assert aws_harness["maxIterations"] == 4

    def test_corrects_out_of_band_drift(
        self, simple_harness, bedrockagentcorecontrol_client
    ):
        ref, _ = simple_harness
        cr = k8s.get_resource(ref)
        harness_id = cr["status"]["harnessID"]

        bedrockagentcorecontrol_client.update_harness(
            harnessId=harness_id,
            maxIterations=8,
        )
        _wait_for_aws_status(bedrockagentcorecontrol_client, harness_id)

        # Trigger an immediate reconciliation instead of waiting for the
        # controller's periodic resync interval.
        k8s.patch_custom_resource(
            ref,
            {"metadata": {"annotations": {"acktest/drift-probe": "1"}}},
        )
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS
        )

        aws_harness = _wait_for_aws_status(bedrockagentcorecontrol_client, harness_id)
        assert aws_harness["maxIterations"] == 4

    def test_validation_error_is_terminal(self):
        harness_name = random_suffix_name("ackinvalidharness", 32, delimiter="")
        namespace = resource_namespace()
        resource_data = load_bedrockagentcorecontrol_resource(
            "harness_invalid",
            additional_replacements={
                "HARNESS_NAME": harness_name,
                "NAMESPACE": namespace,
            },
        )
        ref = k8s.CustomResourceReference(
            CRD_GROUP,
            CRD_VERSION,
            HARNESS_RESOURCE_PLURAL,
            harness_name,
            namespace=namespace,
        )

        k8s.create_custom_resource(ref, resource_data)
        try:
            assert k8s.wait_on_condition(ref, "ACK.Terminal", "True", wait_periods=10)
        finally:
            _, deleted = k8s.delete_custom_resource(
                ref, wait_periods=3, period_length=5
            )
            assert deleted, "Invalid Harness CR was not deleted"
