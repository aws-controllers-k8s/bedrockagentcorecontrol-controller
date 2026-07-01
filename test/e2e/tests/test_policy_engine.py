# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for PolicyEngine API.
"""

import pytest
import time

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

POLICY_ENGINE_RESOURCE_PLURAL = "policyengines"
UPDATE_WAIT_AFTER_SECONDS = 10
# PolicyEngine resources may take longer to provision than other resource types
SYNC_WAIT_PERIODS = 30


@pytest.fixture(scope="module")
def simple_policy_engine():
    pe_name = random_suffix_name("acktest-pe", 32, delimiter="-")
    # AWS API requires name to match ^[A-Za-z][A-Za-z0-9_]*$ (no hyphens)
    # Replace hyphens with underscores for the spec.name field
    pe_spec_name = pe_name.replace("-", "_")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["POLICY_ENGINE_NAME"] = pe_name
    replacements["POLICY_ENGINE_SPEC_NAME"] = pe_spec_name

    resource_data = load_bedrockagentcorecontrol_resource(
        "policy_engine",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        POLICY_ENGINE_RESOURCE_PLURAL,
        pe_name,
        namespace="default",
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr)

    try:
        _, deleted = k8s.delete_custom_resource(ref, wait_periods=5, period_length=15)
        assert deleted
    except:
        pass


@service_marker
@pytest.mark.canary
class TestPolicyEngine:
    def test_create_delete(self, simple_policy_engine, bedrockagentcorecontrol_client):
        (ref, cr) = simple_policy_engine

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_engine_id = cr["status"]["id"]
        assert policy_engine_id is not None
        assert cr["status"]["ackResourceMetadata"]["arn"] is not None

        # Verify the policy engine exists in AWS
        aws_pe = bedrockagentcorecontrol_client.get_policy_engine(
            policyEngineId=policy_engine_id
        )
        assert aws_pe["policyEngineId"] == policy_engine_id
        assert aws_pe["name"] == cr["spec"]["name"]
        assert aws_pe["description"] == "ACK e2e test policy engine"

    def test_update_description(self, simple_policy_engine, bedrockagentcorecontrol_client):
        (ref, cr) = simple_policy_engine

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_engine_id = cr["status"]["id"]

        # Update description
        updates = {
            "spec": {
                "description": "Updated ACK e2e test policy engine description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        # Verify in AWS
        aws_pe = bedrockagentcorecontrol_client.get_policy_engine(
            policyEngineId=policy_engine_id
        )
        assert aws_pe["description"] == "Updated ACK e2e test policy engine description"

    def test_update_tags(self, simple_policy_engine, bedrockagentcorecontrol_client):
        (ref, cr) = simple_policy_engine

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_engine_arn = cr["status"]["ackResourceMetadata"]["arn"]

        # Add tags
        updates = {
            "spec": {
                "tags": {
                    "Environment": "test",
                    "Project": "ack-e2e",
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        # Verify tags in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=policy_engine_arn
        )["tags"]
        assert aws_tags["Environment"] == "test"
        assert aws_tags["Project"] == "ack-e2e"

        # Remove a tag
        cr = k8s.get_resource(ref)
        cr["spec"]["tags"] = {"Environment": "staging"}
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        # Verify tag removal in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=policy_engine_arn
        )["tags"]
        assert aws_tags["Environment"] == "staging"
        assert "Project" not in aws_tags
