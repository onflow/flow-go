#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable,
# and saves the output to the file specified by the TEST_OUTPUT_FILE environment variable.

set -e
shopt -s extglob

export JSON_OUTPUT=true

# run tests and process results
if [[ $TEST_CATEGORY =~ integration-(common|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    make -C integration -s ${BASH_REMATCH[1]}-tests > $TEST_OUTPUT_FILE
else
    case $TEST_CATEGORY in
        unit)
            make -s unittest-main
            # TODO: add this back > $TEST_OUTPUT_FILE
        ;;
        unit-crypto)
            make -C crypto -s test > $TEST_OUTPUT_FILE
        ;;
        unit-integration)
            make -C integration -s test > $TEST_OUTPUT_FILE
        ;;
    esac
fi

