# AgentRuntimeEndpoint

An `AgentRuntimeEndpoint` creates a publicly accessible endpoint for an existing `AgentRuntime`, allowing clients to invoke the agent.

## Prerequisites

- See the [shared prerequisites](../README.md#prerequisites).
- An `AgentRuntime` must already exist and be in `Ready` state. See [`../agent_runtime/`](../agent_runtime/).

## Usage

1. Get the runtime ID from your existing AgentRuntime:

   ```bash
   kubectl describe agentruntime my-agent-runtime
   ```

   Look for `status.agentRuntimeID` in the output.

2. Copy `my-agent-runtime-endpoint.yaml` and replace all `<PLACEHOLDER>` values:

   | Placeholder | Description |
   |---|---|
   | `<AGENT_RUNTIME_ID>` | The `agentRuntimeID` from the AgentRuntime status |
   | `<ENDPOINT_NAME>` | A unique name for the endpoint (alphanumeric + underscores, max 48 chars) |

3. Apply the manifest:

   ```bash
   kubectl apply -f my-agent-runtime-endpoint.yaml
   ```

4. Check the endpoint status:

   ```bash
   kubectl get agentruntimeendpoint my-agent-runtime-endpoint
   kubectl describe agentruntimeendpoint my-agent-runtime-endpoint
   ```

   Wait until `status.status` shows `Ready`.

## Cleanup

```bash
kubectl delete agentruntimeendpoint my-agent-runtime-endpoint
```
