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

"""Integration tests for Browser API.
"""

import pytest
import time

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

BROWSER_RESOURCE_PLURAL = "browsers"

UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_browser():
    browser_name = random_suffix_name("acktestbrowser", 32, delimiter="")

    resources = get_bootstrap_resources()

    replacements = REPLACEMENT_VALUES.copy()
    replacements["BROWSER_NAME"] = browser_name
    replacements["ROLE_ARN"] = resources.AgentRuntimeRole.arn

    resource_data = load_bedrockagentcorecontrol_resource(
        "browser",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        BROWSER_RESOURCE_PLURAL,
        browser_name,
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
class TestBrowser:
    def test_create_delete_browser(self, simple_browser, bedrockagentcorecontrol_client):
        (ref, cr) = simple_browser

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=20)

        cr = k8s.get_resource(ref)
        browser_id = cr["status"]["id"]
        assert browser_id is not None

        # Verify the browser exists in AWS
        aws_browser = bedrockagentcorecontrol_client.get_browser(
            browserId=browser_id
        )
        assert aws_browser["browserId"] == browser_id
        assert aws_browser["name"] == cr["spec"]["name"]

    def test_update_tags(self, simple_browser, bedrockagentcorecontrol_client):
        (ref, cr) = simple_browser

        cr = k8s.get_resource(ref)
        browser_arn = cr["status"]["ackResourceMetadata"]["arn"]

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
            resourceArn=browser_arn
        )["tags"]
        assert aws_tags["Environment"] == "test"
        assert aws_tags["Project"] == "ack-e2e"

        # Remove a tag by replacing the full CR with updated tags
        cr = k8s.get_resource(ref)
        cr["spec"]["tags"] = {"Environment": "staging"}
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        # Verify updated tags in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=browser_arn
        )["tags"]
        assert aws_tags["Environment"] == "staging"
        assert "Project" not in aws_tags
