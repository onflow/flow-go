#!/usr/bin/env bash

set -e
shopt -s extglob

# make crypto_setup_gopath

echo "test category (test-setup):" $TEST_CATEGORY

case $TEST_CATEGORY in
 integration-ghost|integration-mvp|integration-network|integration-epochs|integration-access|integration-collection|integration-consensus|integration-execution|integration-verification)
#    make docker-build-flow
 ;;
 integration-bft)
#    make docker-build-flow-corrupted docker-build-flow
 ;;
 unit)
#    make install-mock-generators
#    make generate-mocks
 ;;
 unit-crypto)
#    make crypto_setup_tests
 ;;
esac

