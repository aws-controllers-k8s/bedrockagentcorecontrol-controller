# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

import os

import boto3
import pytest

from acktest import k8s

def pytest_addoption(parser):
    parser.addoption("--runslow", action="store_true", default=False, help="run slow tests")

def pytest_configure(config):
    config.addinivalue_line(
        "markers", "service(arg): mark test associated with a given service"
    )
    config.addinivalue_line(
        "markers", "slow: mark test as slow to run"
    )

def pytest_collection_modifyitems(config, items):
    if config.getoption("--runslow"):
        return
    skip_slow = pytest.mark.skip(reason="need --runslow option to run")
    for item in items:
        if "slow" in item.keywords:
            item.add_marker(skip_slow)

# Provide a k8s client to interact with the integration test cluster
@pytest.fixture(scope='class')
def k8s_client():
    return k8s._get_k8s_api_client()

@pytest.fixture(scope='module')
def bedrockagentcorecontrol_client():
    return boto3.client('bedrock-agentcore-control')

@pytest.fixture(scope='module')
def bedrockagentcore_client():
    return boto3.client('bedrock-agentcore')

@pytest.fixture(scope='session')
def expected_aws_account_id():
    expected = os.getenv("ACK_E2E_EXPECTED_AWS_ACCOUNT_ID")
    if not expected:
        pytest.fail(
            "ACK_E2E_EXPECTED_AWS_ACCOUNT_ID is required before Harness tests "
            "can create AWS resources"
        )
    identity = boto3.client("sts").get_caller_identity()
    if identity["Account"] != expected:
        pytest.fail(
            f"refusing AWS mutation: caller account {identity['Account']} is "
            f"not expected account {expected}"
        )
    return expected
