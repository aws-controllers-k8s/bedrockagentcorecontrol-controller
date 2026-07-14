# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Bootstraps the resources required to run the Bedrock Agent Core Control integration tests.
"""

import logging
from acktest.bootstrapping import Resources, BootstrapFailureException
from acktest.bootstrapping.iam import Role
from acktest.bootstrapping.cognito_identity import UserPool
from acktest.bootstrapping.function import Function
from acktest.bootstrapping.sns import Topic
from acktest.bootstrapping.s3 import Bucket
from acktest.aws.identity import get_region, get_account_id
from e2e import bootstrap_directory
from e2e.bootstrap_resources import BootstrapResources



def service_bootstrap() -> Resources:
    logging.getLogger().setLevel(logging.INFO)
    aws_region = get_region()
    account_id = get_account_id()
    lambda_code_uri = f"{account_id}.dkr.ecr.{aws_region}.amazonaws.com/ack-e2e-testing-bedrockagentcorecontrol-controller:v1"
    
    resources = BootstrapResources(
        AgentRuntimeRole=Role(
            name_prefix="ack-bedrock-agentruntime",
            principal_service="bedrock-agentcore.amazonaws.com",
            description="IAM role for ACK Bedrock AgentRuntime e2e tests",
            managed_policies=[
                "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryFullAccess",
                "arn:aws:iam::aws:policy/BedrockAgentCoreFullAccess",
            ],
        ),
        GatewayRole=Role(
            name_prefix="ack-bedrock-gateway",
            principal_service="bedrock-agentcore.amazonaws.com",
            description="IAM role for ACK Bedrock Gateway e2e tests",
            managed_policies=[
                "arn:aws:iam::aws:policy/service-role/AWSLambdaRole",
                # Lets the gateway invoke managed connectors (e.g. web search)
                # for the connector target e2e test.
                "arn:aws:iam::aws:policy/BedrockAgentCoreFullAccess",
            ],
        ),
        GatewayUserPool=UserPool(
            name_prefix="ack-bedrock-gateway",
        ),
        GatewayTargetLambda=Function(
            name_prefix="ack-bedrock-gw-target",
            code_uri=lambda_code_uri,
            service="bedrock-agentcore",
            description="Lambda function for ACK Bedrock Gateway Target e2e tests",
        ),
        MemoryRole=Role(
            name_prefix="ack-bedrock-memory",
            principal_service="bedrock-agentcore.amazonaws.com",
            description="IAM role for ACK Bedrock Memory e2e tests",
            managed_policies=[
                "arn:aws:iam::aws:policy/AmazonBedrockFullAccess",
                "arn:aws:iam::aws:policy/AmazonSNSFullAccess",
                "arn:aws:iam::aws:policy/AmazonS3FullAccess",
            ],
        ),
        MemorySNSTopic=Topic(
            name_prefix="ack-bedrock-memory",
        ),
        MemoryS3Bucket=Bucket(
            name_prefix="ack-bedrock-memory",
        ),
    )
    
    try:
        resources.bootstrap()
    except BootstrapFailureException as ex:
        exit(254)
    
    return resources


if __name__ == "__main__":
    config = service_bootstrap()
    # Write config to current directory by default
    config.serialize(bootstrap_directory)