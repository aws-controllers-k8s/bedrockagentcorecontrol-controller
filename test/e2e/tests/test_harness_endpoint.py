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

"""Integration tests for the HarnessEndpoint API."""

import datetime
import os
import time
import uuid

import pytest
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from kubernetes import client as k8s_client

from e2e import (
    CRD_GROUP,
    CRD_VERSION,
    load_bedrockagentcorecontrol_resource,
    service_marker,
)
from e2e.harness_configuration import resource_namespace
from e2e.replacement_values import REPLACEMENT_VALUES

from .test_harness import (
    SYNC_WAIT_PERIODS,
    _wait_for_aws_status,
    _wait_for_not_found,
    _wait_for_runtime_status,
    simple_harness,  # noqa: F401 -- imported so pytest registers the fixture
)

HARNESS_ENDPOINT_RESOURCE_PLURAL = "harnessendpoints"
UPDATE_WAIT_AFTER_SECONDS = 10


def _invoke_harness(client, harness_arn, endpoint_name):
    response = client.invoke_harness(
        harnessArn=harness_arn,
        qualifier=endpoint_name,
        runtimeSessionId=str(uuid.uuid4()),
        messages=[
            {
                "role": "user",
                "content": [{"text": "Reply with the single word READY."}],
            }
        ],
    )
    text_chunks = []
    message_stopped = False
    for event in response["stream"]:
        if "runtimeClientError" in event:
            pytest.fail(f"Harness invocation failed: {event['runtimeClientError']}")
        if "contentBlockDelta" in event:
            delta = event["contentBlockDelta"].get("delta", {})
            if "text" in delta:
                text_chunks.append(delta["text"])
        if "messageStop" in event:
            message_stopped = True
    assert message_stopped, "Harness invocation stream did not finish a message"
    response_text = "".join(text_chunks).strip()
    assert response_text, "Harness invocation returned no text"
    return response_text


def _restart_controller():
    namespace = os.getenv("ACK_E2E_CONTROLLER_NAMESPACE")
    deployment_name = os.getenv("ACK_E2E_CONTROLLER_DEPLOYMENT")
    if not namespace or not deployment_name:
        pytest.fail(
            "ACK_E2E_CONTROLLER_NAMESPACE and ACK_E2E_CONTROLLER_DEPLOYMENT "
            "are required for the full lifecycle restart test"
        )

    api = k8s_client.AppsV1Api(k8s._get_k8s_api_client())
    restarted_at = datetime.datetime.now(datetime.timezone.utc).isoformat()
    api.patch_namespaced_deployment(
        deployment_name,
        namespace,
        {
            "spec": {
                "template": {
                    "metadata": {"annotations": {"acktest/restarted-at": restarted_at}}
                }
            }
        },
    )
    for _ in range(SYNC_WAIT_PERIODS):
        deployment = api.read_namespaced_deployment(deployment_name, namespace)
        desired = deployment.spec.replicas or 1
        if (
            deployment.status.observed_generation == deployment.metadata.generation
            and deployment.status.updated_replicas == desired
            and deployment.status.ready_replicas == desired
        ):
            return
        time.sleep(10)
    pytest.fail(f"Controller deployment {namespace}/{deployment_name} did not restart")


@pytest.fixture(scope="module")
def simple_harness_endpoint(request, bedrockagentcorecontrol_client):
    harness_ref, _ = request.getfixturevalue("simple_harness")
    namespace = resource_namespace()
    assert k8s.wait_on_condition(
        harness_ref,
        "ACK.ResourceSynced",
        "True",
        wait_periods=SYNC_WAIT_PERIODS,
    )
    harness_cr = k8s.get_resource(harness_ref)
    endpoint_name = random_suffix_name("acktestharnessendpoint", 40, delimiter="")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["ENDPOINT_NAME"] = endpoint_name
    replacements["HARNESS_NAME"] = harness_cr["metadata"]["name"]
    replacements["NAMESPACE"] = namespace
    replacements["TARGET_VERSION"] = harness_cr["status"]["harnessVersion"]
    resource_data = load_bedrockagentcorecontrol_resource(
        "harness_endpoint",
        additional_replacements=replacements,
    )
    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        HARNESS_ENDPOINT_RESOURCE_PLURAL,
        endpoint_name,
        namespace=namespace,
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr, harness_ref)

    endpoint = k8s.get_resource(ref)
    harness = k8s.get_resource(harness_ref)
    harness_id = harness["status"]["harnessID"]
    endpoint_name = endpoint["spec"]["name"]
    _, deleted = k8s.delete_custom_resource(ref, wait_periods=10, period_length=15)
    assert deleted, "HarnessEndpoint CR was not deleted"
    _wait_for_not_found(
        bedrockagentcorecontrol_client.get_harness_endpoint,
        f"HarnessEndpoint {endpoint_name}",
        harnessId=harness_id,
        endpointName=endpoint_name,
    )


