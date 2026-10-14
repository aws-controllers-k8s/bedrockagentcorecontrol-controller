# ACK Bedrock AgentCore Control Samples

This directory contains sample YAML manifests for the Bedrock AgentCore Control ACK controller.

| Resource | Directory |
|---|---|
| AgentRuntime | [`agent_runtime/`](agent_runtime/) |
| AgentRuntimeEndpoint | [`agent_runtime_endpoint/`](agent_runtime_endpoint/) |

## Prerequisites

1. **Install the ACK controller** -- follow the [ACK installation guide](https://aws-controllers-k8s.github.io/community/docs/user-docs/install/) to deploy the `bedrockagentcorecontrol-controller` into your cluster.

2. **Create an IAM execution role** for your agent runtime. The role must:
   - Have a trust policy allowing `agentcore.bedrock.amazonaws.com` to assume it.
   - Have the `arn:aws:iam::aws:policy/AmazonBedrockAgentCoreFullAccess` managed policy attached (or equivalent permissions).
   - Allow `iam:PassRole` so the service can assume the role on your behalf.

3. **Push a container image** for your agent to Amazon ECR (or another OCI registry accessible from Bedrock AgentCore). The image must expose an HTTP server on port 8080. A sample agent application is included in [`agent_runtime/agent/`](agent_runtime/agent/) — see the agent_runtime README for build instructions.

## Getting Started

Start with [`agent_runtime/`](agent_runtime/) to create an AgentRuntime, then proceed to [`agent_runtime_endpoint/`](agent_runtime_endpoint/) to expose it via an endpoint.