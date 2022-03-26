generate_sql_query() {
    sed -e "s/<bigquery_dataset>/$BIGQUERY_DATASET/g" \
    -e "s/<bigquery_table>/$BIGQUERY_TABLE/g" \
    -e "s/<skip_reason>/$SKIP_REASON/g" \
    -e "s/<test_category>/$TEST_CATEGORY/g" \
    -e "s/<commit_sha>/$COMMIT_SHA/g" skipped-tests.sql
}

# generate_test_names() {

# }

# TODO: remove project_id flag
generate_sql_query | bq query --format=json --project_id=dapperlabs-data --use_legacy_sql=false