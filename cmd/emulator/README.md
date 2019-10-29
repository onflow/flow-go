# Emulator

The Flow emulator emulates the Flow blockchain and the interface for interacting 
with it. 

## Use
The recommended way to use the emulator is as a Docker container. 

Docker builds for the emulator are automatically built and pushed to 
`gcr.io/dl-flow/emulator`, tagged by commit.

### Configuration
The emulator can be configured by setting environment variables when running 
the container. The configurable variables are specified [by the config of the `start` command](https://github.com/dapperlabs/flow-go/blob/master/internal/cli/emulator/start/start.go#L18-L24).
The environment variable names are the uppercased struct fields names, prefixed
by `FLOW_`.

For example, to run the emulator on port 9001 in verbose mode:
```bash
docker run -e FLOW_PORT=9001 -e FLOW_VERBOSE=true gcr.io/dl-flow/emulator
```

## Building
To build the container locally, use `make docker-build-emulator`.