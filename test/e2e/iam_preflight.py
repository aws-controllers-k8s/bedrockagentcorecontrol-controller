#!/usr/bin/env python3

"""Fail-closed IAM and account preflight for the Harness lifecycle test."""

import argparse
import sys

import boto3


CONTROLLER_ACTIONS = [
    "bedrock-agentcore:CreateHarness",
    "bedrock-agentcore:GetHarness",
    "bedrock-agentcore:UpdateHarness",
    "bedrock-agentcore:DeleteHarness",
    "bedrock-agentcore:CreateHarnessEndpoint",
    "bedrock-agentcore:GetHarnessEndpoint",
    "bedrock-agentcore:UpdateHarnessEndpoint",
    "bedrock-agentcore:DeleteHarnessEndpoint",
    "bedrock-agentcore:CreateAgentRuntime",
    "bedrock-agentcore:GetAgentRuntime",
    "bedrock-agentcore:UpdateAgentRuntime",
    "bedrock-agentcore:DeleteAgentRuntime",
    "bedrock-agentcore:CreateAgentRuntimeEndpoint",
    "bedrock-agentcore:GetAgentRuntimeEndpoint",
    "bedrock-agentcore:UpdateAgentRuntimeEndpoint",
    "bedrock-agentcore:DeleteAgentRuntimeEndpoint",
    "bedrock-agentcore:CreateMemory",
    "bedrock-agentcore:GetMemory",
    "bedrock-agentcore:UpdateMemory",
    "bedrock-agentcore:DeleteMemory",
    "bedrock-agentcore:ListTagsForResource",
    "bedrock-agentcore:TagResource",
    "bedrock-agentcore:UntagResource",
]

INVOKER_ACTIONS = [
    "bedrock-agentcore:InvokeHarness",
    "bedrock-agentcore:InvokeAgentRuntime",
]


def _assert_allowed(iam, role_arn, actions, resource_arns, context_entries=None):
    request = dict(
        PolicySourceArn=role_arn,
        ActionNames=actions,
        ResourceArns=resource_arns,
    )
    if context_entries:
        request["ContextEntries"] = context_entries
    response = iam.simulate_principal_policy(**request)
    decisions = {
        result["EvalActionName"]: result["EvalDecision"]
        for result in response["EvaluationResults"]
    }
    denied = {
        action: decisions.get(action, "missing")
        for action in actions
        if decisions.get(action) != "allowed"
    }
    if denied:
        details = ", ".join(
            f"{action}={decision}" for action, decision in sorted(denied.items())
        )
        raise RuntimeError(f"IAM preflight denied required actions: {details}")


def _assert_execution_role_trust(iam, execution_role_arn):
    role_name = execution_role_arn.rsplit("/", 1)[-1]
    policy = iam.get_role(RoleName=role_name)["Role"]["AssumeRolePolicyDocument"]
    statements = policy.get("Statement", [])
    if isinstance(statements, dict):
        statements = [statements]
    for statement in statements:
        principal = statement.get("Principal", {}).get("Service", [])
        if isinstance(principal, str):
            principal = [principal]
        action = statement.get("Action", [])
        if isinstance(action, str):
            action = [action]
        if (
            statement.get("Effect") == "Allow"
            and "bedrock-agentcore.amazonaws.com" in principal
            and "sts:AssumeRole" in action
        ):
            return
    raise RuntimeError(
        "Harness execution role does not trust "
        "bedrock-agentcore.amazonaws.com for sts:AssumeRole"
    )


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--profile", required=True)
    parser.add_argument("--region", required=True)
    parser.add_argument("--expected-account-id", required=True)
    parser.add_argument("--controller-role-arn", required=True)
    parser.add_argument("--invoker-role-arn", required=True)
    parser.add_argument("--execution-role-arn", required=True)
    return parser.parse_args()


def main():
    args = parse_args()
    session = boto3.Session(profile_name=args.profile, region_name=args.region)
    identity = session.client("sts").get_caller_identity()
    if identity["Account"] != args.expected_account_id:
        raise RuntimeError(
            f"refusing AWS mutation: account {identity['Account']} is not "
            f"expected dev account {args.expected_account_id}"
        )

    iam = session.client("iam")
    region_context = [
        {
            "ContextKeyName": "aws:RequestedRegion",
            "ContextKeyValues": [args.region],
            "ContextKeyType": "string",
        }
    ]
    _assert_allowed(
        iam,
        args.controller_role_arn,
        CONTROLLER_ACTIONS,
        ["*"],
        region_context,
    )
    _assert_allowed(
        iam,
        args.controller_role_arn,
        ["iam:PassRole"],
        [args.execution_role_arn],
        region_context
        + [
            {
                "ContextKeyName": "iam:PassedToService",
                "ContextKeyValues": ["bedrock-agentcore.amazonaws.com"],
                "ContextKeyType": "string",
            }
        ],
    )
    _assert_allowed(
        iam,
        args.invoker_role_arn,
        INVOKER_ACTIONS,
        ["*"],
        region_context,
    )
    _assert_execution_role_trust(iam, args.execution_role_arn)
    print(f"IAM preflight passed for account {identity['Account']} ({identity['Arn']})")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"IAM preflight failed: {error}", file=sys.stderr)
        raise SystemExit(1) from error
