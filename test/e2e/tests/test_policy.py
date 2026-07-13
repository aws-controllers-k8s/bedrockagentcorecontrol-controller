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

"""Integration tests for Policy API.
"""

import pytest
import time
import logging

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

POLICY_RESOURCE_PLURAL = "policies"
POLICY_ENGINE_RESOURCE_PLURAL = "policyengines"
UPDATE_WAIT_AFTER_SECONDS = 10
# Policy resources may take longer to provision than other resource types
SYNC_WAIT_PERIODS = 30


@pytest.fixture(scope="module")
def policy_engine_for_policy():
    """Create a PolicyEngine that will be used as a parent for Policy tests."""
    pe_name = random_suffix_name("acktest-pe-pol", 32, delimiter="-")
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

    assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

    cr = k8s.get_resource(ref)
    policy_engine_id = cr["status"]["id"]
    assert policy_engine_id is not None

    yield (ref, cr, policy_engine_id)

    try:
        _, deleted = k8s.delete_custom_resource(ref, wait_periods=5, period_length=15)
        assert deleted
    except:
        pass


@pytest.fixture(scope="module")
def simple_policy(policy_engine_for_policy):
    """Create a Policy resource within the PolicyEngine fixture."""
    (_, _, policy_engine_id) = policy_engine_for_policy

    pol_name = random_suffix_name("acktest-pol", 32, delimiter="-")
    pol_spec_name = pol_name.replace("-", "_")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["POLICY_NAME"] = pol_name
    replacements["POLICY_SPEC_NAME"] = pol_spec_name
    replacements["POLICY_ENGINE_ID"] = policy_engine_id

    resource_data = load_bedrockagentcorecontrol_resource(
        "policy",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        POLICY_RESOURCE_PLURAL,
        pol_name,
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
class TestPolicy:
    def test_create_delete(self, simple_policy, bedrockagentcorecontrol_client, policy_engine_for_policy):
        (ref, cr) = simple_policy
        (_, _, policy_engine_id) = policy_engine_for_policy

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_id = cr["status"]["id"]
        assert policy_id is not None
        assert cr["status"]["ackResourceMetadata"]["arn"] is not None

        # Verify the policy exists in AWS
        aws_pol = bedrockagentcorecontrol_client.get_policy(
            policyEngineId=policy_engine_id,
            policyId=policy_id,
        )
        assert aws_pol["policyId"] == policy_id
        assert aws_pol["name"] == cr["spec"]["name"]
        assert aws_pol["description"] == "ACK e2e test policy"
        assert aws_pol["enforcementMode"] == "LOG_ONLY"

    def test_update_description(self, simple_policy, bedrockagentcorecontrol_client, policy_engine_for_policy):
        (ref, cr) = simple_policy
        (_, _, policy_engine_id) = policy_engine_for_policy

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_id = cr["status"]["id"]

        # Update description
        updates = {
            "spec": {
                "description": "Updated ACK e2e test policy description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        # Verify in AWS
        aws_pol = bedrockagentcorecontrol_client.get_policy(
            policyEngineId=policy_engine_id,
            policyId=policy_id,
        )
        assert aws_pol["description"] == "Updated ACK e2e test policy description"

    def test_update_enforcement_mode(self, simple_policy, bedrockagentcorecontrol_client, policy_engine_for_policy):
        (ref, cr) = simple_policy
        (_, _, policy_engine_id) = policy_engine_for_policy

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_id = cr["status"]["id"]

        # Confirm the policy starts in LOG_ONLY (the value set at creation) in AWS
        aws_pol = bedrockagentcorecontrol_client.get_policy(
            policyEngineId=policy_engine_id,
            policyId=policy_id,
        )
        assert aws_pol["enforcementMode"] == "LOG_ONLY"

        # Update enforcement mode from LOG_ONLY to ACTIVE
        updates = {
            "spec": {
                "enforcementMode": "ACTIVE",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        # Verify in AWS
        aws_pol = bedrockagentcorecontrol_client.get_policy(
            policyEngineId=policy_engine_id,
            policyId=policy_id,
        )
        assert aws_pol["enforcementMode"] == "ACTIVE"

    def test_update_definition(self, simple_policy, bedrockagentcorecontrol_client, policy_engine_for_policy):
        (ref, cr) = simple_policy
        (_, _, policy_engine_id) = policy_engine_for_policy

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        cr = k8s.get_resource(ref)
        policy_id = cr["status"]["id"]

        # Update Cedar policy definition
        updates = {
            "spec": {
                "definition": {
                    "cedar": {
                        "statement": 'forbid(principal, action, resource is AgentCore::Gateway);',
                    },
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS)

        # Verify in AWS
        aws_pol = bedrockagentcorecontrol_client.get_policy(
            policyEngineId=policy_engine_id,
            policyId=policy_id,
        )
        assert aws_pol["definition"]["cedar"]["statement"] == 'forbid(principal, action, resource is AgentCore::Gateway);'
