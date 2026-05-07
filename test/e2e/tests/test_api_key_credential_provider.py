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

"""Integration tests for ApiKeyCredentialProvider API.
"""

import pytest
import time

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from acktest import tags as acktags
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_bedrockagentcorecontrol_resource
from e2e.replacement_values import REPLACEMENT_VALUES

API_KEY_CREDENTIAL_PROVIDER_RESOURCE_PLURAL = "apikeycredentialproviders"
CREATE_WAIT_AFTER_SECONDS = 10
UPDATE_WAIT_AFTER_SECONDS = 10


@pytest.fixture(scope="module")
def simple_api_key_credential_provider():
    provider_name = random_suffix_name("acktest-akcp", 32, delimiter="-")
    secret_name = f"{provider_name}-secret"

    k8s.create_opaque_secret(
        namespace="default",
        name=secret_name,
        key="api-key",
        value="test-api-key-value",
    )

    replacements = REPLACEMENT_VALUES.copy()
    replacements["API_KEY_CREDENTIAL_PROVIDER_NAME"] = provider_name
    replacements["API_KEY_SECRET_NAME"] = secret_name

    resource_data = load_bedrockagentcorecontrol_resource(
        "api_key_credential_provider",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        API_KEY_CREDENTIAL_PROVIDER_RESOURCE_PLURAL,
        provider_name,
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
    finally:
        try:
            k8s.delete_secret(namespace="default", name=secret_name)
        except:
            pass


@pytest.fixture(scope="module")
def api_key_credential_provider_with_tags():
    provider_name = random_suffix_name("acktest-akcp-tags", 32, delimiter="-")
    secret_name = f"{provider_name}-secret"

    k8s.create_opaque_secret(
        namespace="default",
        name=secret_name,
        key="api-key",
        value="test-api-key-value",
    )

    replacements = REPLACEMENT_VALUES.copy()
    replacements["API_KEY_CREDENTIAL_PROVIDER_NAME"] = provider_name
    replacements["API_KEY_SECRET_NAME"] = secret_name

    resource_data = load_bedrockagentcorecontrol_resource(
        "api_key_credential_provider_with_tags",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        API_KEY_CREDENTIAL_PROVIDER_RESOURCE_PLURAL,
        provider_name,
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
    finally:
        try:
            k8s.delete_secret(namespace="default", name=secret_name)
        except:
            pass


@service_marker
@pytest.mark.canary
class TestApiKeyCredentialProvider:
    def test_create(self, simple_api_key_credential_provider, bedrockagentcorecontrol_client):
        (ref, cr) = simple_api_key_credential_provider

        time.sleep(CREATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        cr = k8s.get_resource(ref)
        provider_name = cr["spec"]["name"]

        # Verify the ARN is set in ACKResourceMetadata
        assert cr["status"]["ackResourceMetadata"]["arn"] is not None
        arn = cr["status"]["ackResourceMetadata"]["arn"]
        assert "apikeycredentialprovider" in arn

        # Verify the resource exists in AWS
        aws_provider = bedrockagentcorecontrol_client.get_api_key_credential_provider(
            name=provider_name
        )
        assert aws_provider["name"] == provider_name
        assert aws_provider["credentialProviderArn"] == arn

    def test_create_with_tags_then_update(self, api_key_credential_provider_with_tags, bedrockagentcorecontrol_client):
        (ref, cr) = api_key_credential_provider_with_tags

        time.sleep(CREATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        cr = k8s.get_resource(ref)
        provider_arn = cr["status"]["ackResourceMetadata"]["arn"]

        # Step 1: Verify initial tags were applied at creation
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=provider_arn
        )["tags"]

        acktags.assert_equal_without_ack_tags(
            expected={
                "Environment": "development",
                "Team": "ack-test",
                "ToBeRemoved": "temporary",
            },
            actual=aws_tags,
        )
        acktags.assert_ack_system_tags(aws_tags)

        # Step 2: Update tags in a single operation that:
        #   - Updates an existing tag: Environment development -> production
        #   - Adds a new tag: Project=ack-e2e
        #   - Removes an existing tag: ToBeRemoved (omitted from new set)
        #   - Keeps an existing tag unchanged: Team=ack-test
        cr = k8s.get_resource(ref)
        cr["spec"]["tags"] = {
            "Environment": "production",
            "Team": "ack-test",
            "Project": "ack-e2e",
        }
        k8s.replace_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        # Verify the final tag state in AWS
        aws_tags = bedrockagentcorecontrol_client.list_tags_for_resource(
            resourceArn=provider_arn
        )["tags"]

        acktags.assert_equal_without_ack_tags(
            expected={
                "Environment": "production",
                "Team": "ack-test",
                "Project": "ack-e2e",
            },
            actual=aws_tags,
        )

        # Verify removed tag is gone
        assert "ToBeRemoved" not in aws_tags

        # Verify ACK system tags are still present
        acktags.assert_ack_system_tags(aws_tags)
