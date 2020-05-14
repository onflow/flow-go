## Flow Integration Tests

A small testing utility which uses local Docker to run a network of nodes.

Uses `github.com/m4ksio/testingdock` which is a slightly enhanced fork of some other library.
See `tests/mvp_test.go` for example usage.

### Running tests

1. Make sure you have the latest docker images built by running `make docker-build-flow` (this is implicitly called if you call `make integration-test` from the flow-go repository root).
2. Run tests with with `make integration-test` or by using IDE - its a standard unit test after all.

### Organization

All integration test files live under `tests`. This is used to distinguish
between unit tests of testing utilities and integration tests for the network
in the Makefile.
