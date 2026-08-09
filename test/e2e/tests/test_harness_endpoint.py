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

import time

import pytest
from acktest.aws.identity import get_account_id
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name

from e2e import (
    CRD_GROUP,
    CRD_VERSION,
    load_bedrockagentcorecontrol_resource,
    service_marker,
)
from e2e.replacement_values import REPLACEMENT_VALUES

from .test_harness import (
    SYNC_WAIT_PERIODS,
    _wait_for_not_found,
    simple_harness,  # noqa: F401 -- imported so pytest registers the fixture
)

HARNESS_ENDPOINT_RESOURCE_PLURAL = "harnessendpoints"
UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_harness_endpoint(request, bedrockagentcorecontrol_client):
    harness_ref, _ = request.getfixturevalue("simple_harness")
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
        namespace="default",
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
            == get_account_id()
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

    def test_update_target_version(
        self,
        simple_harness_endpoint,
        bedrockagentcorecontrol_client,
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
