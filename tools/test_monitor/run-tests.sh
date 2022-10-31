#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable.
# Echo / logging statements send output to standard error to separate that output from test result output
# (which sends output to standard output) which needs to be parsed.

set -e
shopt -s extglob

echo "test category (run-tests):" $TEST_CATEGORY>&2

# run tests and process results

if [[ $TEST_CATEGORY =~ integration-(bft|ghost|mvp|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
  echo "killing and removing orphaned containers from previous run">&2
    # kill and remove orphaned containers from previous run
    containers=$(docker ps -a -q)

    if [ ! -z "$containers" ]
    then
        docker rm -f $containers > /dev/null
    fi

  echo "running $TEST_CATEGORY tests">&2
  make crypto_setup_gopath
  make docker-build-flow
  make -C integration -s ${BASH_REMATCH[1]}-tests > test-output
else
    case $TEST_CATEGORY in
        unit)
          make install-tools
          make verify-mocks
          echo "running unit tests (ci)">&2
          make -s unittest-main > test-output
        ;;
        unit-crypto)
          make -C crypto -s setup
          echo "running crypto unit tests">&2
          make -C crypto -s test_main > test-output
        ;;
        unit-insecure)
          make install-tools
          echo "running insecure unit tests">&2
          make -C insecure -s test > test-output
        ;;
        unit-integration)
          make install-tools
          echo "running integration unit tests">&2
          make -C integration -s test > test-output
        ;;
        *)
          echo "unrecognized test category (run-tests):" $TEST_CATEGORY>&2
          exit 1
        ;;
    esac
fi

