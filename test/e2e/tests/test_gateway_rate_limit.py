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

"""Integration tests for the GatewayRateLimit API."""

import time

import pytest
from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from botocore.exceptions import ClientError

from e2e import (
    CRD_GROUP,
    CRD_VERSION,
    load_bedrockagentcorecontrol_resource,
    service_marker,
)
from e2e.replacement_values import REPLACEMENT_VALUES

from .test_gateway import simple_gateway  # noqa: F401 -- registers the fixture

GATEWAY_RATE_LIMIT_RESOURCE_PLURAL = "gatewayratelimits"

SYNC_WAIT_PERIODS = 10
UPDATE_WAIT_AFTER_SECONDS = 10


def _wait_for_not_found(operation, resource_description, **kwargs):
    last_error = None
    for _ in range(SYNC_WAIT_PERIODS):
        try:
            operation(**kwargs)
        except ClientError as error:
            last_error = error
            if error.response["Error"]["Code"] == "ResourceNotFoundException":
                return
        time.sleep(10)
    pytest.fail(
        f"{resource_description} still exists in AWS after deletion; "
        f"last error: {last_error}"
    )


@pytest.fixture(scope="module")
def simple_gateway_rate_limit(request, bedrockagentcorecontrol_client):
    gateway_ref, _ = request.getfixturevalue("simple_gateway")
    assert k8s.wait_on_condition(
        gateway_ref,
        "ACK.ResourceSynced",
        "True",
        wait_periods=SYNC_WAIT_PERIODS,
    )
    gateway_cr = k8s.get_resource(gateway_ref)

    rate_limit_name = random_suffix_name("acktestratelimit", 40, delimiter="")

    replacements = REPLACEMENT_VALUES.copy()
    replacements["RATE_LIMIT_NAME"] = rate_limit_name
    # The rate limit ID is a customer-supplied create input. Reuse the CR name
    # so the test can assert the value round-trips rather than being replaced
    # by a service-generated one.
    replacements["RATE_LIMIT_ID"] = rate_limit_name
    replacements["GATEWAY_NAME"] = gateway_cr["metadata"]["name"]

    resource_data = load_bedrockagentcorecontrol_resource(
        "gateway_rate_limit",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        GATEWAY_RATE_LIMIT_RESOURCE_PLURAL,
        rate_limit_name,
        namespace="default",
    )

    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    yield (ref, cr, gateway_ref)

    rate_limit = k8s.get_resource(ref)
    rate_limit_id = rate_limit["spec"]["rateLimitID"]
    gateway_id = k8s.get_resource(gateway_ref)["status"]["gatewayID"]

    _, deleted = k8s.delete_custom_resource(ref, wait_periods=10, period_length=15)
    assert deleted, "GatewayRateLimit CR was not deleted"
    _wait_for_not_found(
        bedrockagentcorecontrol_client.get_gateway_rate_limit,
        f"GatewayRateLimit {rate_limit_id}",
        gatewayIdentifier=gateway_id,
        rateLimitId=rate_limit_id,
    )


