# AgentRuntime

An `AgentRuntime` represents a containerized agent registered with Amazon Bedrock AgentCore Runtime. The controller creates the runtime in AWS and reconciles its state back to the cluster.

## Prerequisites

See the [shared prerequisites](../README.md#prerequisites).

## Build and Push the Agent Image

A sample "agent" is provided in the [`agent/`](agent/) directory. It uses the `bedrock-agentcore` SDK and calls Amazon Bedrock Claude to answer prompts.

```bash
export AWS_ACCOUNT_ID=123456789012
export AWS_REGION=us-west-2
export IMAGE_TAG=latest

# Create the ECR repository (if it doesn't exist)
aws ecr create-repository --repository-name my-agent --region $AWS_REGION || true

# Authenticate Docker to ECR
aws ecr get-login-password --region $AWS_REGION | \
  docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

# Build and push
docker build -t my-agent:$IMAGE_TAG agent/
docker tag my-agent:$IMAGE_TAG $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/my-agent:$IMAGE_TAG
docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/my-agent:$IMAGE_TAG
```

## Usage

1. Copy `my-agent-runtime.yaml` and replace all `<PLACEHOLDER>` values:

   | Placeholder | Description |
   |---|---|
   | `<AGENT_RUNTIME_NAME>` | A unique name for the runtime (alphanumeric + underscores, max 48 chars) |
   | `<AGENT_ROLE_ARN>` | IAM role ARN (e.g. `arn:aws:iam::123456789012:role/my-agent-role`) |
   | `<ACCOUNT_ID>` | Your AWS account ID |
   | `<REGION>` | AWS region (e.g. `us-east-1`) |
   | `<IMAGE_TAG>` | Container image tag |
   | `<BEDROCK_MODEL_ID>` | Bedrock model ID to pass to your agent (e.g. `anthropic.claude-sonnet-4-20250514-v1:0`) |

2. Apply the manifest:

   ```bash
   kubectl apply -f my-agent-runtime.yaml
   ```

3. Check the runtime status:

   ```bash
   kubectl get agentruntime my-agent-runtime
   kubectl describe agentruntime my-agent-runtime
   ```

   Wait until `status.status` shows `Ready`.

4. Note the `status.agentRuntimeID` -- you will need it to create an endpoint.

## Cleanup

```bash
kubectl delete agentruntime my-agent-runtime
```