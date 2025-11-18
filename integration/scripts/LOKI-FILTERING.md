# Filtering Go Test Logs in Loki

This guide shows how to filter and search go test logs in Loki using LogQL queries.

## Available Labels

When you send logs using `send-to-loki.sh`, the following labels are available:

- `job` - Default: `"go-test"` (can be changed with `-job` flag)
- `test` - Test name (if provided with `-test` flag)
- `file` - Test file name (if provided with `-file` flag)
- Any additional custom labels (format: `key=value`)

## Basic Filtering by Labels

### All test logs
```
{job="go-test"}
```

### Specific test
```
{job="go-test", test="TestVerifyScheduledTransactions"}
```

### Specific test file
```
{job="go-test", file="verification_test.go"}
```

### Multiple label filters
```
{job="go-test", test="TestVerifyScheduledTransactions", file="verification_test.go"}
```

## Content-Based Filtering

### Search for specific text (case-sensitive)
```
{job="go-test"} |= "FAIL"
```

### Search for specific text (case-insensitive)
```
{job="go-test"} |~ "(?i)error"
```

### Search for text NOT containing a string
```
{job="go-test"} != "PASS"
```

### Multiple text filters (AND)
```
{job="go-test"} |= "FAIL" |~ "(?i)timeout"
```

### Search with regex
```
{job="go-test"} |~ "panic|fatal|error"
```

## Common Test Log Patterns

### Find test failures
```
{job="go-test"} |= "FAIL"
```

### Find test errors
```
{job="go-test"} |~ "(?i)(error|fail|panic)"
```

### Find specific test output
```
{job="go-test"} |= "TestVerifyScheduledTransactions"
```

### Find logs with timestamps (test start/end markers)
```
{job="go-test"} |~ "START TESTING|FINISH TESTING"
```

### Find assertion failures
```
{job="go-test"} |~ "assert|require|expected|got"
```

### Find test output from specific package
```
{job="go-test"} |= "verification"
```

## Combining Label and Content Filters

### Specific test with error
```
{job="go-test", test="TestVerifyScheduledTransactions"} |~ "(?i)error"
```

### All tests with failures in a specific file
```
{job="go-test", file="verification_test.go"} |= "FAIL"
```

## Time-Based Filtering

In Grafana Explore, you can:
1. Set the time range in the top right (e.g., "Last 15 minutes", "Last 1 hour")
2. Use relative time in queries:
   ```
   {job="go-test"} [5m]
   ```

## Advanced Queries

### Count log lines
```
count_over_time({job="go-test"}[5m])
```

### Rate of log lines
```
rate({job="go-test"}[1m])
```

### Filter and count errors
```
count_over_time({job="go-test"} |~ "(?i)error" [5m])
```

### Extract JSON fields (if logs are JSON)
```
{job="go-test"} | json | level="error"
```

### Line formatting
```
{job="go-test"} |= "FAIL" | line_format "{{.test}}: {{.message}}"
```

## Practical Examples

### Example 1: Find all failures from a specific test run
```
{job="go-test", test="TestVerifyScheduledTransactions"} |= "FAIL"
```

### Example 2: Find all error messages excluding PASS logs
```
{job="go-test"} |~ "(?i)error" != "PASS"
```

### Example 3: Find test start/end markers to see test duration
```
{job="go-test"} |~ "===============> (START|FINISH) TESTING"
```

### Example 4: Find logs from last 10 minutes with specific text
```
{job="go-test"} |= "verification" [10m]
```

### Example 5: Find all logs except verbose debug output
```
{job="go-test"} != "DEBUG" != "TRACE"
```

## Using in Grafana

1. **Open Grafana**: http://localhost:3000
2. **Go to Explore** (compass icon in left sidebar)
3. **Select "Loki"** as the data source
4. **Enter your LogQL query** in the query box
5. **Set time range** (top right corner)
6. **Click "Run query"** or press Enter

## LogQL Operators Reference

- `|=` - Line contains string (case-sensitive)
- `!=` - Line does not contain string
- `|~` - Line matches regex
- `!~` - Line does not match regex
- `| json` - Parse log line as JSON
- `| line_format` - Format log line output
- `| label_format` - Format labels

## Tips

1. **Use labels for filtering first** - They're more efficient than content filters
2. **Combine label and content filters** - Use labels to narrow scope, then filter content
3. **Use regex for flexible matching** - `|~` with regex patterns
4. **Set appropriate time ranges** - Narrow time range for better performance
5. **Use test markers** - If your tests log start/end markers, use them to find test boundaries

## Example Workflow

1. Run test with labels:
   ```bash
   cd integration
   go test -failfast ./tests/verification/ --run=TestVerifyScheduledTransactions -v | \
     ./scripts/send-to-loki.sh -test=TestVerifyScheduledTransactions -file=verification_test.go
   ```

2. In Grafana, query for the specific test:
   ```
   {job="go-test", test="TestVerifyScheduledTransactions"}
   ```

3. If test failed, filter for errors:
   ```
   {job="go-test", test="TestVerifyScheduledTransactions"} |~ "(?i)(error|fail)"
   ```

4. Look for specific assertion failures:
   ```
   {job="go-test", test="TestVerifyScheduledTransactions"} |~ "expected|got|assert"
   ```

