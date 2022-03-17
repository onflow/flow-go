#!/usr/bin/env bash

# This script runs the tests in the category specified by the TEST_CATEGORY environment variable,
# processes the results and generates a summary file in JSON format, and uploads the file to the
# GCS bucket specified by the GCS_BUCKET environment variable.

set -e
shopt -s extglob

export JOB_STARTED=$(TZ=":America/Vancouver" date -Iseconds)

case $TEST_CATEGORY in
    unit|unit-@(crypto|integration)|integration-@(common|network|epochs|access|collection|consensus|execution|verification))
        echo "Generating flakiness summary for \"$TEST_CATEGORY\" tests."
    ;; 
    *)
        echo "Valid test category must be provided."
        exit 1
    ;;
esac

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

OUTPUT_FILE=output.txt

# run tests and process results
if [[ $TEST_CATEGORY =~ integration-(common|network|epochs|access|collection|consensus|execution|verification)$ ]]
then
    make docker-build-flow
    make -C integration -s ${BASH_REMATCH[1]}-tests > $OUTPUT_FILE
else
    case $TEST_CATEGORY in
        unit)
            make install-mock-generators
            make generate-mocks
            make -s unittest-main > $OUTPUT_FILE
        ;;
        unit-crypto)
            make -C crypto setup
            make -C crypto -s test-main > $OUTPUT_FILE
        ;;
        unit-integration)
            make -C integration -s test > $OUTPUT_FILE
        ;;
    esac
fi

cat $OUTPUT_FILE | go run tools/flaky_test_monitor/level1/process_summary1_results.go results.json

GCS_URI="gs://$GCS_BUCKET/${JOB_STARTED%T*}/$TEST_CATEGORY-$RUN_ID.json"

# upload results to GCS bucket
gsutil cp results.json $GCS_URI

# upload results to BigQuery
bq load --source_format=NEWLINE_DELIMITED_JSON --autodetect $BIGQUERY_TABLE $GCS_URI