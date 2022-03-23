#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable,
# and saves the output to the file specified by the TEST_OUTPUT_FILE environment variable.

set -e
shopt -s extglob

case $TEST_CATEGORY in
    unit|unit-@(crypto|integration)|integration-@(common|network|epochs|access|collection|consensus|execution|verification))
        echo "Generating skipped test list for \"$TEST_CATEGORY\" tests."
    ;; 
    *)
        echo "Valid test category must be provided."
        exit 1
    ;;
esac

make crypto/relic/build

export JSON_OUTPUT=true

# run tests and process results
if [[ $TEST_CATEGORY =~ integration-(common|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    make docker-build-flow
    make -C integration -s ${BASH_REMATCH[1]}-tests > $TEST_OUTPUT_FILE
else
    case $TEST_CATEGORY in
        unit)
            make install-mock-generators
            make generate-mocks
            make -s unittest-main > $TEST_OUTPUT_FILE
        ;;
        unit-crypto)
            make -C crypto setup
            make -C crypto -s test-main > $TEST_OUTPUT_FILE
        ;;
        unit-integration)
            make -C integration -s test > $TEST_OUTPUT_FILE
        ;;
    esac
fi

