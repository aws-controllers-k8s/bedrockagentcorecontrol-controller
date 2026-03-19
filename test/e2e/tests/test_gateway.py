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

"""Integration tests for Gateway API.
"""

import pytest
import time

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

GATEWAY_RESOURCE_PLURAL = "gateways"

UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_gateway():
    gateway_name = random_suffix_name("acktestgw", 32, delimiter="")

    resources = get_bootstrap_resources()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["GATEWAY_NAME"] = gateway_name
    replacements["ROLE_ARN"] = resources.GatewayRole.arn
    replacements["DISCOVERY_URL"] = (
        f"https://cognito-idp.{resources.GatewayUserPool.region}.amazonaws.com"
        f"/{resources.GatewayUserPool.user_pool_id}/.well-known/openid-configuration"
    )
    replacements["CLIENT_ID"] = "test-client-id"

    resource_data = load_bedrockagentcorecontrol_resource(
        "gateway",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        GATEWAY_RESOURCE_PLURAL,
        gateway_name,
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
class TestGateway:
    def test_create_delete_gateway(self, simple_gateway, bedrockagentcorecontrol_client):
        (ref, cr) = simple_gateway

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        gateway_id = cr["status"]["gatewayID"]
        assert gateway_id is not None

        # Verify the gateway exists in AWS
        aws_gateway = bedrockagentcorecontrol_client.get_gateway(
            gatewayIdentifier=gateway_id
        )
        assert aws_gateway["gatewayId"] == gateway_id
        assert aws_gateway["name"] == cr["spec"]["name"]
        assert aws_gateway["protocolType"] == "MCP"

    def test_update_gateway(self, simple_gateway, bedrockagentcorecontrol_client):
        (ref, cr) = simple_gateway

        cr = k8s.get_resource(ref)
        gateway_id = cr["status"]["gatewayID"]

        # Update the description
        updates = {
            "spec": {
                "description": "Updated ACK e2e test gateway description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the update in AWS
        aws_gateway = bedrockagentcorecontrol_client.get_gateway(
            gatewayIdentifier=gateway_id
        )
        assert aws_gateway["description"] == "Updated ACK e2e test gateway description"
        assert aws_gateway["description"] == "Updated ACK e2e test gateway description"

    def test_update_tags(self, simple_gateway, bedrockagentcorecontrol_client):
        (ref, cr) = simple_gateway

        cr = k8s.get_resource(ref)
        gateway_id = cr["status"]["gatewayID"]
        gateway_arn = cr["status"]["ackResourceMetadata"]["arn"]

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

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify tags in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=gateway_arn
        )["tags"]
        assert aws_tags["Environment"] == "test"
        assert aws_tags["Project"] == "ack-e2e"

        # Update one tag, remove another
        updates = {
            "spec": {
                "tags": {
                    "Environment": "staging",
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify updated tags in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=gateway_arn
        )["tags"]
        assert aws_tags["Environment"] == "staging"
        assert "Project" not in aws_tags