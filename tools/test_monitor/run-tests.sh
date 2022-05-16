#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable.

set -e
shopt -s extglob

echo "test category (run-tests):" $TEST_CATEGORY

# run tests and process results

if [[ $TEST_CATEGORY =~ integration-(bft|ghost|mvp|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
  echo "killing and removing orphaned containers from previous run"
#    # kill and remove orphaned containers from previous run
#    containers=$(docker ps -a -q)
#
#    if [ ! -z "$containers" ]
#    then
#        docker rm -f $containers > /dev/null
#    fi
#

  echo "running $TEST_CATEGORY tests"

#    make -C integration -s ${BASH_REMATCH[1]}-tests
else
    case $TEST_CATEGORY in
        unit)
          echo "running unittest-main"
#          make -s unittest-main
        ;;
        unit-crypto)
          echo "running crypto unit tests"
#          make -C crypto -s test
        ;;
        unit-integration)
          echo "running integration unit tests"
#          make -C integration -s test
        ;;
        *)
          echo "unrecognized test category (run-tests):" $TEST_CATEGORY
          exit 1
        ;;
    esac
fi

