#!/usr/bin/env bash

set -o errexit
set -o nounset

__usage() {
  cat <<EOF
USAGE:

GOTESTSUM="" TEST_TAG="" ${0}

With:
    GOTESTSUM   Path to go-test-sum or "go run" command.
    TEST_TAG    Tag to target the test, i.e.: "unit", "integration", "functional", or "e2e".

EOF
  exit 1
}

trap __usage EXIT

${GOTESTSUM} --junitfile ".ignore.test-${TEST_TAG}.xml" -- -tags "${TEST_TAG}" -race ./... -count=1 -short -cover -coverprofile ".ignore.test-${TEST_TAG}-coverage.out" ./...

trap 'echo "✅ ${TEST_TAG} tests ran successfully"' EXIT
