# Cleaning Up Old Logs in Loki

This guide covers multiple methods to clean up old logs from Loki.

## Method 1: Delete All Loki Data (Quick Cleanup)

This removes all logs stored in Loki. **Warning: This deletes ALL logs permanently.**

### Stop Loki first:
```bash
cd integration/localnet
docker compose -f docker-compose.metrics.yml stop loki
```

### Delete Loki data directory:
```bash
cd integration/localnet
rm -rf ./data/loki
```

### Restart Loki:
```bash
docker compose -f docker-compose.metrics.yml up -d loki
```

## Method 2: Clean All Metrics Data

The Makefile has a `clean-data` target that removes all data including Loki, Prometheus, Tempo, and node data:

```bash
cd integration/localnet
make clean-data
```

**Warning: This deletes ALL data (Loki, Prometheus, Tempo, and all node data).**

## Method 3: Delete Logs via Loki API (Selective Deletion)

You can delete specific logs using Loki's deletion API. This is useful for deleting logs from specific tests or time ranges.

### Delete logs by label selector and time range:

```bash
# Delete all logs from go-test job in the last 24 hours
curl -X POST 'http://localhost:3100/loki/api/v1/delete' \
  -H 'Content-Type: application/json' \
  -d '{
    "selector": "{job=\"go-test\"}",
    "start": "'$(date -u -d '24 hours ago' +%s)000000000'",
    "end": "'$(date -u +%s)000000000'"
  }'
```

### Delete logs from a specific test:

```bash
# Delete logs from a specific test
curl -X POST 'http://localhost:3100/loki/api/v1/delete' \
  -H 'Content-Type: application/json' \
  -d '{
    "selector": "{job=\"go-test\", test=\"TestVerifyScheduledTransactions\"}",
    "start": "0",
    "end": "'$(date -u +%s)000000000'"
  }'
```

### Delete logs older than a specific date:

```bash
# Delete all go-test logs older than 7 days
curl -X POST 'http://localhost:3100/loki/api/v1/delete' \
  -H 'Content-Type: application/json' \
  -d '{
    "selector": "{job=\"go-test\"}",
    "start": "0",
    "end": "'$(date -u -d '7 days ago' +%s)000000000'"
  }'
```

**Note:** The timestamps are in nanoseconds since epoch. The examples above use `date` to calculate timestamps.

## Method 4: Configure Automatic Retention

To automatically delete old logs, you need to create a custom Loki configuration file with retention settings.

### Create a custom Loki config:

Create `integration/localnet/conf/loki-local.yaml`:

```yaml
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    instance_addr: 127.0.0.1
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

ruler:
  alertmanager_url: http://localhost:9093

# Enable retention
compactor:
  working_directory: /loki/compactor
  shared_store: filesystem
  retention_enabled: true
  retention_delete_delay: 2h
  retention_delete_worker_count: 150

# Configure retention period (7 days = 168h)
limits_config:
  retention_period: 168h  # Retain logs for 7 days
  # Optional: Different retention for different streams
  # retention_stream:
  #   - selector: '{job="go-test"}'
  #     priority: 1
  #     retention_period: 24h  # Keep test logs for only 24 hours
```

### Update docker-compose.metrics.yml:

Update the loki service to use your custom config:

```yaml
loki:
  image: grafana/loki:main-f1bbdc5
  user: root
  command: [ "-config.file=/etc/loki/loki-local.yaml" ]
  volumes:
    - ./conf/loki-local.yaml:/etc/loki/loki-local.yaml:z
    - ./data/loki:/loki:z
  ports:
    - "3100:3100"
```

### Restart Loki:

```bash
cd integration/localnet
docker compose -f docker-compose.metrics.yml up -d loki
```

## Method 5: Using the Cleanup Script

A helper script is provided to make cleanup easier:

```bash
cd integration
# Delete all Loki logs
./scripts/cleanup-loki.sh --all

# Delete logs older than 7 days
./scripts/cleanup-loki.sh --older-than 7d

# Delete logs from specific test
./scripts/cleanup-loki.sh --test TestVerifyScheduledTransactions

# Delete logs from specific job
./scripts/cleanup-loki.sh --job go-test
```

## Checking Disk Usage

### Check Loki data directory size:

```bash
du -sh integration/localnet/data/loki
```

### Check all metrics data size:

```bash
du -sh integration/localnet/data/*
```

## Recommendations

1. **For Development**: Use Method 1 (delete all) or Method 4 (automatic retention with short period like 24h)
2. **For Selective Cleanup**: Use Method 3 (API deletion) to remove specific test logs
3. **For Production-like Setup**: Use Method 4 with appropriate retention periods

## Troubleshooting

### Loki won't start after cleanup:

If Loki fails to start after deleting data, make sure the directory exists:

```bash
mkdir -p integration/localnet/data/loki
docker compose -f docker-compose.metrics.yml up -d loki
```

### Deletion API returns errors:

- Make sure Loki is running: `curl http://localhost:3100/ready`
- Check that the selector syntax is correct
- Verify timestamps are in nanoseconds (append 9 zeros to Unix timestamp)

### Retention not working:

- Ensure `compactor.retention_enabled: true` is set
- Check that the compactor is running (it should start automatically)
- Verify the retention period is set correctly in `limits_config`

