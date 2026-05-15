#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ORCH_FILE="$SCRIPT_DIR/orch.yml"
TMP_DIR="$(mktemp -d)"
ORCH_BIN="$TMP_DIR/orch"

ENV_ID="${ORCH_CLOUDFORMATION_SMOKE_ENV:-cf-smoke-one}"
REGION="${ORCH_CLOUDFORMATION_SMOKE_REGION:-us-east-1}"
COMPONENT="cf"
STACK_NAME="orch-$ENV_ID-$COMPONENT"
PARAMETER_NAME="/orch/smoke/$STACK_NAME"
STATE_FILE="$REPO_ROOT/.orch/$ENV_ID/state.json"

export AWS_REGION="$REGION"
export AWS_DEFAULT_REGION="$REGION"

cleanup() {
  set +e
  "$ORCH_BIN" down --file "$ORCH_FILE" --env-id "$ENV_ID" >/dev/null 2>&1
  aws cloudformation delete-stack --stack-name "$STACK_NAME" --region "$REGION" >/dev/null 2>&1
  aws cloudformation wait stack-delete-complete --stack-name "$STACK_NAME" --region "$REGION" >/dev/null 2>&1
  rm -rf "$REPO_ROOT/.orch/$ENV_ID"
  rm -rf "$SCRIPT_DIR/.workdir/orch/$ENV_ID"
  rmdir "$SCRIPT_DIR/.workdir/orch" "$SCRIPT_DIR/.workdir" >/dev/null 2>&1
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if [[ "${ORCH_RUN_AWS_SMOKE:-}" != "1" ]]; then
  echo "Skipping CloudFormation smoke test."
  echo "Set ORCH_RUN_AWS_SMOKE=1 to create and delete a real AWS CloudFormation stack."
  exit 0
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required for the CloudFormation smoke test" >&2
  exit 1
fi

if ! aws sts get-caller-identity --region "$REGION" >/dev/null; then
  echo "AWS ambient auth is required for the CloudFormation smoke test" >&2
  exit 1
fi

cd "$REPO_ROOT"

echo "Building orch CLI..."
go build -o "$ORCH_BIN" ./cmd/orch

echo "Starting $ENV_ID in $REGION..."
"$ORCH_BIN" up --file "$ORCH_FILE" --env-id "$ENV_ID"

if [[ ! -f "$STATE_FILE" ]]; then
  echo "Expected state file $STATE_FILE to exist" >&2
  exit 1
fi

if ! aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" >/dev/null; then
  echo "Expected CloudFormation stack $STACK_NAME to exist" >&2
  exit 1
fi

if [[ "$(aws ssm get-parameter --name "$PARAMETER_NAME" --region "$REGION" --query 'Parameter.Value' --output text)" != "orch cloudformation smoke" ]]; then
  echo "Expected SSM parameter $PARAMETER_NAME to be created by the stack" >&2
  exit 1
fi

echo "Tearing down $ENV_ID..."
"$ORCH_BIN" down --file "$ORCH_FILE" --env-id "$ENV_ID"

if aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" >/dev/null 2>&1; then
  echo "Expected CloudFormation stack $STACK_NAME to be deleted" >&2
  exit 1
fi

if aws ssm get-parameter --name "$PARAMETER_NAME" --region "$REGION" >/dev/null 2>&1; then
  echo "Expected SSM parameter $PARAMETER_NAME to be deleted" >&2
  exit 1
fi

if ! grep -q '"status": "destroyed"' "$STATE_FILE"; then
  echo "Expected Orch component state to be marked destroyed" >&2
  exit 1
fi

echo "CloudFormation smoke test passed"
