generate_sql_query() {
    sed -e "s/<bigquery_dataset>/$BIGQUERY_DATASET/g" \
    -e "s/<bigquery_table>/$BIGQUERY_TABLE/g" latest-commit.sql
}

# TODO: remove project_id flag
generate_sql_query | bq query --format=csv --project_id=dapperlabs-data --use_legacy_sql=false | sed -n '2 p'