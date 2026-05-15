#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ORCH_FILE="$SCRIPT_DIR/orch.yml"
TMP_DIR="$(mktemp -d)"
ORCH_BIN="$TMP_DIR/orch"

ENV_ID="${ORCH_TERRAFORM_SMOKE_ENV:-tf-smoke-one}"
COMPONENT="tf"
WORK_DIR="$SCRIPT_DIR/.workdir/orch/$ENV_ID/$COMPONENT"
STATE_FILE="$REPO_ROOT/.orch/$ENV_ID/state.json"

cleanup() {
  set +e
  "$ORCH_BIN" down --file "$ORCH_FILE" --env-id "$ENV_ID" >/dev/null 2>&1
  rm -rf "$REPO_ROOT/.orch/$ENV_ID"
  rm -rf "$SCRIPT_DIR/.workdir/orch/$ENV_ID"
  rmdir "$SCRIPT_DIR/.workdir/orch" "$SCRIPT_DIR/.workdir" >/dev/null 2>&1
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cd "$REPO_ROOT"

echo "Building orch CLI..."
go build -o "$ORCH_BIN" ./cmd/orch

echo "Starting $ENV_ID..."
"$ORCH_BIN" up --file "$ORCH_FILE" --env-id "$ENV_ID"

if [[ ! -f "$STATE_FILE" ]]; then
  echo "Expected state file $STATE_FILE to exist" >&2
  exit 1
fi

if ! terraform -chdir="$WORK_DIR" state list | grep -q '^terraform_data.smoke$'; then
  echo "Expected terraform_data.smoke in Terraform state" >&2
  exit 1
fi

echo "Tearing down $ENV_ID..."
"$ORCH_BIN" down --file "$ORCH_FILE" --env-id "$ENV_ID"

if terraform -chdir="$WORK_DIR" state list | grep -q '^terraform_data.smoke$'; then
  echo "Expected terraform_data.smoke to be destroyed" >&2
  exit 1
fi

if ! grep -q '"status": "destroyed"' "$STATE_FILE"; then
  echo "Expected Orch component state to be marked destroyed" >&2
  exit 1
fi

echo "Terraform smoke test passed"
