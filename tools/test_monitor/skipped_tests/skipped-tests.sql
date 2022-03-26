SELECT
  package,
  test
FROM
  `<bigquery_dataset>.<bigquery_table>`
WHERE
  skip_reason = "<skip_reason>"
  AND category = "<test_category>"
  AND commit_sha = "<commit_sha>"