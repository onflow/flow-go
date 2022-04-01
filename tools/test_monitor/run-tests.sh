#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable.

set -e
shopt -s extglob

# run tests and process results
if [[ $TEST_CATEGORY =~ integration-(ghost|mvp|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    # kill and remove orphaned containers from previous run
    sudo systemctl restart docker >&2
    (docker rm -f $(docker ps -a -q) || true) >&2
    
    make -C integration -s ${BASH_REMATCH[1]}-tests
else
    case $TEST_CATEGORY in
        unit)
            make -s unittest-main
        ;;
        unit-crypto)
            make -C crypto -s test
        ;;
        unit-integration)
            make -C integration -s test
        ;;
    esac
fi

