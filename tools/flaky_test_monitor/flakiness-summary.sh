#!/usr/bin/env bash

set -e
shopt -s extglob

# download files

# GCS_BUCKET
#

# START_DATE (YYYY-MM-DD)
# NUM_DAYS

# gsutil ls "gs://production_ddp_flow_test_metrics/**"

# gs://$GCS_BUCKET/${JOB_STARTED[0]}/${JOB_STARTED[1]}-$COMMIT_SHA-$TEST_CATEGORY.json

mkdir results

for i in $(seq $NUM_DAYS); do
    day=$(date +"%Y-%m-%d" -d "$START_DATE+$((i-1)) days")
    gsutil cp -r "gs://$GCS_BUCKET/$day/*" results/
done

go run level2/process_summary2_results.go results/

go run level3/process_summary3_results.go level2-summary.json level3/flaky-test-monitor.json

# TODO: upload all level 2 folders to gcs
gsutil cp $TEST_RESULT_FILE gs://$GCS_BUCKET/${JOB_STARTED[0]}/${JOB_STARTED[1]}-$COMMIT_SHA-$TEST_CATEGORY-$JOB_ID.json

curl -X POST -H 'Content-type: application/json' --data @slack_message.json $FLAKINESS_RESULTS_WEBHOOK


# generate slack message json
# need: link from each test to the folder of results
#