@service_marker
@pytest.mark.canary
class TestGatewayRateLimit:
    def test_create_and_observe(
        self,
        simple_gateway_rate_limit,
        bedrockagentcorecontrol_client,
    ):
        ref, _, gateway_ref = simple_gateway_rate_limit

        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS
        )
        condition.assert_synced(ref)

        cr = k8s.get_resource(ref)
        assert cr["status"]["status"] == "ACTIVE"
        assert cr["status"]["createdAt"]
        assert cr["status"]["updatedAt"]

        # The Gateway reference resolved into the concrete gateway identifier.
        gateway_id = k8s.get_resource(gateway_ref)["status"]["gatewayID"]
        assert cr["spec"]["gatewayIdentifier"] == gateway_id

        aws_rate_limit = bedrockagentcorecontrol_client.get_gateway_rate_limit(
            gatewayIdentifier=gateway_id,
            rateLimitId=cr["spec"]["rateLimitID"],
        )
        assert aws_rate_limit["status"] == "ACTIVE"
        assert aws_rate_limit["dimensionKeys"] == ["toolName"]
        assert aws_rate_limit["description"] == cr["spec"]["description"]

        entry = aws_rate_limit["entries"][0]
        assert entry["dimensions"] == {"toolName": "*"}
        assert entry["requests"][0]["period"] == "minute"
        assert entry["requests"][0]["rate"] == 100

    def test_update_entries_and_description(
        self,
        simple_gateway_rate_limit,
        bedrockagentcorecontrol_client,
    ):
        ref, _, gateway_ref = simple_gateway_rate_limit

        cr = k8s.get_resource(ref)
        gateway_id = k8s.get_resource(gateway_ref)["status"]["gatewayID"]
        rate_limit_id = cr["spec"]["rateLimitID"]

        updates = {
            "spec": {
                "description": "Updated ACK e2e test gateway rate limit",
                "entries": [
                    {
                        "dimensions": {"toolName": "*"},
                        "requests": [{"period": "second", "rate": 25}],
                    }
                ],
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=SYNC_WAIT_PERIODS
        )

        aws_rate_limit = bedrockagentcorecontrol_client.get_gateway_rate_limit(
            gatewayIdentifier=gateway_id,
            rateLimitId=rate_limit_id,
        )
        assert (
            aws_rate_limit["description"]
            == "Updated ACK e2e test gateway rate limit"
        )
        entry = aws_rate_limit["entries"][0]
        assert entry["requests"][0]["period"] == "second"
        assert entry["requests"][0]["rate"] == 25
        # Dimension keys are immutable and must survive an entries update.
        assert aws_rate_limit["dimensionKeys"] == ["toolName"]

    def test_immutable_fields_rejected(self, simple_gateway_rate_limit):
        ref, _, _ = simple_gateway_rate_limit

        # dimensionKeys, gatewayIdentifier and rateLimitID carry CEL
        # immutability rules, so the API server rejects the patch outright and
        # the controller never sees a change it could not apply.
        for field, value in (
            ("dimensionKeys", ["targetName"]),
            ("rateLimitID", "some-other-id"),
        ):
            with pytest.raises(Exception, match="immutable"):
                k8s.patch_custom_resource(ref, {"spec": {field: value}})

    def test_list_contains_rate_limit(
        self,
        simple_gateway_rate_limit,
        bedrockagentcorecontrol_client,
    ):
        ref, _, gateway_ref = simple_gateway_rate_limit

        cr = k8s.get_resource(ref)
        gateway_id = k8s.get_resource(gateway_ref)["status"]["gatewayID"]

        listed = bedrockagentcorecontrol_client.list_gateway_rate_limits(
            gatewayIdentifier=gateway_id,
        )["rateLimits"]
        assert cr["spec"]["rateLimitID"] in [r["rateLimitId"] for r in listed]

    def test_invalid_dimension_key_is_terminal(self, simple_gateway):  # noqa: F811
        gateway_ref, _ = simple_gateway
        assert k8s.wait_on_condition(
            gateway_ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=SYNC_WAIT_PERIODS,
        )
        gateway_cr = k8s.get_resource(gateway_ref)

        rate_limit_name = random_suffix_name("acktestbadratelimit", 40, delimiter="")
        replacements = REPLACEMENT_VALUES.copy()
        replacements["RATE_LIMIT_NAME"] = rate_limit_name
        replacements["GATEWAY_NAME"] = gateway_cr["metadata"]["name"]

        resource_data = load_bedrockagentcorecontrol_resource(
            "gateway_rate_limit_invalid",
            additional_replacements=replacements,
        )

        ref = k8s.CustomResourceReference(
            CRD_GROUP,
            CRD_VERSION,
            GATEWAY_RATE_LIMIT_RESOURCE_PLURAL,
            rate_limit_name,
            namespace="default",
        )

        k8s.create_custom_resource(ref, resource_data)
        k8s.wait_resource_consumed_by_controller(ref)

        try:
            assert k8s.wait_on_condition(
                ref,
                condition.CONDITION_TYPE_TERMINAL,
                "True",
                wait_periods=SYNC_WAIT_PERIODS,
            )
            condition.assert_synced_status(ref, False)

            terminal = k8s.get_resource_condition(
                ref, condition.CONDITION_TYPE_TERMINAL
            )
            assert "ValidationException" in terminal["message"]
        finally:
            _, deleted = k8s.delete_custom_resource(
                ref, wait_periods=3, period_length=10
            )
            assert deleted, "GatewayRateLimit CR was not deleted"
