## Running an example that shows metrics data?

1. Launch a local metrics server, which exposes the metrics data via the `/metrics` endpoint.

You can choose one of the following:
```
go run module/metrics/example/collection/main.go
```
```
go run module/metrics/example/execution/main.go
```
```
go run module/metrics/example/verification/main.go
```
```
go run module/metrics/example/consensus/main.go
```
The above commands each launch a metrics server on localhost with port 9090

2. Install the prometheus server
```
brew install prometheus
```

3. Launch the prometheus server to scrape the metrics from our local metrics server
```
prometheus --config.file=module/metrics/example/prometheus.yml
```

4. Open the prometheus UI in your browser
http://localhost:9090/graph

5. Go to the "Graph" tab to query and verify the collected metrics data
type `consensus_cur_view`, and press "Enter" to view the metrics data over time
