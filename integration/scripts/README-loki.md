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

