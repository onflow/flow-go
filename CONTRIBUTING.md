# Contribution Guide

This guide provides a comprehensive overview of our development processes, guidelines, and steps required to contribute to the project.

## Getting Started

By the time you read this you should be assigned to a work stream. Here's how to get started:

* Familiarize yourself with the workflow below
* Read through the [project setup](/docs/setup.md) instructions to install required tools
* Read the documentation pertaining to [your stream](/docs/streams)
* Browse the [remaining documentation](/docs) to get up to speed on concepts like testing, code style, and common code patterns
* Contact your stream lead to receive your first task

## Work Streams

Bamboo development is divided across several streams of work with the goal of separating concerns and facilitating rapid development. 

Each stream is owned by a Bamboo core team member who oversees and directs all development within that stream. As a contributor, you will communicate primarily with your stream owner.

Stream owners will assign tasks to contributors and ensure that all TODOs are tracked.

| Stream         | Owner(s)                    | Packages  |
| -------------- | --------------------------- | --------- |
| Pre-execution  | [Peter Siemens](https://github.com/psiemens])   | [/internal/access](/internal/access), [/cmd/access](/cmd/access) |
| Execution      | [Bastian Müller](https://github.com/@turbolent) | [/runtime](/runtime), [/language](/language), [/internal/execute](/internal/execute), [/cmd/execute](/cmd/execute) |
| Post-execution | [Moar Zamski](https://github.com/@pazams)     | [/internal/access](/internal/access), [/cmd/access](/cmd/access) |
| Consensus | [Alexander Hentschel](https://github.com/@AlexHentschel)     | [/consensus](/consensus), [/internal/security](/internal/security) |
| Networking | [Yahya Hassanzadeh](https://github.com/@yhassanzadeh)     | [/network](/network) |
| Keystone | [Tarak Ben Youssef](https://github.com/@tarakby)     | [/pkg/crypto](/pkg/crypto) |
| Emulator | [Brian Ho](https://github.com/@mrbrianhobo), [Peter Siemens](https://github.com/@psiemens)     | [/internal/emulator](/internal/emulator), [/client](/client), [/cmd/bamboo](/cmd/bamboo)|
| Engineer Performance | [Timofey Smirnov](https://github.com/@tsmirnov) | |

## Workflow

### Issues

Development tasks are assigned using GitHub issues. Each issue will contain a breakdown of the required task and any necessary background information, as well as an esitmate of the required work. You are expected to track the progress of issues assigned to you and provide updates if needed, in the form of issue comments.

If you need to create a new issue, please use the provided issue templates to ensure that all necessary information is included.

### Branches

Work for a specific task should be completed in a separate branch corresponding to the issue for that task.

When creating a new branch, use the following convention: `<your-name>/<issue-number>-<issue-description>`

For example, `peter/125-update-transaction` is the name of a branch Peter is working on, and corresponds to issue 125 regarding transaction updates.

#### Feature Branches

When working on a larger feature, feel free to create a feature branch with the following format: `feature/<feature-name>`.

### Pull Requests

You should open a pull request when you have completed work for a task and would like to receive a review from teammates and stream owners. Please use the provided pull request template when opening a PR.

#### Reviews

You should request a review from any relevant team members who are also working within your stream. The stream owner will automatically be requested for review.

A PR can be merged once all CI checks pass and it is approved by at least two people, including the stream owner.

If you are reviewing another team member's PR, please keep feedback constructive and friendly.

#### Work-In-Progress PRs

You can open a WIP pull request to track ongoing work for a task.

### Testing

Each PR that you open should include necessary tests to ensure the correctness and stability of your code. The specific testing requirements for each task will be defined in the issue itself.

## Code Standards

The Bamboo project has a high standard for code quality and expects all submitted PRs to meet the guidelines outlined in our [code style guide](docs/code-style.md).
