#!/usr/bin/env bash

# This script prepares the environment for running the tests in the category specified by the TEST_CATEGORY environment variable.
# Echo / logging statements send output to standard error to separate that output from test result output
# (which sends output to standard output) which needs to be parsed.

set -e
shopt -s extglob

make crypto_setup_gopath

echo "test category (test-setup):" $TEST_CATEGORY>&2

case $TEST_CATEGORY in integration-ghost|integration-mvp|integration-network|integration-epochs|integration-access|integration-collection|integration-consensus|integration-execution|integration-verification)
  echo "running make docker-build-flow">&2
  make docker-build-flow
 ;;
  integration-bft)
    echo "running BFT make docker-build-flow-corrupted docker-build-flow">&2
    make docker-build-flow-corrupted docker-build-flow
 ;;
  unit)
    echo "generating mocks for unit tests">&2
    make install-mock-generators
    make generate-mocks
 ;;
  unit-crypto)
    echo "running crypto setup">&2
    make crypto_setup_tests
 ;;
  *)
    echo "nothing to setup">&2
  ;;
esac
