# Flow

[![Build Status](https://travis-ci.com/dapperlabs/flow-go.svg?token=MYJ5scBoBxhZRGvDecen&branch=master)](https://travis-ci.com/dapperlabs/flow-go)

Flow is a fast, secure, and developer-friendly blockchain built to support the next generation of games, apps and the digital assets that power them.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Getting started](#getting-started)
- [Documentation](#documentation)
- [Installation](#installation)
  - [Setting up your environment](#setting-up-your-environment)
    - [Install Go](#install-go)
    - [Install tooling dependencies](#install-tooling-dependencies)
  - [Generating code](#generating-code)
    - [Dependency injection using Wire](#dependency-injection-using-wire)
    - [Generate gRPC stubs from protobuf files](#generate-grpc-stubs-from-protobuf-files)
    - [Generate all code](#generate-all-code)
- [Testing](#testing)
- [Contributing](#contributing)
  - [Work streams](#work-streams)
  - [Workflow](#workflow)
  - [Issues](#issues)
    - [Branches](#branches)
      - [Feature branches](#feature-branches)
    - [Pull requests](#pull-requests)
      - [Reviews](#reviews)
      - [Work-In-Progress PRs](#work-in-progress-prs)
    - [Testing](#testing-1)
  - [Code standards](#code-standards)
  - [Code documentation](#code-documentation)
    - [Documentation instructions for stream owners](#documentation-instructions-for-stream-owners)
    - [Stream package documentation](#stream-package-documentation)
      - [Auto-generated READMEs](#auto-generated-readmes)
    - [Documentation instructions for contributors](#documentation-instructions-for-contributors)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Getting started

* Read through the [project setup](#installation) instructions to install required tools
* Read the documentation pertaining to [your stream](#work-streams)
* Familiarize yourself with the [workflow](#workflow) below
* Browse the rest of this README to get up to speed on concepts like testing, code style, and common code patterns
* Contact your stream owner to receive your first task!

## Documentation

You can find a high-level overview of the Flow architecture on the [documentation website](https://bamboo-docs.herokuapp.com/). Application-level documentation lives [within the packages of this repository](#code-documentation).

## Installation

### Setting up your environment

#### Install Go

- Download and install [Go 1.13](https://golang.org/doc/install)
- Create your workspace `$GOPATH` directory and update your bash_profile to contain the following:

```bash
export `$GOPATH=$HOME/path-to-your-go-workspace/`
```

It's also a good idea to update your `$PATH` to use third party GO binaries:

```bash
export PATH="$PATH:$GOPATH/bin"
```

- Test that Go was installed correctly: https://golang.org/doc/install#testing
- Clone this repository to `$GOPATH/src/github.com/dapperlabs/flow-go/`
- Clone this repository submodules:
```bash
git submodule update --init --recursive
```

_Note: since we are using go modules and we prepend every `go` command with `GO111MODULE=on`, you can also clone this repo anywhere you want._

#### Install tooling dependencies

First, install [CMake](https://cmake.org/install/) as it is used for code generation of some tools.

Additional development tools required for code generation can be installed with this command:

```bash
make install-tools
```

### Generating code

#### Generate gRPC stubs from protobuf files

```bash
make generate-proto
```

#### Generate all code

You can run all code generators with a single command:

```bash
make generate
```

## Testing

The following command will run all unit tests for this repository:

```bash
make test
```

## Contributing

This guide provides a comprehensive overview of our development processes, guidelines, and steps required to contribute to the project.

### Work streams

Flow development is divided across several streams of work with the goal of separating concerns and facilitating rapid development.

Each stream is owned by a Flow core team member who oversees and directs all development within that stream. As a contributor, you will communicate primarily with your stream owner.

Stream owners will assign tasks to contributors and ensure that all TODOs are tracked.

| Stream         | Owner(s)                    | Home directory  |
| -------------- | --------------------------- | --------- |
| Collection  | [Peter Siemens](https://github.com/psiemens) | [/engine/collection](/engine/collection) |
| Consensus | [Alexander Hentschel](https://github.com/AlexHentschel) | [/engine/consensus](/engine/consensus) |
| Execution      | [Bastian Müller](https://github.com/turbolent) | [/engine/execution](/engine/execution) |
| Verification | [Moar Zamski](https://github.com/pazams) [Ramtin Seraj](https://github.com/ramtinms)| [/engine/verification](/engine/verification) |
| Observation | [Peter Siemens](https://github.com/psiemens)     | [/engine/observation](/engine/observation) |
| Networking | [Yahya Hassanzadeh](https://github.com/yhassanzadeh)     | [/networking/gossip](/networking/gossip) |
| Cryptography | [Tarak Ben Youssef](https://github.com/tarakby)     | [/crypto](/crypto) |
| SDK & Emulator| [Brian Ho](https://github.com/mrbrianhobo), [Peter Siemens](https://github.com/psiemens)     | [Flow Go SDK](https://github.com/dapperlabs/flow-go-sdk) |
| Ops & Performance | [Leo Zhang](https://github.com/zhangchiqing) | |
| Language & Runtime | [Bastian Müller](https://github.com/turbolent) | [/language](/language) |

### Workflow

### Issues

Development tasks are assigned using GitHub issues. Each issue will contain a breakdown of the required task and any necessary background information, as well as an esitmate of the required work. You are expected to track the progress of issues assigned to you and provide updates if needed, in the form of issue comments.

If you need to create a new issue, please use the provided issue templates to ensure that all necessary information is included.

#### Branches

Work for a specific task should be completed in a separate branch corresponding to the issue for that task.

When creating a new branch, use the following convention: `<your-name>/<issue-number>-<issue-description>`

For example, `peter/125-update-transaction` is the name of a branch Peter is working on, and corresponds to issue 125 regarding transaction updates.

##### Feature branches

When working on a larger feature, feel free to create a feature branch with the following format: `feature/<feature-name>`.

#### Pull requests

You should open a pull request when you have completed work for a task and would like to receive a review from teammates and stream owners. Please use the provided pull request template when opening a PR.

##### Reviews

You should request a review from any relevant team members who are also working within your stream. The stream owner will automatically be requested for review.

A PR can be merged once all CI checks pass and it is approved by at least two people, including the stream owner.

If you are reviewing another team member's PR, please keep feedback constructive and friendly.

##### Work-In-Progress PRs

You can open a WIP pull request to track ongoing work for a task.

#### Testing

Each PR that you open should include necessary tests to ensure the correctness and stability of your code. The specific testing requirements for each task will be defined in the issue itself.

### Code standards

The Flow project has a high standard for code quality and expects all submitted PRs to meet the guidelines outlined in our [code style guide](code-style.md).

To develop in _production level_ standard of the Flow project, the following best practice set is recommended:
- Please think as a user of your code, and optimize the interface for as easy and error-prone experience as possible.
	- TODO: add example(s)
- Please optimize the time, memory, and communication overhead of our code.
	- TODO: add example(s)
- Please properly identify the possible errors and make sure that they are handled.
	- TODO: add example(s)
- Please properly identify the corner cases and edge cases and handle them on our happy path.
	- TODO: add example(s)
- Please make sure that the packages you developed are independent and portable.
	- TODO: add example(s)
- Please make sure that variables, functions, packages, etc, are well-named.
	- TODO: add example(s)
- Please write tests for your code that covers all possible range of inputs.
	- TODO: add example(s)
- Please test each (tiny) module individually, and then move to the composability testing.
	- TODO: add example(s)
- Please break your implementation into as concise and precise modules, functions, and methods as possible.
	- TODO: add example(s)
- Please make sure that your code is well-documented with a proper quick start that helps other engineers to quickly utilize your code without any hard effort.
	- TODO: add example(s)
- Please append your suggestions to this list, advertise them within the team, and replace the _"TODO: add example"_ parts with the code pieces you think are exemplary and worthy to share.

TODO: add style guide

### Code documentation

The application-level documentation for Flow lives inside each of the sub-packages of this repository.

#### Documentation instructions for stream owners

Stream owners are responsible for ensuring that all code owned by their stream is well-documented. Documentation for a stream should accomplish the following:

1. Provide an overview of all stream functions
2. Outline the different packages used by the stream
3. Highlight dependencies on other streams

Each stream should contain a README in its home directory. This page, which acts as a jumping-off point for new contributors, should list each function of the stream along with a short description and links to relevant packages.

Here's an example: [cmd/consensus/README.md](cmd/consensus/README.md)

#### Stream package documentation

All packages owned by a stream should be documented using `godoc`.

Here's an example: [network/gossip](network/gossip)

##### Auto-generated READMEs

A `README.md` can be generated from the `godoc` output by updating the make file with the path of your package. The above example was generated by this line:

```bash
godoc2md github.com/dapperlabs/flow-go/engine/collection/clusters > internal/roles/collect/clusters/README.md
```

WARNING: `godoc2md` is currently not working with Go modules.

Once your package is added to that file, running `go generate` in the root of this repo will generate a new `README.md`.

#### Documentation instructions for contributors

TODO: describe documentation standards for all code
