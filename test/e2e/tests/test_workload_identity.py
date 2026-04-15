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

"""Integration tests for WorkloadIdentity API.
"""

import pytest
import time

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

WORKLOAD_IDENTITY_RESOURCE_PLURAL = "workloadidentities"
UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_workload_identity():
    identity_name = random_suffix_name("acktest-wi", 32, delimiter="-")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["WORKLOAD_IDENTITY_NAME"] = identity_name

    resource_data = load_bedrockagentcorecontrol_resource(
        "workload_identity",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        WORKLOAD_IDENTITY_RESOURCE_PLURAL,
        identity_name,
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
class TestWorkloadIdentity:
    def test_create_delete(self, simple_workload_identity, bedrockagentcorecontrol_client):
        (ref, cr) = simple_workload_identity

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        cr = k8s.get_resource(ref)
        identity_name = cr["spec"]["name"]
        assert cr["status"]["ackResourceMetadata"]["arn"] is not None

        # Verify the workload identity exists in AWS
        aws_identity = bedrockagentcorecontrol_client.get_workload_identity(
            name=identity_name
        )
        assert aws_identity["name"] == identity_name

    def test_update_oauth2_urls(self, simple_workload_identity, bedrockagentcorecontrol_client):
        (ref, cr) = simple_workload_identity

        cr = k8s.get_resource(ref)
        identity_name = cr["spec"]["name"]

        # Add OAuth2 return URLs
        updates = {
            "spec": {
                "allowedResourceOauth2ReturnURLs": [
                    "https://example.com/callback",
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        # Verify in AWS
        aws_identity = bedrockagentcorecontrol_client.get_workload_identity(
            name=identity_name
        )
        assert "https://example.com/callback" in aws_identity.get("allowedResourceOauth2ReturnUrls", [])

    def test_update_tags(self, simple_workload_identity, bedrockagentcorecontrol_client):
        (ref, cr) = simple_workload_identity

        cr = k8s.get_resource(ref)
        identity_arn = cr["status"]["ackResourceMetadata"]["arn"]

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
            resourceArn=identity_arn
        )["tags"]
        assert aws_tags["Environment"] == "test"
        assert aws_tags["Project"] == "ack-e2e"

        # Remove a tag
        cr = k8s.get_resource(ref)
        cr["spec"]["tags"] = {"Environment": "staging"}
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        # Verify tag removal in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=identity_arn
        )["tags"]
        assert aws_tags["Environment"] == "staging"
        assert "Project" not in aws_tags
