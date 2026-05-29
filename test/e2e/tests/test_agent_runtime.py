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
import time

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from acktest.aws.identity import get_region, get_account_id
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

AGENT_RUNTIME_RESOURCE_PLURAL = "agentruntimes"
CREATE_WAIT_AFTER_SECONDS = 10
UPDATE_WAIT_AFTER_SECONDS = 10


def get_testing_container_uri(repository: str = "bedrock-agentcore-test", tag: str = "test") -> str:
    account_id = get_account_id()
    region = get_region()
    return f"{account_id}.dkr.ecr.{region}.amazonaws.com/{repository}:{tag}"

@pytest.fixture(scope="module")
def simple_agent_runtime():
    runtime_name = random_suffix_name("acktestagentruntime", 32, delimiter="")

    resources = get_bootstrap_resources()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["AGENT_RUNTIME_NAME"] = runtime_name
    replacements["ROLE_ARN"] = resources.AgentRuntimeRole.arn
    replacements["CONTAINER_URI"] = get_testing_container_uri()

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


@pytest.fixture(scope="module")
def agent_runtime_with_filesystem():
    runtime_name = random_suffix_name("acktestfsruntime", 32, delimiter="")

    resources = get_bootstrap_resources()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["AGENT_RUNTIME_NAME"] = runtime_name
    replacements["ROLE_ARN"] = resources.AgentRuntimeRole.arn
    replacements["CONTAINER_URI"] = get_testing_container_uri()
    replacements["MOUNT_PATH"] = "/mnt/session"

    resource_data = load_bedrockagentcorecontrol_resource(
        "agent_runtime_with_filesystem",
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

    def test_create_update_filesystem_configurations(self, agent_runtime_with_filesystem, bedrockagentcorecontrol_client):
        (ref, cr) = agent_runtime_with_filesystem

        time.sleep(CREATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        agent_runtime_id = cr["status"]["agentRuntimeID"]
        assert agent_runtime_id is not None

        # Verify filesystem configuration was applied at creation
        aws_runtime = bedrockagentcorecontrol_client.get_agent_runtime(
            agentRuntimeId=agent_runtime_id
        )
        fs_configs = aws_runtime.get("filesystemConfigurations", [])
        assert len(fs_configs) == 1
        assert fs_configs[0]["sessionStorage"]["mountPath"] == "/mnt/session"

        # Update the mount path
        cr = k8s.get_resource(ref)
        cr["spec"]["filesystemConfigurations"][0]["sessionStorage"]["mountPath"] = "/mnt/updated"
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        aws_runtime = bedrockagentcorecontrol_client.get_agent_runtime(
            agentRuntimeId=agent_runtime_id
        )
        fs_configs = aws_runtime.get("filesystemConfigurations", [])
        assert len(fs_configs) == 1
        assert fs_configs[0]["sessionStorage"]["mountPath"] == "/mnt/updated"