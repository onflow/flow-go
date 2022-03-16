#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable,
# processes the results and generates a summary file in JSON format, and uploads the file to the
# GCS bucket specified by the GCS_BUCKET environment variable.

set -e
shopt -s extglob

export JOB_STARTED=$(TZ=":America/Vancouver" date -Iseconds)
export JOB_ID=$(cat /proc/sys/kernel/random/uuid | sed 's/[-]//g' | head -c 20)

case $TEST_CATEGORY in
    unit|unit-@(crypto|integration)|integration-@(common|network|epochs|access|collection|consensus|execution|verification))
        echo "Generating flakiness summary for \"$TEST_CATEGORY\" tests."
    ;; 
    *)
        echo "Valid test category must be provided."
        exit 1
    ;;
esac

# save result processing script command for later use
export TEST_RESULT_FILE=test-results-level1.json

process_results="go run $(realpath ./level1/process_summary1_results.go) $TEST_RESULT_FILE"

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
        unit-crypto)
            make -C crypto -s test | $process_results
        ;;
        unit-integration)
            make -C integration -s test | $process_results
        ;;
    esac
fi

GCS_URI="gs://$GCS_BUCKET/${JOB_STARTED%T*}/$TEST_CATEGORY-$JOB_ID.json"

# upload results to GCS bucket
gsutil cp $TEST_RESULT_FILE $GCS_URI

# upload results to BigQuery
bq load --source_format=NEWLINE_DELIMITED_JSON --autodetect $BIGQUERY_TABLE $GCS_URI