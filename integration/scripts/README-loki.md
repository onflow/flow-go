# Sending Test Logs to Loki/Grafana

This directory contains tools to send test logs to Loki for viewing in Grafana.

## Prerequisites

1. **Loki must be running** - The localnet setup includes Loki. Start it with:
   ```bash
   cd integration/localnet
   docker-compose -f docker-compose.metrics.yml up -d loki
   ```

2. **Verify Loki is accessible**:
   ```bash
   curl http://localhost:3100/ready
   ```

## Usage

### Basic Usage

Pipe your test output to the script:

```bash
cd integration
go test -failfast ./tests/verification/ --run=TestVerifyScheduledTransactions -v | ./scripts/send-to-loki.sh
```

### With Custom Labels

Add labels to make logs easier to filter in Grafana:

```bash
cd integration
go test -failfast ./tests/verification/ --run=TestVerifyScheduledTransactions -v | \
  ./scripts/send-to-loki.sh -test=TestVerifyScheduledTransactions -file=verification_test.go
```

### Environment Variables

You can also configure via environment variables:

```bash
cd integration
export LOKI_URL="http://localhost:3100/loki/api/v1/push"
export JOB="my-test-job"
go test -failfast ./tests/verification/ --run=TestVerifyScheduledTransactions -v | \
  ./scripts/send-to-loki.sh
```

## Viewing Logs in Grafana

1. **Start Grafana** (if not already running):
   ```bash
   cd integration/localnet
   docker-compose -f docker-compose.metrics.yml up -d grafana
   ```

2. **Open Grafana**: http://localhost:3000

3. **Explore Logs**:
   - Go to Explore (compass icon in left sidebar)
   - Select "Loki" as the data source
   - Use LogQL queries to filter logs:
     ```
     {job="go-test"}
     ```
     Or with test name:
     ```
     {job="go-test", test="TestVerifyScheduledTransactions"}
     ```

4. **Example Queries**:
   - All test logs: `{job="go-test"}`
   - Specific test: `{job="go-test", test="TestVerifyScheduledTransactions"}`
   - Search for errors: `{job="go-test"} |= "error"`
   - Search for specific text: `{job="go-test"} |= "FAIL"`

## How It Works

The script reads from stdin line by line and:
1. Echoes each line to stdout (so you still see output in terminal)
2. Batches log lines together
3. Sends batches to Loki's push API with proper timestamps and labels

The Go implementation (`send-to-loki.go`) is more efficient and recommended. The shell script falls back to a curl-based approach if Go is not available.

## Cleaning Up Old Logs

See [CLEANUP-LOGS.md](./CLEANUP-LOGS.md) for detailed instructions on cleaning up old logs.

Quick examples:
```bash
cd integration
# Delete all Loki logs
./scripts/cleanup-loki.sh --all

# Delete logs older than 7 days
./scripts/cleanup-loki.sh --older-than 7d

# Delete logs from specific test
./scripts/cleanup-loki.sh --test TestVerifyScheduledTransactions
```

## Troubleshooting

- **Loki not accessible**: Make sure Loki is running and accessible at `http://localhost:3100`
- **No logs appearing**: Check that the test is actually producing output (use `-v` flag)
- **Connection errors**: Verify the LOKI_URL is correct and Loki is running
- **Disk space issues**: Use the cleanup script to remove old logs (see CLEANUP-LOGS.md)

