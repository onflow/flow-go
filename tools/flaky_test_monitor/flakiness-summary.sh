#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable,
# processes the results and generates a summary file in JSON format, and uploads the file to the
# GCS bucket specified by the GCS_BUCKET environment variable.

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

# save result processing script command for later use
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
export TEST_FLAKY=true
export TEST_LONG_RUNNING=true

# run tests and process results
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

# upload results to GCS bucket
gsutil cp test-results.json gs://$GCS_BUCKET/$COMMIT_SHA-$JOB_STARTED-$TEST_CATEGORY.json