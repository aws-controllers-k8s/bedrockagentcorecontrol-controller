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

import time

import pytest
from acktest.aws.identity import get_account_id
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
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e.replacement_values import REPLACEMENT_VALUES

HARNESS_RESOURCE_PLURAL = "harnesses"
MODEL_ID = "us.amazon.nova-lite-v1:0"
SYNC_WAIT_PERIODS = 30
UPDATE_WAIT_AFTER_SECONDS = 10


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
def simple_harness(bedrockagentcorecontrol_client):
    harness_name = random_suffix_name("acktestharness", 32, delimiter="")
    resources = get_bootstrap_resources()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["HARNESS_NAME"] = harness_name
    replacements["ROLE_ARN"] = resources.HarnessRole.arn
    replacements["MODEL_ID"] = MODEL_ID

    resource_data = load_bedrockagentcorecontrol_resource(
        "harness",
        additional_replacements=replacements,
    )
    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        HARNESS_RESOURCE_PLURAL,
        harness_name,
        namespace="default",
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr)

    current = k8s.get_resource(ref)
    harness_id = current["status"]["harnessID"]
    _, deleted = k8s.delete_custom_resource(ref, wait_periods=10, period_length=15)
    assert deleted, "Harness CR was not deleted"
    _wait_for_not_found(
        bedrockagentcorecontrol_client.get_harness,
        f"Harness {harness_id}",
        harnessId=harness_id,
    )


@service_marker
@pytest.mark.canary
class TestHarness:
    def test_create_and_observe(
        self,
        simple_harness,
        bedrockagentcorecontrol_client,
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
            == get_account_id()
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
