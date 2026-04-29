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

"""Integration tests for GatewayTarget API.
"""

import pytest
import time

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

from .test_gateway import simple_gateway

GATEWAY_TARGET_RESOURCE_PLURAL = "gatewaytargets"

CREATE_WAIT_AFTER_SECONDS = 10
UPDATE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_AFTER_SECONDS = 10

SMITHY_MODEL_PAYLOAD = """{
  "smithy": "2.0",
  "shapes": {
    "example.test#TestService": {
      "type": "service",
      "version": "1.0.0",
      "operations": [
        { "target": "example.test#SayHello" }
      ],
      "traits": {
        "aws.auth#sigv4": { "name": "execute-api" },
        "aws.protocols#restJson1": {}
      }
    },
    "example.test#SayHello": {
      "type": "operation",
      "input": { "target": "example.test#SayHelloInput" },
      "output": { "target": "example.test#SayHelloOutput" },
      "traits": {
        "smithy.api#http": { "method": "GET", "uri": "/hello" },
        "smithy.api#documentation": "Returns a greeting"
      }
    },
    "example.test#SayHelloInput": {
      "type": "structure",
      "members": {
        "name": {
          "target": "smithy.api#String",
          "traits": {
            "smithy.api#required": {},
            "smithy.api#httpQuery": "name",
            "smithy.api#documentation": "Name to greet"
          }
        }
      }
    },
    "example.test#SayHelloOutput": {
      "type": "structure",
      "members": {
        "message": {
          "target": "smithy.api#String",
          "traits": {
            "smithy.api#documentation": "Greeting message"
          }
        }
      }
    }
  }
}"""


@pytest.fixture(scope="module")
def simple_gateway_target(simple_gateway):
    (gateway_ref, gateway_cr) = simple_gateway

    # Wait for the parent gateway to be synced before creating a target
    assert k8s.wait_on_condition(gateway_ref, "ACK.ResourceSynced", "True", wait_periods=10)

    gateway_cr = k8s.get_resource(gateway_ref)
    gateway_id = gateway_cr["status"]["gatewayID"]

    target_name = random_suffix_name("acktestgwtarget", 32, delimiter="")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["GATEWAY_TARGET_NAME"] = target_name
    replacements["GATEWAY_ID"] = gateway_id
    replacements["SMITHY_MODEL_PAYLOAD"] = SMITHY_MODEL_PAYLOAD

    resource_data = load_bedrockagentcorecontrol_resource(
        "gateway_target",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        GATEWAY_TARGET_RESOURCE_PLURAL,
        target_name,
        namespace="default",
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr, gateway_id)

    try:
        _, deleted = k8s.delete_custom_resource(ref, wait_periods=3, period_length=10)
        assert deleted
    except:
        pass


@service_marker
@pytest.mark.canary
class TestGatewayTarget:
    def test_create_delete_gateway_target(self, simple_gateway_target, bedrockagentcorecontrol_client):
        (ref, cr, gateway_id) = simple_gateway_target

        time.sleep(CREATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        target_id = cr["status"]["targetID"]
        assert target_id is not None

        # Verify the gateway target exists in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        assert aws_target["targetId"] == target_id
        assert aws_target["name"] == cr["spec"]["name"]

    def test_update_gateway_target(self, simple_gateway_target, bedrockagentcorecontrol_client):
        (ref, cr, gateway_id) = simple_gateway_target

        cr = k8s.get_resource(ref)
        target_id = cr["status"]["targetID"]

        # Update the description
        updates = {
            "spec": {
                "description": "Updated ACK e2e test gateway target description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        assert aws_target["description"] == "Updated ACK e2e test gateway target description"