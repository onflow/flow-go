#!/usr/bin/env bash

set -e
shopt -s extglob

export JOB_STARTED=$(date -Iseconds)

case $TEST_CATEGORY in
    unit|crypto-unit|integration-@(unit|common|network|epochs|access|collection|consensus|execution|verification))
        echo "Generating flakiness summary for \"$TEST_CATEGORY\" tests."
    ;; 
    *)
        echo "Valid test category must be provided."
        exit 1
    ;;
esac

process_results="go run $(realpath ./process_results.go) test-results.json"

cd ../..

# checkout specified commit
if [[ -n $COMMIT_SHA ]]
then
    git checkout $COMMIT_SHA
fi

export COMMIT_SHA=$(git rev-parse HEAD)
export COMMIT_DATE=$(git show --no-patch --no-notes --pretty='%cI' $COMMIT_SHA)

make crypto/relic/build

export JSON_OUTPUT=true

if [[ $TEST_CATEGORY =~ integration-(common|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    make docker-build-flow
    make -C integration -s ${BASH_REMATCH[1]}-tests | $process_results
else
    case $TEST_CATEGORY in
        unit)
            make install-mock-generators
            make generate-mocks
            make -s unittest-main | $process_results
        ;;
        crypto-unit)
            make -C crypto -s test | $process_results
        ;;
        integration-unit)
            make -C integration -s test | $process_results
        ;;
    esac
fi

gsutil cp test-results.json gs://$GCS_BUCKET/$COMMIT_SHA-$JOB_STARTED-$TEST_CATEGORY.json