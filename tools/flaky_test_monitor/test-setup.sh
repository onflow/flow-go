#!/usr/bin/env bash

set -e
shopt -s extglob

make crypto_setup_gopath

if [[ $TEST_CATEGORY =~ integration-(common|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    make docker-build-flow
else
    case $TEST_CATEGORY in
        unit)
            make install-mock-generators
            make generate-mocks
        ;;
        unit-crypto)
            make crypto_setup_tests
        ;;
        unit-integration)
        ;;
    esac
fi

