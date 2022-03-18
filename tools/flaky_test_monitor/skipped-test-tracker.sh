#!/usr/bin/env bash

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

# checkout specified commit
git checkout $COMMIT_SHA

export COMMIT_DATE=$(git show --no-patch --no-notes --pretty='%cI' $COMMIT_SHA)

make crypto/relic/build

export JSON_OUTPUT=true

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

cat $OUTPUT_FILE | go run tools/flaky_test_monitor/level1/process_summary1_results.go results.json skipped-tests.json

# TODO: send the skipped tests list to BigQuery
cat skipped-tests.json

