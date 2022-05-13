#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable.

set -e
shopt -s extglob

echo "test category:" $TEST_CATEGORY

# run tests and process results

if [[ $TEST_CATEGORY =~ integration-(bft|ghost|mvp|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    # kill and remove orphaned containers from previous run
    containers=$(docker ps -a -q)

    if [ ! -z "$containers" ]
    then
        docker rm -f $containers > /dev/null
    fi

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
        *)
          echo "unrecognized test category:" $TEST_CATEGORY
          exit 1
        ;;
    esac
fi

