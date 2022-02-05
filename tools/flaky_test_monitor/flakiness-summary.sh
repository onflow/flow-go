#!/usr/bin/env bash

set -e
shopt -s extglob

# TODO: remove this
export GCS_BUCKET=flow-internal
export START_DATE=2021-01-01
export NUM_DAYS=7

mkdir results

for i in $(seq $NUM_DAYS); do
    day=$(date +"%Y-%m-%d" -d "$START_DATE+$((i-1)) days")
    gsutil -m cp -r "gs://$GCS_BUCKET/$day/*" results/
done

export FOLDER_NAME=$(echo $START_DATE | sed 's/[-]//g')-$(date +"%Y%m%d" -d "$START_DATE+$((NUM_DAYS-1)) days")

upload_data () {
    mkdir results/$1
    mv results/$1*.json results/$1/
    go run level2/process_summary2_results.go results/$1
    mv level2-summary.json level2-summary-$1.json
    mkdir test_outputs $1_test_outputs
    go run level3/process_summary3_results.go level2-summary-$1.json level3/flaky-test-monitor.json
    mv level3-summary.json level3-summary-$1.json
    gsutil -m cp -r $1_test_outputs gs://$GCS_BUCKET/SUMMARIES/$FOLDER_NAME
}

upload_data unit
upload_data integration

go run slack-message-generator.go level3-summary-unit.json level3-summary-integration.json

# TODO
# curl -X POST -H 'Content-type: application/json' --data @slack-message.json $FLAKINESS_RESULTS_WEBHOOK


