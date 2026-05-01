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
from e2e.bootstrap_resources import get_bootstrap_resources

from .test_gateway import simple_gateway

GATEWAY_TARGET_RESOURCE_PLURAL = "gatewaytargets"

API_KEY_CREDENTIAL_PROVIDER_PREFIX = "ack-test-apikey-"

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

OPENAPI_SCHEMA_PAYLOAD = """{
  "openapi": "3.0.0",
  "info": {
    "title": "Test Service",
    "version": "1.0.0",
    "description": "ACK e2e test OpenAPI schema"
  },
  "servers": [
    {
      "url": "https://api.example.com"
    }
  ],
  "paths": {
    "/hello": {
      "get": {
        "operationId": "SayHello",
        "summary": "Returns a greeting",
        "parameters": [
          {
            "name": "name",
            "in": "query",
            "required": true,
            "schema": { "type": "string" },
            "description": "Name to greet"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "message": { "type": "string" }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}"""

UPDATED_OPENAPI_SCHEMA_PAYLOAD = """{
  "openapi": "3.0.0",
  "info": {
    "title": "Test Service",
    "version": "1.0.0",
    "description": "ACK e2e test OpenAPI schema - updated"
  },
  "servers": [
    {
      "url": "https://api.example.com"
    }
  ],
  "paths": {
    "/hello": {
      "get": {
        "operationId": "SayHello",
        "summary": "Returns a greeting",
        "parameters": [
          {
            "name": "name",
            "in": "query",
            "required": true,
            "schema": { "type": "string" },
            "description": "Name to greet"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "message": { "type": "string" }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/goodbye": {
      "get": {
        "operationId": "SayGoodbye",
        "summary": "Returns a farewell",
        "parameters": [
          {
            "name": "name",
            "in": "query",
            "required": true,
            "schema": { "type": "string" },
            "description": "Name to bid farewell"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "message": { "type": "string" }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}"""

UPDATED_SMITHY_MODEL_PAYLOAD = """{
  "smithy": "2.0",
  "shapes": {
    "example.test#TestService": {
      "type": "service",
      "version": "1.0.0",
      "operations": [
        { "target": "example.test#SayHello" },
        { "target": "example.test#SayGoodbye" }
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
    },
    "example.test#SayGoodbye": {
      "type": "operation",
      "input": { "target": "example.test#SayGoodbyeInput" },
      "output": { "target": "example.test#SayGoodbyeOutput" },
      "traits": {
        "smithy.api#http": { "method": "GET", "uri": "/goodbye" },
        "smithy.api#documentation": "Returns a farewell"
      }
    },
    "example.test#SayGoodbyeInput": {
      "type": "structure",
      "members": {
        "name": {
          "target": "smithy.api#String",
          "traits": {
            "smithy.api#required": {},
            "smithy.api#httpQuery": "name",
            "smithy.api#documentation": "Name to bid farewell"
          }
        }
      }
    },
    "example.test#SayGoodbyeOutput": {
      "type": "structure",
      "members": {
        "message": {
          "target": "smithy.api#String",
          "traits": {
            "smithy.api#documentation": "Farewell message"
          }
        }
      }
    }
  }
}"""

LAMBDA_INPUT_SCHEMA = """{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name to greet"
    }
  },
  "required": ["name"]
}"""

LAMBDA_OUTPUT_SCHEMA = """{
  "type": "object",
  "properties": {
    "message": {
      "type": "string",
      "description": "Greeting message"
    }
  }
}"""

