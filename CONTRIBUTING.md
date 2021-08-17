
# Contributing to Flow Go

Thank you for your interest in contributing to Flow!

The following is a set of guidelines for contributing to Flow Go. These are mostly guidelines, 
not rules. Use your best judgment, and feel free to propose changes to this document in a pull request.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
    - [Before Submitting A Bug Report](#before-submitting-a-bug-report)
    - [How Do I Submit A (Good) Bug Report?](#how-do-i-submit-a-good-bug-report)
  - [Suggesting Enhancements](#suggesting-enhancements)
    - [Before Submitting An Enhancement Suggestion](#before-submitting-an-enhancement-suggestion)
    - [How Do I Submit A (Good) Enhancement Suggestion?](#how-do-i-submit-a-good-enhancement-suggestion)
  - [Your First Code Contribution](#your-first-code-contribution)
- [Pull Request Checklist](#pull-request-checklist)
- [Style Guide](#style-guide)
  - [Git Commit Messages](#git-commit-messages)
  - [Go Style Guide](#go-style-guide)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## How Can I Contribute?

### Reporting Bugs

#### Before Submitting A Bug Report

Please **search existing issues** to see if the problem has already been reported.
If it has **and the issue is still open**, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Bug Report?

Descriptive and detailed bug reports are incredibly valuable to help maintainers reproduce and
diagnose the problem. When writing a bug report, consider the following guidelines for making
it as impactful as possible.

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

Please **search existing issues** to see if the enhancement has already been suggested.
If it has, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Enhancement Suggestion?

Enhancement suggestions are tracked as [GitHub issues](https://guides.github.com/features/issues/).
When suggesting an enhancement, consider the following guidelines for 

- **Use a clear and descriptive title** for the issue to identify the suggestion.
- **Provide a step-by-step description of the suggested enhancement** in as many details as possible.
- **Provide specific examples to demonstrate the steps**.
  Include copy/pasteable snippets which you use in those examples,
  as [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines).
- **Describe the current behavior** and **explain which behavior you expected to see instead** and why.
- **Explain why this enhancement would be useful** to users.

### Your First Code Contribution

Unsure where to begin contributing to Flow Go?
You can start by looking through these `good-first-issue` and `help-wanted` issues:

- [Good first issues](https://github.com/onflow/flow-go/labels/good%20first%20issue):
  issues which should only require a few lines of code, and a test or two.
- [Help wanted issues](https://github.com/onflow/flow-go/labels/help%20wanted):
  issues which should be a bit more involved than `good-first-issue` issues.

Both issue lists are sorted by total number of comments.
While not perfect, number of comments is a reasonable proxy for impact a given change will have.

When you're ready to start, see the [development workflow](/README.md#development-workflow), 
[PR checklist](#pull-request-checklist), and [style guide](#style-guide) for more information.

## Pull Request Checklist

When you're ready for your contribution to be reviewed, please ensure it satisfies the
following when creating your pull request:

* Ensure your PR is up-to-date with `master` and has no conflicts
* Ensure your PR passes existing tests and the linter
* Ensure your PR adds relevant tests for any fixed bugs or added features
* Ensure your PR adds or updates any relevant documentation

A reviewer will be assigned automatically when your PR is created.

We use [bors](https://github.com/bors-ng/bors-ng) merge bot to ensure that the `master` branch never breaks.
Once a PR is approved, you can comment on it with the following to add your PR to the merge queue:

```
bors merge
```

If the PR passes CI, it will automatically be pushed to the `master` branch. If it fails, bors will comment
on the PR so you can fix it.

See the [documentation](https://bors.tech/documentation/) for a more comprehensive list of bors commands.

## Style Guide

The following is a brief summary of the coding style used in Flow. 

The best way to familiarize yourself with the patterns and styles in use in this project 
is to read through some code!

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

### Go Style Guide

The majority of this project is written in Go.

We try to follow the coding guidelines from the Go community.

- Code should be formatted using `go fmt`
- Code should pass the linter: `make lint`
- Code should follow the guidelines covered in
  [Effective Go](http://golang.org/doc/effective_go.html)
  and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Code should be commented
- Code should pass all tests: `make test`

