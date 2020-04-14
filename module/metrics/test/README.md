## How to test the code that collections metrics data?

1. Launch a local metrics server, which exposes the metrics data via the `/metrics` endpoint.

You can choose one of the following:
```
go run module/metrics/test/collection/main.go
```
```
go run module/metrics/test/execution/main.go
```
```
go run module/metrics/test/verification/main.go
```
The above commands each launch a metrics server on localhost with port 9090

2. Install the prometheus server
```
brew install prometheus
```

3. Launch the prometheus server to scrape the metrics from our local metrics server
```
prometheus --config.file=module/metrics/test/prometheus.yml
```

4. Go to the "Graph" tab to query and verify the collected metrics data
type `consensus_cur_view`, and press "Enter" to view the metrics data over time
