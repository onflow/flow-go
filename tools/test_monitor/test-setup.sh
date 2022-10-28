#!/usr/bin/env bash

# This script prepares the environment for running the tests in the category specified by the TEST_CATEGORY environment variable.
# Echo / logging statements send output to standard error to separate that output from test result output
# (which sends output to standard output) which needs to be parsed.

set -e
shopt -s extglob

echo "test category (test-setup):" $TEST_CATEGORY>&2

case $TEST_CATEGORY in integration-bft|integration-ghost|integration-mvp|integration-network|integration-epochs|integration-access|integration-collection|integration-consensus|integration-execution|integration-verification)
  echo "running make crypto_setup_gopath">&2
  make crypto_setup_gopath
  echo "running make docker-build-flow">&2
  make docker-build-flow
 ;;
  *)
    echo "nothing to setup">&2
  ;;
esac
