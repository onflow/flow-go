package evm_test

// Regression test: an intrinsically-invalid EVM transaction must not
// burn the signer's gasLimit*gasPrice.
//
// geth deducts gasLimit*gasPrice in buyGas() before the intrinsic-gas
// check runs; when the check fails, the deduction must be discarded, not
// committed. This test drives a signed, intrinsically-invalid EVM
// transaction (gasLimit below the 21_000 intrinsic gas floor, gasPrice > 0)
// through the public Cadence `EVM.run` entry point and asserts that the
// transaction is still reported as invalid (and no event is emitted for
// it), but the signer's balance is untouched and replaying the identical
// signed RLP burns nothing.
// Unit-level regressions (RunTransaction, BatchRunTransactions, and the
// EIP-7623 floor-data-gas variant) live in
// fvm/evm/emulator/emulator_invalid_tx_burn_test.go.
import (
	"fmt"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type snapTree = snapshot.SnapshotTree

func TestInvalidTxDoesNotBurnGas_EndToEnd(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()

	const gasLimit = uint64(20_999) // one below the 21_000 intrinsic gas floor
	gasPrice := big.NewInt(1_000_000_000)
	expectedBurn := new(big.Int).Mul(big.NewInt(int64(gasLimit)), gasPrice)

	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {
			sc := systemcontracts.SystemContractsForChain(chain.ChainID())
			coinbaseAddr := types.Address{1, 2, 3}

			code := fmt.Appendf(nil,
				`
				import EVM from %s
				transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
					prepare(account: &Account) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)
						assert(res.status == EVM.Status.invalid, message: "unexpected status")
						assert(res.errorCode == 209, message: "unexpected error code: \(res.errorCode)")
					}
				}
				`,
				sc.EVMContract.Address.HexWithPrefix(),
			)

			// baseline state of the signer EOA
			balanceBefore := types.BalanceToBigInt(getEVMAccountBalance(t, ctx, vm, snapshot, testAccount.Address()))
			nonceBefore := getEVMAccountNonce(t, ctx, vm, snapshot, testAccount.Address())
			require.Equal(t, uint64(0), nonceBefore)

			// signed tx that passes preCheck (nonce, funds, gas-limit cap, fee cap)
			// but fails the intrinsic gas check AFTER geth's buyGas()
			innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
				testContract.DeployedAt.ToCommon(),
				testContract.MakeCallData(t, "store", big.NewInt(12)),
				big.NewInt(0),
				gasLimit,
				gasPrice,
			)

			innerTx := cadence.NewArray(
				unittest.BytesToCdcUInt8(innerTxBytes),
			).WithType(stdlib.EVMTransactionBytesCadenceType)
			coinbase := cadence.NewArray(
				unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType)

			runOnce := func(snap snapTree) (snapTree, fvm.ProcedureOutput) {
				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				state, output, err := vm.Run(ctx, fvm.Transaction(txBody, 0), snap)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				return snap.Append(state), output
			}

			// --- first submission ---
			snapshot, output := runOnce(snapshot)

			// event-invisibility: no TransactionExecuted event for the invalid tx
			require.Len(t, output.Events, 0)

			balanceAfter := types.BalanceToBigInt(getEVMAccountBalance(t, ctx, vm, snapshot, testAccount.Address()))
			nonceAfter := getEVMAccountNonce(t, ctx, vm, snapshot, testAccount.Address())

			// the signer's balance is untouched by the invalid tx
			require.Equal(t,
				balanceBefore,
				balanceAfter,
				"invalid tx must not burn gasLimit*gasPrice",
			)
			// nonce not incremented -> the same signed RLP remains submittable
			require.Equal(t, nonceBefore, nonceAfter)

			// --- replay: identical RLP burns nothing either ---
			snapshot, output = runOnce(snapshot)
			require.Len(t, output.Events, 0)

			balanceAfterReplay := types.BalanceToBigInt(getEVMAccountBalance(t, ctx, vm, snapshot, testAccount.Address()))
			nonceAfterReplay := getEVMAccountNonce(t, ctx, vm, snapshot, testAccount.Address())

			// replaying the invalid tx also burns nothing
			require.Equal(t,
				balanceBefore,
				balanceAfterReplay,
				"replayed invalid tx must not burn gasLimit*gasPrice",
			)
			require.Equal(t, nonceBefore, nonceAfterReplay)

			t.Logf("balance %s -> %s -> %s (nothing burned; gasLimit*gasPrice of %s stayed with the signer), nonce at %d",
				balanceBefore, balanceAfter, balanceAfterReplay, expectedBurn, nonceAfterReplay)
		})
}
