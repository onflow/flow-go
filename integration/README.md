## Flow Integration Tests

A small testing utility which uses local Docker to run a network of nodes.

Uses `github.com/m4ksio/testingdock` which is a slightly enhanced fork of some other library.
See `tests/mvp_test.go` for example usage.

Run with `make integration-test` or by using IDE - its a standard unit test after all.

### Organization

All integration test files live under `tests`. This is used to distinguish 
between unit tests of testing utilities and integration tests for the network
in the Makefile.
