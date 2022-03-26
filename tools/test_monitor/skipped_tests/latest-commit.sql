SELECT
  DISTINCT commit_sha
FROM
  `<bigquery_dataset>.<bigquery_table>`
WHERE
  commit_date = (
  SELECT
    MAX(commit_date)
  FROM
    `<bigquery_dataset>.<bigquery_table>`)