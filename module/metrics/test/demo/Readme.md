## Verification Metrics Demo
This demo runs a local tracer server with a Prometheus server interacting with it to monitor collected metrics.
1. Install the prometheus server
```
brew install prometheus
```
2. Launch a metrics server in local, which exposes the metrics data via the `/metrics` endpoint.
```
go run module/metrics/test/local.go
```
The above command launches a metrics server in local with port 9090


3. Launch the prometheus server to scrape the metrics from our local metrics server
```
prometheus --config.file=module/metrics/test/prometheus.yml
```

4. Go to the "Graph" tab to query and verify the collected metrics data
type `result_approvals_per_block` or `chunks_checked_per_block`, and press "Enter" to view the metrics data over time
