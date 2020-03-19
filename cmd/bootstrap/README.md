# Bootstrap

This package contains the CLIs to bootstrap a flow network.

## Example Process

1. `go run -tags relic ./cmd/bootstrap keys -c ./cmd/bootstrap/example_files/node_config.yml`
1. `go run -tags relic ./cmd/bootstrap dkg -s node-infos.pub.yml`
1. `go run -tags relic ./cmd/bootstrap state -a f87db879307702010104208ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc43a00a06082a8648ce3d030107a14403420004b60899344f1779bb4c6df5a81db73e5781d895df06dee951e813eba99e76dd3c9288cceb5a5a9ada390671f60b71f3fd2653ca5c4e1ccc6f8b5a62be6d17256a0201`
1. `go run -tags relic ./cmd/bootstrap block -s state.yml -n node-infos.pub.yml`
