# Contributing to Flow Go

The following is a set of guidelines for contributing to the Flow Go SDK. These are mostly guidelines, 
not rules. Use your best judgment, and feel free to propose changes to this document in a pull request.

## Work streams

Flow development is divided across several streams of work with the goal of separating
concerns and facilitating rapid development.

Each stream is owned by a Flow core team member who oversees and directs all development
within that stream. As a contributor, you will communicate primarily with your stream owner.

Stream owners will assign tasks to contributors and ensure that all TODOs are tracked.

| Stream         | Owner(s)                    | Home directory  |
| -------------- | --------------------------- | --------- |
| Access | [Vishal Changrani](https://github.com/vishalchangraniaxiom), [Peter Siemens](https://github.com/psiemens)     | [/cmd/access](/cmd/access) |
| Collection  | [Jordan Schalm](https://github.com/jordanschalm), [Peter Siemens](https://github.com/psiemens) | [/cmd/collection](/engine/collection) |
| Consensus | [Alexander Hentschel](https://github.com/AlexHentschel), [Leo Zhang](https://github.com/zhangchiqing), [Max Wolter](https://github.com/awfm), [Jordan Schalm](https://github.com/jordanschalm) | [/cmd/consensus](/engine/consensus) |
| Execution | [Maks Pawlak](https://github.com/m4ksio) | [/cmd/execution](/cmd/execution) |
| Verification | [Ramtin Seraj](https://github.com/ramtinms), [Yahya Hassanzadeh](https://github.com/yhassanzadeh) | [/engine/verification](/cmd/verification) |
| Networking | [Yahya Hassanzadeh](https://github.com/yhassanzadeh), [Vishal Changrani](https://github.com/vishalchangraniaxiom)     | [/network](/network/) |
| Cryptography | [Tarak Ben Youssef](https://github.com/tarakby)     | [/crypto](/crypto) |
| SDK & Emulator| [Peter Siemens](https://github.com/psiemens)     | [Flow Go SDK](https://github.com/onflow/flow-go-sdk) |
| Language & Runtime | [Bastian Müller](https://github.com/turbolent) | [Cadence](https://github.com/onflow/cadence) |
| Ops & Performance | [Leo Zhang](https://github.com/zhangchiqing) | |

## How Can I Contribute?

### Reporting Bugs

#### Before Submitting A Bug Report

- **Search existing issues** to see if the problem has already been reported.
  If it has **and the issue is still open**, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Bug Report?

Explain the problem and include additional details to help maintainers reproduce the problem:

- **Use a clear and descriptive title** for the issue to identify the problem.
- **Describe the exact steps which reproduce the problem** in as many details as possible.
  When listing steps, **don't just say what you did, but explain how you did it**.
- **Provide specific examples to demonstrate the steps**.
  Include links to files or GitHub projects, or copy/pasteable snippets, which you use in those examples.
  If you're providing snippets in the issue,
  use [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines).
- **Describe the behavior you observed after following the steps** and point out what exactly is the problem with that behavior.
- **Explain which behavior you expected to see instead and why.**
- **Include error messages and stack traces** which show the output / crash and clearly demonstrate the problem.

Provide more context by answering these questions:

- **Can you reliably reproduce the issue?** If not, provide details about how often the problem happens
  and under which conditions it normally happens.

Include details about your configuration and environment:

- **What is the version of Flow you're using**?
- **What is the name and version of the Operating System you're using**?

### Suggesting Enhancements

#### Before Submitting An Enhancement Suggestion

- **Perform a cursory search** to see if the enhancement has already been suggested.
  If it has, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Enhancement Suggestion?

Enhancement suggestions are tracked as [GitHub issues](https://guides.github.com/features/issues/).
Create an issue and provide the following information:

- **Use a clear and descriptive title** for the issue to identify the suggestion.
- **Provide a step-by-step description of the suggested enhancement** in as many details as possible.
- **Provide specific examples to demonstrate the steps**.
  Include copy/pasteable snippets which you use in those examples,
  as [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines).
- **Describe the current behavior** and **explain which behavior you expected to see instead** and why.
- **Explain why this enhancement would be useful** to users.


### Your First Code Contribution

Unsure where to begin contributing to the Flow Go SDK?
You can start by looking through these `good-first-issue` and `help-wanted` issues:

- [Good first issues](https://github.com/onflow/flow-go/labels/good%20first%20issue):
  issues which should only require a few lines of code, and a test or two.
- [Help wanted issues](https://github.com/onflow/flow-go/labels/help%20wanted):
  issues which should be a bit more involved than `good-first-issue` issues.

Both issue lists are sorted by total number of comments.
While not perfect, number of comments is a reasonable proxy for impact a given change will have.

Read the [Workflow](#workflow) section for a detailed guide on the development workflow.

## Workflow

### Issues

Development tasks are assigned using GitHub issues. Each issue will contain a breakdown 
of the required task and any necessary background information, as well as an estimate 
of the required work. Please track task progress and updates by commenting in the issue. 

If you need to create a new issue, please use the provided issue templates to ensure that 
all necessary information is included.

### Branches

Work for a specific task should be completed in a separate branch corresponding to the issue for that task.

We use the following convention for naming branches: `<your-name>/<issue-number>-<issue-description>`.

For example, `peter/125-update-transaction` is the name of a branch Peter is working on, 
and corresponds to issue #125 regarding transaction updates.

#### Feature Branches

When working on a larger feature, feel free to create a feature branch to track progress.

We use the the following convention for naming feature branches: `feature/<feature-name>`.

#### Pull Requests

You should open a pull request when you have completed work for a task and would like to 
receive a review from teammates and stream owners. Please use the provided pull request 
template when opening a PR.

Feel free to open a Work-In-Progress PR to track ongoing work or receive feedback on an
early version of a feature.

#### Code Review

You should request a review from any relevant team members who are also working within 
your stream. The stream owner will automatically be requested for review.

A PR can be merged once all CI checks pass and it is approved by at least two people, including the stream owner.

If you are reviewing another contributor's PR, please keep feedback constructive and friendly.

### Testing

Each PR that you open should include necessary tests to ensure the correctness and stability 
of your code. Any specific testing requirements for each task will be defined in the issue itself.

### Documentation

All new code should include appropriate documentation to ensure it is readable and maintainable.

All public variables and functions should be documented using `godoc`. 

High-level module documentation should be included in the appropriate package's `README`.

#### Stream Owner Documentation

Stream owners are responsible for ensuring that all code owned by their stream is well-documented. 
Documentation for a stream should accomplish the following:

1. Provide an overview of all stream functions
2. Outline the different packages used by the stream
3. Highlight dependencies on other streams

Each stream should contain a README in its home directory. This page, which acts as a 
jumping-off point for new contributors, should list each function of the stream along
with a short description and links to relevant packages.

Here's an example: [cmd/consensus/README.md](cmd/consensus/README.md)

## Style Guide

Before contributing, make sure to examine the project to get familiar with the patterns and style already being used.

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

### Go Style Guide

The majority of this project is written Go.

We try to follow the coding guidelines from the Go community.

- Code should be formatted using `gofmt`
- Code should pass the linter: `make lint`
- Code should follow the guidelines covered in
  [Effective Go](http://golang.org/doc/effective_go.html)
  and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Code should be commented
- Code should pass all tests: `make test`

## Additional Notes

Thank you for your interest in contributing to Flow!