@service_marker
@pytest.mark.canary
class TestHarnessEndpoint:
    def test_create_and_observe(
        self,
        simple_harness_endpoint,
        bedrockagentcorecontrol_client,
        bedrockagentcore_client,
        expected_aws_account_id,
    ):
        ref, _, harness_ref = simple_harness_endpoint

        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS
        )
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        assert cr["status"]["status"] == "READY"
        assert cr["status"]["ackResourceMetadata"]["arn"]
        assert cr["status"]["liveVersion"] == cr["spec"]["targetVersion"]

        harness_cr = k8s.get_resource(harness_ref)
        assert (
            harness_cr["status"]["ackResourceMetadata"]["ownerAccountID"]
            == expected_aws_account_id
        )
        aws_endpoint = bedrockagentcorecontrol_client.get_harness_endpoint(
            harnessId=harness_cr["status"]["harnessID"],
            endpointName=cr["spec"]["name"],
        )["endpoint"]
        assert aws_endpoint["status"] == "READY"
        assert aws_endpoint["liveVersion"] == cr["spec"]["targetVersion"]
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=cr["status"]["ackResourceMetadata"]["arn"]
        )["tags"]
        assert aws_tags["ack-e2e"] == "harness-endpoint"
        _invoke_harness(
            bedrockagentcore_client,
            harness_cr["status"]["ackResourceMetadata"]["arn"],
            cr["spec"]["name"],
        )

    def test_update_target_version(
        self,
        simple_harness_endpoint,
        bedrockagentcorecontrol_client,
        bedrockagentcore_client,
    ):
        endpoint_ref, _, harness_ref = simple_harness_endpoint
        harness_cr = k8s.get_resource(harness_ref)
        old_version = harness_cr["status"]["harnessVersion"]
        new_iterations = harness_cr["spec"]["maxIterations"] + 1

        k8s.patch_custom_resource(
            harness_ref, {"spec": {"maxIterations": new_iterations}}
        )
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            harness_ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=SYNC_WAIT_PERIODS,
        )
        harness_cr = k8s.get_resource(harness_ref)
        new_version = harness_cr["status"]["harnessVersion"]
        assert new_version != old_version

        k8s.patch_custom_resource(
            endpoint_ref,
            {
                "spec": {
                    "description": "Updated ACK e2e Harness endpoint",
                    "targetVersion": new_version,
                }
            },
        )
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            endpoint_ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=SYNC_WAIT_PERIODS,
        )

        endpoint_cr = k8s.get_resource(endpoint_ref)
        harness_cr = k8s.get_resource(harness_ref)
        aws_endpoint = bedrockagentcorecontrol_client.get_harness_endpoint(
            harnessId=harness_cr["status"]["harnessID"],
            endpointName=endpoint_cr["spec"]["name"],
        )["endpoint"]
        assert endpoint_cr["status"]["liveVersion"] == new_version
        assert aws_endpoint["liveVersion"] == new_version
        assert aws_endpoint["description"] == "Updated ACK e2e Harness endpoint"
        _invoke_harness(
            bedrockagentcore_client,
            harness_cr["status"]["ackResourceMetadata"]["arn"],
            endpoint_cr["spec"]["name"],
        )

    def test_corrects_drift_and_survives_controller_restart(
        self, simple_harness_endpoint, bedrockagentcorecontrol_client
    ):
        _, _, harness_ref = simple_harness_endpoint
        cr = k8s.get_resource(harness_ref)
        desired_iterations = cr["spec"]["maxIterations"]
        harness_id = cr["status"]["harnessID"]

        bedrockagentcorecontrol_client.update_harness(
            harnessId=harness_id,
            maxIterations=desired_iterations + 5,
        )
        _wait_for_aws_status(bedrockagentcorecontrol_client, harness_id)
        k8s.patch_custom_resource(
            harness_ref,
            {"metadata": {"annotations": {"acktest/full-drift-probe": "1"}}},
        )
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            harness_ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=SYNC_WAIT_PERIODS,
        )
        corrected = _wait_for_aws_status(bedrockagentcorecontrol_client, harness_id)
        assert corrected["maxIterations"] == desired_iterations

        _restart_controller()
        k8s.patch_custom_resource(
            harness_ref,
            {"metadata": {"annotations": {"acktest/post-restart-probe": "1"}}},
        )
        assert k8s.wait_on_condition(
            harness_ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=SYNC_WAIT_PERIODS,
        )
        after_restart = k8s.get_resource(harness_ref)
        _wait_for_runtime_status(
            bedrockagentcorecontrol_client,
            after_restart["status"]["agentRuntimeID"],
        )
