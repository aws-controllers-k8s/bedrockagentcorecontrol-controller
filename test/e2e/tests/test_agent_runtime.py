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

"""Integration tests for Agent Runtime API.
"""

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

AGENT_RUNTIME_RESOURCE_PLURAL = "agentruntimes"


@pytest.fixture(scope="module")
def simple_agent_runtime():
    runtime_name = random_suffix_name("acktestagentruntime", 32, delimiter="")

    resources = get_bootstrap_resources()
    
    replacements = REPLACEMENT_VALUES.copy()
    replacements["AGENT_RUNTIME_NAME"] = runtime_name
    replacements["ROLE_ARN"] = "arn:aws:iam::951637587566:role/ack-bedrock-test-role"
    replacements["CONTAINER_URI"] = "951637587566.dkr.ecr.us-west-2.amazonaws.com/bedrock-agentcore-test:test"

    resource_data = load_bedrockagentcorecontrol_resource(
        "agent_runtime",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        AGENT_RUNTIME_RESOURCE_PLURAL,
        runtime_name,
        namespace="default",
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr)

    try:
        _, deleted = k8s.delete_custom_resource(ref, wait_periods=3, period_length=10)
        assert deleted
    except:
        pass


@service_marker
@pytest.mark.canary
class TestAgentRuntime:
    def test_create_delete_agent_runtime(self, simple_agent_runtime, bedrockagentcorecontrol_client):
        (ref, cr) = simple_agent_runtime

        k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        agent_runtime_id = cr["status"]["agentRuntimeID"]
        assert agent_runtime_id is not None

        aws_runtime = bedrockagentcorecontrol_client.get_agent_runtime(
            agentRuntimeId=agent_runtime_id
        )
        assert aws_runtime["agentRuntimeId"] == agent_runtime_id