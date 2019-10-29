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
by `BAM_`.

For example, to run the emulator on port 9001 in verbose mode:
```bash
docker run -e BAM_PORT=9001 -e BAM_VERBOSE=true gcr.io/dl-flow/emulator
```

#### Accounts
The emulator uses a `flow.json` configuration file to store persistent
configuration, including account keys. In order to start, at least one
key (called the root key) must be configured. This key is used by default
when creating other accounts.

Because Docker does not persist files by default, this file will be 
re-generated each time the emulator starts when using Docker. For situations
where it is important that the emulator always uses the same root key (ie.
unit tests) you can specify a hex-encoded key as an environment variable.

```bash
docker run -e BAM_ROOTKEY=<hex-encoded key> gcr.io/dl-flow/emulator
```

To generate a root key, use the `keys generate` command.
```bash
docker run gcr.io/dl-flow/emulator keys generate
```

## Building
To build the container locally, use `make docker-build-emulator`.