UPDATED_LAMBDA_INPUT_SCHEMA = """{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name to greet"
    },
    "language": {
      "type": "string",
      "description": "Language for the greeting"
    }
  },
  "required": ["name"]
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

        # Verify the description update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        assert aws_target["description"] == "Updated ACK e2e test gateway target description"

        # Update the Smithy model inline payload
        updates = {
            "spec": {
                "targetConfiguration": {
                    "mcp": {
                        "smithyModel": {
                            "inlinePayload": UPDATED_SMITHY_MODEL_PAYLOAD,
                        },
                    },
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the Smithy model update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        aws_smithy_model = aws_target["targetConfiguration"]["mcp"]["smithyModel"]["inlinePayload"]
        assert "SayGoodbye" in aws_smithy_model


@pytest.fixture(scope="module")
def api_key_credential_provider(bedrockagentcorecontrol_client):
    provider_name = random_suffix_name(API_KEY_CREDENTIAL_PROVIDER_PREFIX, 48, delimiter="")

    resp = bedrockagentcorecontrol_client.create_api_key_credential_provider(
        name=provider_name,
        apiKey="ack-e2e-test-dummy-key",
    )
    provider_arn = resp["credentialProviderArn"]

    yield provider_arn

    try:
        bedrockagentcorecontrol_client.delete_api_key_credential_provider(
            name=provider_name,
        )
    except Exception:
        pass


@pytest.fixture(scope="module")
def openapi_gateway_target(simple_gateway, api_key_credential_provider):
    (gateway_ref, gateway_cr) = simple_gateway

    # Wait for the parent gateway to be synced before creating a target
    assert k8s.wait_on_condition(gateway_ref, "ACK.ResourceSynced", "True", wait_periods=10)

    gateway_cr = k8s.get_resource(gateway_ref)
    gateway_id = gateway_cr["status"]["gatewayID"]

    target_name = random_suffix_name("acktestgwoapi", 32, delimiter="")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["GATEWAY_TARGET_NAME"] = target_name
    replacements["GATEWAY_ID"] = gateway_id
    replacements["OPENAPI_SCHEMA_PAYLOAD"] = OPENAPI_SCHEMA_PAYLOAD
    replacements["API_KEY_CREDENTIAL_PROVIDER_ARN"] = api_key_credential_provider

    resource_data = load_bedrockagentcorecontrol_resource(
        "gateway_target_openapi",
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
class TestGatewayTargetOpenAPI:
    def test_create_delete_openapi_gateway_target(self, openapi_gateway_target, bedrockagentcorecontrol_client):
        (ref, cr, gateway_id) = openapi_gateway_target

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

        # Verify the target was created with an openApiSchema configuration
        aws_openapi = aws_target["targetConfiguration"]["mcp"]["openApiSchema"]["inlinePayload"]
        assert "SayHello" in aws_openapi

    def test_update_openapi_gateway_target(self, openapi_gateway_target, bedrockagentcorecontrol_client):
        (ref, cr, gateway_id) = openapi_gateway_target

        cr = k8s.get_resource(ref)
        target_id = cr["status"]["targetID"]

        # Update the description
        updates = {
            "spec": {
                "description": "Updated ACK e2e test openapi gateway target description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the description update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        assert aws_target["description"] == "Updated ACK e2e test openapi gateway target description"

        # Update the OpenAPI schema inline payload
        updates = {
            "spec": {
                "targetConfiguration": {
                    "mcp": {
                        "openAPISchema": {
                            "inlinePayload": UPDATED_OPENAPI_SCHEMA_PAYLOAD,
                        },
                    },
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the OpenAPI schema update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        aws_openapi = aws_target["targetConfiguration"]["mcp"]["openApiSchema"]["inlinePayload"]
        assert "SayGoodbye" in aws_openapi


@pytest.fixture(scope="module")
def lambda_gateway_target(simple_gateway):
    (gateway_ref, gateway_cr) = simple_gateway

    # Wait for the parent gateway to be synced before creating a target
    assert k8s.wait_on_condition(gateway_ref, "ACK.ResourceSynced", "True", wait_periods=10)

    gateway_cr = k8s.get_resource(gateway_ref)
    gateway_id = gateway_cr["status"]["gatewayID"]

    resources = get_bootstrap_resources()
    lambda_function_arn = resources.GatewayTargetLambda.arn

    target_name = random_suffix_name("acktestgwlambda", 32, delimiter="")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["GATEWAY_TARGET_NAME"] = target_name
    replacements["GATEWAY_ID"] = gateway_id
    replacements["LAMBDA_FUNCTION_ARN"] = lambda_function_arn
    replacements["INPUT_SCHEMA"] = LAMBDA_INPUT_SCHEMA
    replacements["OUTPUT_SCHEMA"] = LAMBDA_OUTPUT_SCHEMA

    resource_data = load_bedrockagentcorecontrol_resource(
        "gateway_target_lambda",
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
class TestGatewayTargetLambda:
    def test_create_delete_lambda_gateway_target(self, lambda_gateway_target, bedrockagentcorecontrol_client):
        (ref, cr, gateway_id) = lambda_gateway_target

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

        # Verify the target was created with a Lambda configuration
        aws_lambda_config = aws_target["targetConfiguration"]["mcp"]["lambda"]
        assert aws_lambda_config["lambdaArn"] is not None
        tool_defs = aws_lambda_config["toolSchema"]["inlinePayload"]
        assert len(tool_defs) == 1
        assert tool_defs[0]["name"] == "SayHello"

    def test_update_lambda_gateway_target(self, lambda_gateway_target, bedrockagentcorecontrol_client):
        (ref, cr, gateway_id) = lambda_gateway_target

        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        synced = k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        if not synced:
            cr = k8s.get_resource(ref)
            print(cr)


        cr = k8s.get_resource(ref)
        target_id = cr["status"]["targetID"]

        # Update the description
        updates = {
            "spec": {
                "description": "Updated ACK e2e test lambda gateway target description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the description update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        assert aws_target["description"] == "Updated ACK e2e test lambda gateway target description"

        # Update the tool definition's input schema
        updates = {
            "spec": {
                "targetConfiguration": {
                    "mcp": {
                        "lambda": {
                            "toolSchema": {
                                "inlinePayload": [
                                    {
                                        "name": "SayHello",
                                        "description": "Returns a greeting message",
                                        "inputSchema": UPDATED_LAMBDA_INPUT_SCHEMA,
                                        "outputSchema": LAMBDA_OUTPUT_SCHEMA,
                                    },
                                ],
                            },
                        },
                    },
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the tool schema update in AWS
        aws_target = bedrockagentcorecontrol_client.get_gateway_target(
            gatewayIdentifier=gateway_id,
            targetId=target_id,
        )
        tool_defs = aws_target["targetConfiguration"]["mcp"]["lambda"]["toolSchema"]["inlinePayload"]
        assert len(tool_defs) == 1
        assert "language" in tool_defs[0]["inputSchema"]["properties"]
