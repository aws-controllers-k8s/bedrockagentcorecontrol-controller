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

"""Integration tests for Agent Runtime Endpoint API.
"""

import pytest
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

from .test_agent_runtime import simple_agent_runtime

AGENT_RUNTIME_ENDPOINT_RESOURCE_PLURAL = "agentruntimeendpoints"


@pytest.fixture(scope="module")
def simple_agent_runtime_endpoint(simple_agent_runtime):
    (runtime_ref, runtime_cr) = simple_agent_runtime
    
    # Wait for runtime to be synced
    k8s.wait_on_condition(runtime_ref, "ACK.ResourceSynced", "True", wait_periods=10)
    
    # Get the latest CR to ensure we have the agentRuntimeID in status
    runtime_cr = k8s.get_resource(runtime_ref)
    agent_runtime_id = runtime_cr["status"]["agentRuntimeID"]
    endpoint_name = random_suffix_name("acktestendpoint", 32, delimiter="")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["AGENT_RUNTIME_ID"] = agent_runtime_id
    replacements["ENDPOINT_NAME"] = endpoint_name

    resource_data = load_bedrockagentcorecontrol_resource(
        "agent_runtime_endpoint",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        AGENT_RUNTIME_ENDPOINT_RESOURCE_PLURAL,
        endpoint_name,
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
class TestAgentRuntimeEndpoint:
    def test_create_delete_agent_runtime_endpoint(self, simple_agent_runtime_endpoint, bedrockagentcorecontrol_client):
        (ref, cr) = simple_agent_runtime_endpoint

        k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        agent_runtime_endpoint_arn = cr["status"]["ackResourceMetadata"]["arn"]
        assert agent_runtime_endpoint_arn is not None

        agent_runtime_id = cr["spec"]["agentRuntimeID"]
        endpoint_name = cr["spec"]["name"]

        aws_endpoint = bedrockagentcorecontrol_client.get_agent_runtime_endpoint(
            agentRuntimeId=agent_runtime_id,
            endpointName=endpoint_name
        )
        assert aws_endpoint["agentRuntimeEndpointArn"] is not None