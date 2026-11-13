#!/bin/bash
# Runs the e2e tests (test/e2e.yml, test/e2e-link.yml) against an AWS emulator.
# https://github.com/aws/aws-emulator
#
# Start the emulator with `aws-emulator --auth off` before running this.
# This requires the AWS CLI and runn (https://github.com/k1LoW/runn).
set -euo pipefail

cd "$(dirname "$0")/../.."

# Never run against real AWS: always use the emulator endpoint with dummy
# credentials.
export AWS_ENDPOINT_URL="${AWS_ENDPOINT_URL:-https://127.0.0.1:9297}"
export AWS_CA_BUNDLE="${AWS_CA_BUNDLE:-$HOME/.aws/emulator/9297/ca.pem}"
export AWS_REGION=ap-northeast-1
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
export AWS_PAGER=""
unset AWS_PROFILE AWS_DEFAULT_PROFILE AWS_SESSION_TOKEN

export MIRAGE_E2E_PORT="${MIRAGE_E2E_PORT:-18080}"
export MIRAGE_E2E_ENDPOINT="http://127.0.0.1:$MIRAGE_E2E_PORT"

MIRAGE_E2E_TMP="$(mktemp -d)"
export MIRAGE_E2E_TMP
cleanup() {
  status=$?
  ./test/e2e/mirage.sh stop || true
  if [[ $status -ne 0 && -f "$MIRAGE_E2E_TMP/mirage-ecs.log" ]]; then
    echo "--- mirage-ecs.log" >&2
    cat "$MIRAGE_E2E_TMP/mirage-ecs.log" >&2
  fi
  rm -rf "$MIRAGE_E2E_TMP"
  exit $status
}
trap cleanup EXIT

MIRAGE_E2E_ZONE_ID="$(./test/e2e/setup.sh "$MIRAGE_E2E_TMP/config.yaml" | tail -n 1)"
export MIRAGE_E2E_ZONE_ID

./test/e2e/mirage.sh start

for runbook in test/e2e.yml test/e2e-link.yml; do
  runn run "$runbook" --scopes run:exec --verbose < /dev/null
done
