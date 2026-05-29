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

"""Integration tests for Memory API.
"""

import pytest
import time

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

MEMORY_RESOURCE_PLURAL = "memories"

UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="function")
def simple_memory():
    memory_name = random_suffix_name("acktestmem", 32, delimiter="")

    resources = get_bootstrap_resources()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["MEMORY_NAME"] = memory_name
    replacements["ROLE_ARN"] = resources.MemoryRole.arn

    resource_data = load_bedrockagentcorecontrol_resource(
        "memory",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        MEMORY_RESOURCE_PLURAL,
        memory_name,
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
class TestMemory:
    def test_create_delete_memory(self, simple_memory, bedrockagentcorecontrol_client):
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]
        assert memory_id is not None

        # Verify the memory exists in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        assert aws_memory["memory"]["id"] == memory_id
        assert aws_memory["memory"]["name"] == cr["spec"]["name"]

    def test_update_description(self, simple_memory, bedrockagentcorecontrol_client):
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        updates = {
            "spec": {
                "description": "Updated ACK e2e test memory description",
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        assert aws_memory["memory"]["description"] == "Updated ACK e2e test memory description"

    def test_update_add_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding a new strategy to an existing memory."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Add a summary strategy alongside the existing semantic strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "summaryMemoryStrategy": {
                            "name": "ack_test_summary",
                            "description": "ACK e2e test summary strategy",
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify strategies in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        strategy_names = {s["name"] for s in strategies}
        assert "ack_test_semantic" in strategy_names
        assert "ack_test_summary" in strategy_names

        # Verify status strategies are populated on the CR
        cr = k8s.get_resource(ref)
        status_strategies = cr["status"]["strategies"]
        assert len(status_strategies) == 2
        status_strategy_names = {s["name"] for s in status_strategies}
        assert "ack_test_semantic" in status_strategy_names
        assert "ack_test_summary" in status_strategy_names

    def test_update_modify_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test modifying an existing strategy's description."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Modify the semantic strategy's description
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "Updated semantic strategy description",
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the description was updated in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        semantic = next(s for s in strategies if s["name"] == "ack_test_semantic")
        assert semantic["description"] == "Updated semantic strategy description"

    def test_update_delete_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test removing a strategy from the memory."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # First add a second strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "summaryMemoryStrategy": {
                            "name": "ack_test_summary",
                            "description": "ACK e2e test summary strategy",
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Now remove the summary strategy by only including the semantic one
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify only the semantic strategy remains
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        strategy_names = {s["name"] for s in strategies}
        assert "ack_test_semantic" in strategy_names
        assert "ack_test_summary" not in strategy_names

    def test_update_add_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding a custom memory strategy with configuration."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Add a custom strategy with semantic override configuration
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack_test_custom",
                            "description": "ACK e2e test custom strategy",
                            "configuration": {
                                "semanticOverride": {
                                    "extraction": {
                                        "appendToPrompt": "Extract key facts only.",
                                        "modelID": "us.amazon.nova-lite-v1:0",
                                    },
                                },
                            },
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the custom strategy exists in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        custom = next(s for s in strategies if s["name"] == "ack_test_custom")
        assert custom["type"] == "CUSTOM"
        assert custom["description"] == "ACK e2e test custom strategy"

        # Verify the semantic override extraction configuration was persisted
        aws_config = custom["configuration"]
        extraction = aws_config["extraction"]["customExtractionConfiguration"]
        semantic_override = extraction["semanticExtractionOverride"]
        assert semantic_override["appendToPrompt"] == "Extract key facts only."
        assert semantic_override["modelId"] == "us.amazon.nova-lite-v1:0"

    def test_update_modify_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test modifying the configuration of a custom strategy."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # First add the custom strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack_test_custom",
                            "description": "ACK e2e test custom strategy",
                            "configuration": {
                                "semanticOverride": {
                                    "extraction": {
                                        "appendToPrompt": "Extract key facts only.",
                                        "modelID": "us.amazon.nova-lite-v1:0",
                                    },
                                },
                            },
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Now modify the custom strategy's description and add consolidation
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack_test_custom",
                            "description": "Updated custom strategy description",
                            "configuration": {
                                "semanticOverride": {
                                    "extraction": {
                                        "appendToPrompt": "Extract all relevant information.",
                                        "modelID": "us.amazon.nova-lite-v1:0",
                                    },
                                    "consolidation": {
                                        "appendToPrompt": "Consolidate into concise summaries.",
                                        "modelID": "us.amazon.nova-lite-v1:0",
                                    },
                                },
                            },
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the custom strategy was updated in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        custom = next(s for s in strategies if s["name"] == "ack_test_custom")
        assert custom["description"] == "Updated custom strategy description"

    def test_update_add_self_managed_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding a self-managed custom strategy with invocation configuration."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        resources = get_bootstrap_resources()
        topic_arn = resources.MemorySNSTopic.arn
        bucket_name = resources.MemoryS3Bucket.name

        # Add a self-managed custom strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack_test_self_managed",
                            "description": "ACK e2e test self-managed custom strategy",
                            "configuration": {
                                "selfManagedConfiguration": {
                                    "historicalContextWindowSize": 10,
                                    "invocationConfiguration": {
                                        "topicARN": topic_arn,
                                        "payloadDeliveryBucketName": bucket_name,
                                    },
                                    "triggerConditions": [
                                        {
                                            "messageBasedTrigger": {
                                                "messageCount": 5,
                                            },
                                        },
                                        {
                                            "tokenBasedTrigger": {
                                                "tokenCount": 3000,
                                            },
                                        },
                                        {
                                            "timeBasedTrigger": {
                                                "idleSessionTimeout": 60,
                                            },
                                        },
                                    ],
                                },
                            },
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the self-managed custom strategy in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        custom = next(s for s in strategies if s["name"] == "ack_test_self_managed")
        assert custom["type"] == "CUSTOM"
        assert custom["description"] == "ACK e2e test self-managed custom strategy"

        # Verify self-managed configuration details
        aws_config = custom["configuration"]["selfManagedConfiguration"]
        assert aws_config["historicalContextWindowSize"] == 10
        assert aws_config["invocationConfiguration"]["topicArn"] == topic_arn
        assert aws_config["invocationConfiguration"]["payloadDeliveryBucketName"] == bucket_name
        triggers = aws_config["triggerConditions"]
        assert len(triggers) == 3
        msg_trigger = next(t for t in triggers if "messageBasedTrigger" in t)
        assert msg_trigger["messageBasedTrigger"]["messageCount"] == 5
        token_trigger = next(t for t in triggers if "tokenBasedTrigger" in t)
        assert token_trigger["tokenBasedTrigger"]["tokenCount"] == 3000
        time_trigger = next(t for t in triggers if "timeBasedTrigger" in t)
        assert time_trigger["timeBasedTrigger"]["idleSessionTimeout"] == 60

    def test_update_modify_self_managed_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test modifying self-managed custom strategy configuration."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        resources = get_bootstrap_resources()
        topic_arn = resources.MemorySNSTopic.arn
        bucket_name = resources.MemoryS3Bucket.name

        # First add the self-managed custom strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack_test_self_managed",
                            "description": "ACK e2e test self-managed custom strategy",
                            "configuration": {
                                "selfManagedConfiguration": {
                                    "historicalContextWindowSize": 10,
                                    "invocationConfiguration": {
                                        "topicARN": topic_arn,
                                        "payloadDeliveryBucketName": bucket_name,
                                    },
                                    "triggerConditions": [
                                        {
                                            "messageBasedTrigger": {
                                                "messageCount": 5,
                                            },
                                        },
                                        {
                                            "tokenBasedTrigger": {
                                                "tokenCount": 3000,
                                            },
                                        },
                                        {
                                            "timeBasedTrigger": {
                                                "idleSessionTimeout": 60,
                                            },
                                        },
                                    ],
                                },
                            },
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Now modify: change context window and add a time-based trigger
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack_test_self_managed",
                            "description": "Updated self-managed strategy",
                            "configuration": {
                                "selfManagedConfiguration": {
                                    "historicalContextWindowSize": 20,
                                    "invocationConfiguration": {
                                        "topicARN": topic_arn,
                                        "payloadDeliveryBucketName": bucket_name,
                                    },
                                    "triggerConditions": [
                                        {
                                            "messageBasedTrigger": {
                                                "messageCount": 10,
                                            },
                                        },
                                        {
                                            "tokenBasedTrigger": {
                                                "tokenCount": 6000,
                                            },
                                        },
                                        {
                                            "timeBasedTrigger": {
                                                "idleSessionTimeout": 300,
                                            },
                                        },
                                    ],
                                },
                            },
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the update propagated to AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        custom = next(s for s in strategies if s["name"] == "ack_test_self_managed")
        assert custom["description"] == "Updated self-managed strategy"

        # Verify updated self-managed configuration in AWS
        aws_config = custom["configuration"]["selfManagedConfiguration"]
        assert aws_config["historicalContextWindowSize"] == 20
        assert aws_config["invocationConfiguration"]["topicArn"] == topic_arn
        assert aws_config["invocationConfiguration"]["payloadDeliveryBucketName"] == bucket_name
        triggers = aws_config["triggerConditions"]
        assert len(triggers) == 3
        msg_trigger = next(t for t in triggers if "messageBasedTrigger" in t)
        assert msg_trigger["messageBasedTrigger"]["messageCount"] == 10
        token_trigger = next(t for t in triggers if "tokenBasedTrigger" in t)
        assert token_trigger["tokenBasedTrigger"]["tokenCount"] == 6000
        time_trigger = next(t for t in triggers if "timeBasedTrigger" in t)
        assert time_trigger["timeBasedTrigger"]["idleSessionTimeout"] == 300

        # Verify the spec is reconciled correctly on read-back
        cr = k8s.get_resource(ref)
        spec_strategies = cr["spec"]["memoryStrategies"]
        self_managed = next(
            s for s in spec_strategies if s.get("customMemoryStrategy", {}).get("name") == "ack_test_self_managed"
        )
        sm_config = self_managed["customMemoryStrategy"]["configuration"]["selfManagedConfiguration"]
        assert sm_config["historicalContextWindowSize"] == 20
        assert sm_config["invocationConfiguration"]["topicARN"] == topic_arn
        assert len(sm_config["triggerConditions"]) == 3

    def test_update_combined_add_modify_delete(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding, modifying, and deleting strategies in a single update."""
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # First add a summary strategy so we have something to delete
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "summaryMemoryStrategy": {
                            "name": "ack_test_summary",
                            "description": "ACK e2e test summary strategy",
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Now in one update:
        # - Delete "ack_test_summary" (omit it)
        # - Modify "ack_test_semantic" (change description)
        # - Add "ack_test_user_pref" (new strategy)
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack_test_semantic",
                            "description": "Combined update semantic description",
                        },
                    },
                    {
                        "userPreferenceMemoryStrategy": {
                            "name": "ack_test_user_pref",
                            "description": "ACK e2e test user preference strategy",
                        },
                    },
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify the combined update in AWS
        aws_memory = bedrockagentcorecontrol_client.get_memory(
            memoryId=memory_id
        )
        strategies = aws_memory["memory"]["strategies"]
        strategy_names = {s["name"] for s in strategies}

        # Added
        assert "ack_test_user_pref" in strategy_names
        # Modified (still present)
        assert "ack_test_semantic" in strategy_names
        # Deleted
        assert "ack_test_summary" not in strategy_names

        # Verify the modification applied
        semantic = next(s for s in strategies if s["name"] == "ack_test_semantic")
        assert semantic["description"] == "Combined update semantic description"

        # Verify the new strategy has correct type
        user_pref = next(s for s in strategies if s["name"] == "ack_test_user_pref")
        assert user_pref["type"] == "USER_PREFERENCE"

    def test_update_tags(self, simple_memory, bedrockagentcorecontrol_client):
        (ref, cr) = simple_memory

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        memory_arn = cr["status"]["ackResourceMetadata"]["arn"]

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
            resourceArn=memory_arn
        )["tags"]
        assert aws_tags["Environment"] == "test"
        assert aws_tags["Project"] == "ack-e2e"

        # Remove a tag
        cr = k8s.get_resource(ref)
        cr["spec"]["tags"] = {"Environment": "staging"}
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify tag removal in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=memory_arn
        )["tags"]
        assert aws_tags["Environment"] == "staging"
        assert "Project" not in aws_tags
