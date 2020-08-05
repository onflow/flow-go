<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Cadence Programming Language – K Semantics](#cadence-programming-language--k-semantics)
  - [Dependencies](#dependencies)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Cadence Programming Language – K Semantics

A K definition of the programming language.

This currently only defines the language syntax and rules to execute a subset of the language.

`make test` will run the parsing tests from `/tests/parser`,
which are extracted from the positive and negative unit tests
of the Go interpreter.

## Dependencies

Using this syntax requires the K framework,
which can be installed from a recent snapshot such as

https://github.com/kframework/k/releases/tag/nightly-36cdcbe1e

On macOS and using Homebrew, download the "Mac OS X Mojave Homebrew Bottle" and install via:
`brew install -f kframework-5.0.0.mojave.bottle.tar.gz`
