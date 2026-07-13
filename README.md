# flow-go — Flow Network Reference Implementation in Go

[![GoDoc](https://godoc.org/github.com/onflow/flow-go?status.svg)](https://godoc.org/github.com/onflow/flow-go)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Release](https://img.shields.io/github/v/release/onflow/flow-go)](https://github.com/onflow/flow-go/releases)
[![Discord](https://img.shields.io/badge/Discord-Flow-7289DA?logo=discord&logoColor=white)](https://discord.gg/flow)
[![Built on Flow](https://img.shields.io/badge/Built_on-Flow-00EF8B)](https://flow.com)

Reference implementation of the [Flow network](https://flow.com) in Go. Flow is a Layer 1 proof-of-stake blockchain built for consumer apps, AI Agents, and DeFi at scale. This repo hosts the node software — consensus, execution, verification, access, and collection roles — and the Cadence VM integration used on mainnet, testnet, and local networks.

## TL;DR

- **What:** flow-go is the Go reference implementation of the Flow network, a Layer 1 proof-of-stake blockchain.
- **Who it's for:** protocol contributors, node operators, and teams building infrastructure on or adjacent to Flow.
- **Why use it:** canonical source of truth for Flow consensus (HotStuff), multi-role architecture, Cadence VM integration, and Flow EVM.
- **Status:** see [Releases](https://github.com/onflow/flow-go/releases) for the latest version. Live on mainnet.
- **License:** AGPL-3.0
- **Related repos:** [cadence](https://github.com/onflow/cadence) (language) · [flow-cli](https://github.com/onflow/flow-cli) (CLI) · [fcl-js](https://github.com/onflow/fcl-js) (JS client) · [flow-go-sdk](https://github.com/onflow/flow-go-sdk) (Go client)
- The reference Go implementation for the Flow network, open-sourced since 2019.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

## Table of Contents

- [Getting started](#getting-started)
- [Documentation](#documentation)
- [Installation](#installation)
    - [Clone Repository](#clone-repository)
    - [Install Dependencies](#install-dependencies)
- [Development Workflow](#development-workflow)
    - [Testing](#testing)
    - [Building](#building)
    - [Local Network](#local-network)
    - [Code Generation](#code-generation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Getting started

- To install all dependencies and tools, see the [project setup](#installation) guide
- To dig into more documentation about Flow, see the [documentation](#documentation)
- To learn how to contribute, see the [contributing guide](/CONTRIBUTING.md)
- To see information on developing Flow, see the [development workflow](#development-workflow)

## Documentation

You can find an overview of the Flow architecture on the [documentation website](https://www.flow.com/primer).

Development on Flow is divided into work streams. Each work stream has a home directory containing high-level
documentation for the stream, as well as links to documentation for relevant components used by that work stream.

The following table lists all work streams and links to their home directory and documentation:

| Work Stream        | Home directory                             |
|--------------------|--------------------------------------------|
| Access Node        | [/cmd/access](/cmd/access)                 |
| Collection Node    | [/cmd/collection](/cmd/collection)         |
| Consensus Node     | [/cmd/consensus](/cmd/consensus)           |
| Execution Node     | [/cmd/execution](/cmd/execution)           |
| Verification Node  | [/cmd/verification](/cmd/verification)     |
| Observer Service   | [/cmd/observer](/cmd/observer)             |
| HotStuff           | [/consensus/hotstuff](/consensus/hotstuff) |
| Storage            | [/storage](/storage)                       |
| Ledger             | [/ledger](/ledger)                         |
| Networking         | [/network](/network/)                      |
| Cryptography       | [/crypto](/crypto)                         |

## Installation

- Clone this repository
- Install [Go](https://golang.org/doc/install) (Flow requires Go 1.25 and later)
- Install [Docker](https://docs.docker.com/get-docker/), which is used for running a local network and integration tests
- Make sure the [`GOPATH`](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable) and `GOBIN` environment variables
  are set, and `GOBIN` is added to your path:

    ```bash
    export GOPATH=$(go env GOPATH)
    export GOBIN=$GOPATH/bin
    export PATH=$PATH:$GOBIN
    ```

  Add these to your shell profile to persist them for future runs.
- Then, run the following command:

    ```bash
    make install-tools
    ```

At this point, you should be ready to build, test, and run Flow! 🎉

## Development Workflow

### Testing

Flow has a unit test suite and an integration test suite. Unit tests for a module live within the module they are
testing. Integration tests live in `integration/tests`.

Run the unit test suite:

```bash
make test
```

Run the integration test suite:

```bash
make integration-test
```

### Building

The recommended way to build and run Flow for local development is using Docker.

Build a Docker image for all nodes:

```bash
make docker-native-build-flow
```

Build a Docker image for a particular node role (replace `$ROLE` with `collection`, `consensus`, etc.):

```bash
make docker-native-build-$ROLE
```

#### Building a binary for the access node

Build the binary for an access node that can be run directly on the machine without using Docker.

```bash
make docker-native-build-access-binary
```
_this builds a binary for Linux/x86_64 machine_.

The make command will generate a binary called `flow_access_node`

### Importing the module

When importing the `github.com/onflow/flow-go` module in your Go project, testing or building your project may require setting extra Go flags because the module requires [cgo](https://pkg.go.dev/cmd/cgo). In particular, `CGO_ENABLED` must be set to `1` if `cgo` isn't enabled by default. This constraint comes from the underlying cryptography library. Refer to the [crypto repository build](https://github.com/onflow/crypto?tab=readme-ov-file#build) for more details.

### Local Network

A local version of the network can be run for manual testing and integration. See
the [Local Network Guide](/integration/localnet/README.md) for instructions.

### Code Generation

Generated code is kept up to date in the repository, so should be committed whenever it changes.

Run all code generators:

```bash
make generate
```

Generate protobuf stubs:

```bash
make generate-proto
```

Generate OpenAPI schema models:

```bash
make generate-openapi
```

Generate mocks used for unit tests:

```bash
make generate-mocks
```

### Mocks

We use `github.com/vektra/mockery` for mocking interfaces within tests. The configuration is in `.mockery.yaml`.

#### Adding and updating packages

You can add new packages by their fully qualified name. e.g.
```
github.com/onflow/flow-go/module/execution:
```

This will add all interfaces within the `module/execution/` (non-recursive).

#### Mocking functions

Mockery dropped support for generating function mocks. Instead, you can use this pattern:

1. Create a `mock_interfaces` directory in the package where the function mock exists.
2. Add a file that mocks the function. for example, this mocks the `StateMachineEventsTelemetryFactory(candidateView uint64) protocol_state.StateMachineTelemetryConsumer
```golang
package mockinterfaces

import "github.com/onflow/flow-go/state/protocol/protocol_state"

// ExecForkActor allows to create a mock for the ExecForkActor callback
type StateMachineEventsTelemetryFactory interface {
	Execute(candidateView uint64) protocol_state.StateMachineTelemetryConsumer
}
```
3. Add the package to `.mockery.yaml`. Note: specify the directory where you want the mock to be placed.
```
  github.com/onflow/flow-go/state/protocol/protocol_state/mock_interfaces:
    config:
      dir: "state/protocol/protocol_state/mock"
```

## FAQ

### What is flow-go?
flow-go is the Go reference implementation of the Flow network — a Layer 1 proof-of-stake blockchain. It implements the node software (Access, Collection, Consensus, Execution, Verification roles), the HotStuff consensus algorithm, ledger, storage, networking, and cryptography subsystems.

### How does flow-go differ from the Cadence repo?
flow-go is the **protocol / node** implementation. [`onflow/cadence`](https://github.com/onflow/cadence) is the **Cadence smart contract language** (compiler, interpreter, type system). flow-go embeds the Cadence VM to execute transactions, but the language itself is developed in the Cadence repo.

### What are the five node roles in Flow?
Access, Collection, Consensus, Execution, and Verification. Each has its own entry point under `/cmd/`. There is also an Observer service for staking-free read-only access.

### Which Go version does flow-go require?
Go 1.25 or later. See the [Installation](#installation) section for the full environment setup.

### Where is the consensus algorithm implemented?
Under [`/consensus/hotstuff`](/consensus/hotstuff). HotStuff is the BFT consensus family used by Flow.

### How do I run a local Flow network?
See the [Local Network Guide](/integration/localnet/README.md) for a full local multi-role network suitable for integration testing.

### Where do I report a security issue?
See [SECURITY.md](/SECURITY.md) for the responsible disclosure policy.

## About Flow

This repo is the Go reference implementation of the [Flow network](https://flow.com), a Layer 1 blockchain built for consumer applications, AI Agents, and DeFi at scale.

- Developer docs: https://developers.flow.com
- Cadence language: https://cadence-lang.org
- Community: [Flow Discord](https://discord.gg/flow) · [Flow Forum](https://forum.flow.com)
- Governance: [Flow Improvement Proposals](https://github.com/onflow/flips)
