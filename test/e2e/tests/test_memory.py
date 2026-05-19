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

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

MEMORY_RESOURCE_PLURAL = "memories"

UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
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
        assert aws_memory["memory"]["memoryId"] == memory_id
        assert aws_memory["memory"]["name"] == cr["spec"]["name"]

    def test_update_description(self, simple_memory, bedrockagentcorecontrol_client):
        (ref, cr) = simple_memory

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

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Add an episodic strategy alongside the existing semantic strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "ACK e2e test semantic strategy",
                        },
                    },
                    {
                        "summaryMemoryStrategy": {
                            "name": "ack-test-summary",
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
        assert "ack-test-semantic" in strategy_names
        assert "ack-test-summary" in strategy_names

        # Verify status strategies are populated on the CR
        cr = k8s.get_resource(ref)
        status_strategies = cr["status"]["strategies"]
        assert len(status_strategies) == 2
        status_strategy_names = {s["name"] for s in status_strategies}
        assert "ack-test-semantic" in status_strategy_names
        assert "ack-test-summary" in status_strategy_names

    def test_update_modify_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test modifying an existing strategy's description."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Modify the semantic strategy's description, keep the summary strategy
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Updated semantic strategy description",
                        },
                    },
                    {
                        "summaryMemoryStrategy": {
                            "name": "ack-test-summary",
                            "description": "ACK e2e test summary strategy",
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
        semantic = next(s for s in strategies if s["name"] == "ack-test-semantic")
        assert semantic["description"] == "Updated semantic strategy description"

    def test_update_delete_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test removing a strategy from the memory."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Remove the summary strategy by only including the semantic one
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Updated semantic strategy description",
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
        assert "ack-test-semantic" in strategy_names
        assert "ack-test-summary" not in strategy_names

    def test_update_add_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding a custom memory strategy with configuration."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Add a custom strategy with semantic override configuration
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Updated semantic strategy description",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack-test-custom",
                            "description": "ACK e2e test custom strategy",
                            "configuration": {
                                "semanticOverride": {
                                    "extraction": {
                                        "appendToPrompt": "Extract key facts only.",
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
        custom = next(s for s in strategies if s["name"] == "ack-test-custom")
        assert custom["type"] == "CUSTOM"
        assert custom["description"] == "ACK e2e test custom strategy"

    def test_update_modify_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test modifying the configuration of a custom strategy."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # Update the custom strategy's description and configuration
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Updated semantic strategy description",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack-test-custom",
                            "description": "Updated custom strategy description",
                            "configuration": {
                                "semanticOverride": {
                                    "extraction": {
                                        "appendToPrompt": "Extract all relevant information.",
                                    },
                                    "consolidation": {
                                        "appendToPrompt": "Consolidate into concise summaries.",
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
        custom = next(s for s in strategies if s["name"] == "ack-test-custom")
        assert custom["description"] == "Updated custom strategy description"

    def test_update_add_self_managed_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding a self-managed custom strategy with invocation configuration."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        resources = get_bootstrap_resources()
        topic_arn = resources.MemorySNSTopic.arn
        bucket_name = resources.MemoryS3Bucket.name

        # Replace the current strategies with semantic + self-managed custom
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Updated custom strategy description",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack-test-self-managed",
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
        custom = next(s for s in strategies if s["name"] == "ack-test-self-managed")
        assert custom["type"] == "CUSTOM"
        assert custom["description"] == "ACK e2e test self-managed custom strategy"

        # Verify self-managed configuration details
        aws_config = custom["configuration"]["selfManagedConfiguration"]
        assert aws_config["historicalContextWindowSize"] == 10
        assert aws_config["invocationConfiguration"]["topicArn"] == topic_arn
        assert aws_config["invocationConfiguration"]["payloadDeliveryBucketName"] == bucket_name
        triggers = aws_config["triggerConditions"]
        assert len(triggers) == 1
        assert triggers[0]["messageBasedTrigger"]["messageCount"] == 5

    def test_update_modify_self_managed_custom_strategy(self, simple_memory, bedrockagentcorecontrol_client):
        """Test modifying self-managed custom strategy configuration."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        resources = get_bootstrap_resources()
        topic_arn = resources.MemorySNSTopic.arn
        bucket_name = resources.MemoryS3Bucket.name

        # Update the self-managed strategy: change context window and add a time-based trigger
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Updated custom strategy description",
                        },
                    },
                    {
                        "customMemoryStrategy": {
                            "name": "ack-test-self-managed",
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
        custom = next(s for s in strategies if s["name"] == "ack-test-self-managed")
        assert custom["description"] == "Updated self-managed strategy"

        # Verify updated self-managed configuration in AWS
        aws_config = custom["configuration"]["selfManagedConfiguration"]
        assert aws_config["historicalContextWindowSize"] == 20
        assert aws_config["invocationConfiguration"]["topicArn"] == topic_arn
        assert aws_config["invocationConfiguration"]["payloadDeliveryBucketName"] == bucket_name
        triggers = aws_config["triggerConditions"]
        assert len(triggers) == 2
        trigger_types = {list(t.keys())[0] for t in triggers}
        assert "messageBasedTrigger" in trigger_types
        assert "timeBasedTrigger" in trigger_types
        msg_trigger = next(t for t in triggers if "messageBasedTrigger" in t)
        assert msg_trigger["messageBasedTrigger"]["messageCount"] == 10
        time_trigger = next(t for t in triggers if "timeBasedTrigger" in t)
        assert time_trigger["timeBasedTrigger"]["idleSessionTimeout"] == 300

        # Verify the spec is reconciled correctly on read-back
        cr = k8s.get_resource(ref)
        spec_strategies = cr["spec"]["memoryStrategies"]
        self_managed = next(
            s for s in spec_strategies if s.get("customMemoryStrategy", {}).get("name") == "ack-test-self-managed"
        )
        sm_config = self_managed["customMemoryStrategy"]["configuration"]["selfManagedConfiguration"]
        assert sm_config["historicalContextWindowSize"] == 20
        assert sm_config["invocationConfiguration"]["topicARN"] == topic_arn
        assert len(sm_config["triggerConditions"]) == 2

    def test_update_combined_add_modify_delete(self, simple_memory, bedrockagentcorecontrol_client):
        """Test adding, modifying, and deleting strategies in a single update."""
        (ref, cr) = simple_memory

        cr = k8s.get_resource(ref)
        memory_id = cr["status"]["id"]

        # In one update:
        # - Delete "ack-test-self-managed" (omit it)
        # - Modify "ack-test-semantic" (change description)
        # - Add "ack-test-episodic" (new strategy)
        updates = {
            "spec": {
                "memoryStrategies": [
                    {
                        "semanticMemoryStrategy": {
                            "name": "ack-test-semantic",
                            "description": "Combined update semantic description",
                        },
                    },
                    {
                        "episodicMemoryStrategy": {
                            "name": "ack-test-episodic",
                            "description": "ACK e2e test episodic strategy",
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
        assert "ack-test-episodic" in strategy_names
        # Modified (still present)
        assert "ack-test-semantic" in strategy_names
        # Deleted
        assert "ack-test-self-managed" not in strategy_names

        # Verify the modification applied
        semantic = next(s for s in strategies if s["name"] == "ack-test-semantic")
        assert semantic["description"] == "Combined update semantic description"

        # Verify the new strategy has correct type
        episodic = next(s for s in strategies if s["name"] == "ack-test-episodic")
        assert episodic["type"] == "EPISODIC"

    def test_update_tags(self, simple_memory, bedrockagentcorecontrol_client):
        (ref, cr) = simple_memory

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
