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

"""Integration tests for BrowserProfile API.
"""

import pytest
import time

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from acktest import tags as acktags
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

BROWSER_PROFILE_RESOURCE_PLURAL = "browserprofiles"
UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_browser_profile():
    bp_name = random_suffix_name("acktest-bp", 32, delimiter="-")
    # AWS API requires name to match [a-zA-Z][a-zA-Z0-9_]{0,47}
    # Replace hyphens with underscores for the spec.name field
    bp_spec_name = bp_name.replace("-", "_")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["BROWSER_PROFILE_K8S_NAME"] = bp_name
    replacements["BROWSER_PROFILE_NAME"] = bp_spec_name

    resource_data = load_bedrockagentcorecontrol_resource(
        "browser_profile",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        BROWSER_PROFILE_RESOURCE_PLURAL,
        bp_name,
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
class TestBrowserProfile:
    def test_create_delete(self, simple_browser_profile, bedrockagentcorecontrol_client):
        (ref, cr) = simple_browser_profile

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=10)

        cr = k8s.get_resource(ref)
        profile_id = cr["status"]["id"]
        assert profile_id is not None
        assert cr["status"]["ackResourceMetadata"]["arn"] is not None
        assert cr["status"]["status"] == "READY"

        # Verify the browser profile exists in AWS
        aws_bp = bedrockagentcorecontrol_client.get_browser_profile(
            profileId=profile_id
        )
        assert aws_bp["name"] == cr["spec"]["name"]
        assert aws_bp["status"] == "READY"

    def test_update_tags(self, simple_browser_profile, bedrockagentcorecontrol_client):
        (ref, cr) = simple_browser_profile

        cr = k8s.get_resource(ref)
        bp_arn = cr["status"]["ackResourceMetadata"]["arn"]

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

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        # Verify tags in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=bp_arn
        )["tags"]
        acktags.assert_equal_without_ack_tags(
            expected={
                "Environment": "test",
                "Project": "ack-e2e",
            },
            actual=aws_tags,
        )
        acktags.assert_ack_system_tags(aws_tags)

        # Remove a tag
        cr = k8s.get_resource(ref)
        cr["spec"]["tags"] = {"Environment": "staging"}
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        # Verify tag removal in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=bp_arn
        )["tags"]
        acktags.assert_equal_without_ack_tags(
            expected={
                "Environment": "staging",
            },
            actual=aws_tags,
        )
        acktags.assert_ack_system_tags(aws_tags)
