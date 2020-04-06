## How to test the code that collections metrics data?

1. Launch a metrics server in local, which exposes the metrics data via the `/metrics` endpoint.
```
go run module/metrics/test/local.go
```
The above command launches a metrics server in local with port 9090

2. Install the prometheus server
```
brew install prometheus
```

3. Launch the prometheus server to scrape the metrics from our local metrics server
```
prometheus --config.file=module/metrics/test/prometheus.yml
```

4. Go to the "Graph" tab to query and verify the collected metrics data
type `cur_view`, and press "Enter" to view the metrics data over time
