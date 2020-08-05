<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Contributing to Cadence](#contributing-to-cadence)
  - [Table Of Contents](#table-of-contents)
  - [How Can I Contribute?](#how-can-i-contribute)
    - [Reporting Bugs](#reporting-bugs)
      - [Before Submitting A Bug Report](#before-submitting-a-bug-report)
      - [How Do I Submit A (Good) Bug Report?](#how-do-i-submit-a-good-bug-report)
    - [Suggesting Enhancements](#suggesting-enhancements)
      - [Before Submitting An Enhancement Suggestion](#before-submitting-an-enhancement-suggestion)
      - [How Do I Submit A (Good) Enhancement Suggestion?](#how-do-i-submit-a-good-enhancement-suggestion)
    - [Your First Code Contribution](#your-first-code-contribution)
    - [Pull Requests](#pull-requests)
  - [Styleguides](#styleguides)
    - [Git Commit Messages](#git-commit-messages)
    - [Go Styleguide](#go-styleguide)
  - [Additional Notes](#additional-notes)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Contributing to Cadence

The following is a set of guidelines for contributing to Cadence.
These are mostly guidelines, not rules.
Use your best judgment, and feel free to propose changes to this document in a pull request.

## Table Of Contents

[Getting Started](#project-overview)

[How Can I Contribute?](#how-can-i-contribute)

- [Reporting Bugs](#reporting-bugs)
- [Suggesting Enhancements](#suggesting-enhancements)
- [Your First Code Contribution](#your-first-code-contribution)
- [Pull Requests](#pull-requests)

[Styleguides](#styleguides)

- [Git Commit Messages](#git-commit-messages)
- [Go Styleguide](#go-styleguide)

[Additional Notes](#additional-notes)

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

- **What is the version of the Cadence you're using**?
- **What's the name and version of the Operating System you're using**?

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
- **Explain why this enhancement would be useful** to Cadence users.

### Your First Code Contribution

Unsure where to begin contributing to Cadence?
You can start by looking through these "Good first issue" and "Help wanted" issues:

- [Good first issues](https://github.com/onflow/cadence/labels/good%20first%20issue):
  issues which should only require a few lines of code, and a test or two.
- [Help wanted issues](https://github.com/onflow/cadence/labels/help%20wanted):
  issues which should be a bit more involved than "Good first issue" issues.

Both issue lists are sorted by total number of comments.
While not perfect, number of comments is a reasonable proxy for impact a given change will have.

### Pull Requests

The process described here has several goals:

- Maintain code quality
- Fix problems that are important to users
- Engage the community in working toward the best possible Cadence UX
- Enable a sustainable system for the Cadence's maintainers to review contributions

Please follow the [styleguides](#styleguides) to have your contribution considered by the maintainers.
Reviewer(s) may ask you to complete additional design work, tests,
or other changes before your pull request can be ultimately accepted.

## Styleguides

Before contributing, make sure to examine the project to get familiar with the patterns and style already being used.

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

### Go Styleguide

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

Thank you for your interest in contributing to Cadence!
