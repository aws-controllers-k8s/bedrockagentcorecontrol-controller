"""
Sample agent container for the AgentRuntime ACK sample.

Uses the bedrock-agentcore SDK which handles the HTTP protocol contract
(GET /ping, POST /invocations) automatically via the @app.entrypoint decorator.

Environment variables (injected via AgentRuntime.spec.environmentVariables):
  AWS_REGION         -- e.g. us-west-2
  BEDROCK_MODEL_ID   -- e.g. us.anthropic.claude-haiku-4-5-20251001-v1:0
"""

import json
import logging
import os
import traceback

import boto3
from bedrock_agentcore.runtime import BedrockAgentCoreApp

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = BedrockAgentCoreApp()

AWS_REGION = os.environ.get("AWS_REGION", "us-west-2")
BEDROCK_MODEL_ID = os.environ.get(
    "BEDROCK_MODEL_ID", "us.anthropic.claude-haiku-4-5-20251001-v1:0"
)

_bedrock_client = None

def _get_bedrock_client():
    """Lazy-init the Bedrock client so the container can start even if credentials aren't ready yet."""
    global _bedrock_client
    if _bedrock_client is None:
        _bedrock_client = boto3.client("bedrock-runtime", region_name=AWS_REGION)
    return _bedrock_client

@app.entrypoint
def invoke(request):
    """Receive a prompt, call Anthropic Claude, return the answer. Replace this with a real agent."""
    logger.info("invoke called with request: %s", request)
    try:
        prompt = request.get("prompt", request.get("input", ""))

        if not prompt:
            return {"response": "Error: 'prompt' must be a non-empty string", "status": "error"}

        payload = {
            "anthropic_version": "bedrock-2023-05-31",
            "max_tokens": 1024,
            "messages": [
                {"role": "user", "content": prompt},
            ],
        }

        logger.info("Calling Bedrock model %s in %s", BEDROCK_MODEL_ID, AWS_REGION)
        response = _get_bedrock_client().invoke_model(
            modelId=BEDROCK_MODEL_ID,
            body=json.dumps(payload),
            contentType="application/json",
            accept="application/json",
        )
        body = json.loads(response["body"].read())
        answer = body["content"][0]["text"]
        logger.info("Got answer of length %d", len(answer))

        return answer
    except Exception as e:
        logger.error("Error in invoke: %s\n%s", e, traceback.format_exc())
        return {"response": f"Error: {e}", "status": "error"}

if __name__ == "__main__":
    app.run()