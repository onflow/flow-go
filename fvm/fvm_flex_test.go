package fvm_test

import (
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestWithFlexEnabled(t *testing.T) {

	chain, vm := createChainAndVm(flow.Emulator)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	snapshotTree := snapshot.NewSnapshotTree(nil)

	baseBootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, _, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, baseBootstrapOpts...),
		snapshotTree)
	require.NoError(t, err)

	snapshotTree = snapshotTree.Append(executionSnapshot)

	// now setup ctx with flex enabled
	ctx = fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithFlexEnabled(true),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	script := []byte(`
			transaction(addr bytes: [UInt8; 20]) {
				prepare(signer: AuthAccount) {
				}
				execute {
					Flex.FlexAddress(bytes: bytes)
				}
			}
	`)

	encodedArg, err := jsoncdc.Encode(
		cadence.NewArray([]cadence.Value{
			cadence.UInt8(1), cadence.UInt8(1),
			cadence.UInt8(2), cadence.UInt8(2),
			cadence.UInt8(3), cadence.UInt8(3),
			cadence.UInt8(4), cadence.UInt8(4),
			cadence.UInt8(5), cadence.UInt8(5),
			cadence.UInt8(6), cadence.UInt8(6),
			cadence.UInt8(7), cadence.UInt8(7),
			cadence.UInt8(8), cadence.UInt8(8),
			cadence.UInt8(9), cadence.UInt8(9),
			cadence.UInt8(10), cadence.UInt8(10),
		}),
	)
	require.NoError(t, err)

	txBody := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(chain.ServiceAddress()).
		AddArgument(encodedArg)

	t.Run("first operation", func(t *testing.T) {
		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		// transaction should pass
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)
	})

	t.Run("second operation (reuse of vm)", func(t *testing.T) {
		// Currently reusing cadence run time doens't work in this environment
		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		// transaction should still pass
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)
	})
}
