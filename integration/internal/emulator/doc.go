// Package emulator is a minimal version of the Flow Emulator (https://github.com/onflow/flow-emulator)
// for use within some integration tests for flow-go.
// Using an Emulator is desirable for test cases where:
//   - we don't want to, or can't, run the test case against a local Docker network (package integration/testnet)
//   - we want the test to include execution of smart contract code in a realistic environment
//
// Before using this package, flow-go's integration tests used the Flow Emulator directly.
// This created a repository-wise circular dependency and complicated version upgrades (see https://github.com/onflow/flow-go/issues/2863).
// The main purpose for this package is to replace that dependency with minimal ongoing
// maintenance overhead.
package emulator
