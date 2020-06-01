# Flow [![GoDoc](https://godoc.org/github.com/onflow/flow-go?status.svg)](https://godoc.org/github.com/onflow/flow-go)

Flow is a fast, secure, and developer-friendly blockchain built to support the 
next generation of games, apps and the digital assets that power them. Read more
about it [here](https://github.com/onflow/flow).

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Getting started](#getting-started)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Installation](#installation)
  - [Install Go](#install-go)
  - [Install tooling dependencies](#install-tooling-dependencies)
- [Building](#building)
- [Local Network](#local-network)
- [Testing](#testing)
- [Code Generation](#code-generation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Getting started

* Read through the [project setup](#installation) instructions to install required tools
* Read the documentation pertaining to [your stream](#work-streams)
* Familiarize yourself with the [workflow](#workflow) below
* Browse the rest of this README to get up to speed on concepts like testing, code style, and common code patterns
* Contact your stream owner to receive your first task!

## Documentation

You can find a high-level overview of the Flow architecture on the [documentation website](https://www.onflow.org/primer). 
Module-level documentation lives along with the corresponding code [within the packages of this repository](#code-documentation).

## Contributing

To learn how to contribute, please read the [Contributing Guide](/CONTRIBUTING.md).

## Installation

### Install Go

- Download and install [Go 1.13](https://golang.org/doc/install) or later
- Create your workspace `$GOPATH` directory and update your `bash_profile` to contain the following:

```bash
export `$GOPATH=$HOME/path-to-your-go-workspace/`
```

- Confirm that Go was installed correctly: https://golang.org/doc/install#testing
- Clone this repository to `$GOPATH/src/github.com/onflow/flow-go/`
- Clone this repository's submodules:
```bash
git submodule update --init --recursive
```

💡 Since this repository uses [Go modules](https://github.com/golang/go/wiki/Modules) 
and we prepend every `go` command with `GO111MODULE=on`, you can also clone this repo 
anywhere you want.

### Install tooling dependencies

First, install [CMake](https://cmake.org/install/) as it is used for code generation of some tools.

Next, install [Docker](https://docs.docker.com/get-docker/) as it is used for running a local 
network and integration tests.

All other tooling dependencies can be installed automatically with this command:

```bash
make install-tools
```

At this point, you should be ready to build, test, and run Flow! 🎉

## Building

The recommended way to build and run Flow for local development is using Docker. 

Build a Docker image for all nodes:
```bash
make docker-build-flow
```

Build a Docker image for a particular node role (replace `$ROLE` with `collection`, `consensus`, etc.):
```bash
make docker-build-$ROLE
```

## Local Network

A local version of the network can be run for manual testing and integration. 
See the [Local Network Guide](/integration/localnet/README.md) for instructions.

## Testing

Flow has a unit test suite and an integration test suite. Unit tests for a module 
live within the module they are testing. Integration tests live in `integration/tests`.

Run the unit test suite:

```bash
make unit-test
```

Run the integration test suite:

```bash
make integration-test
```

## Code Generation

Generated code is kept up to date in the repository, so should be committed whenever it changes. 

Run all code generators:

```bash
make generate
```

Generate protobuf stubs:

```bash
make generate-proto
```

Generate mocks used for unit tests:

```bash
make generate-mocks
```
