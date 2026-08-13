package emulator_test

// Regression tests: intrinsically-invalid EVM transactions must not burn
// the signer's gasLimit*gasPrice.
//
// Background: geth's state transition (core/state_transition.go, v1.16.8)
// deducts gasLimit*gasPrice in buyGas() at the end of preCheck(), BEFORE the
// intrinsic-gas, EIP-7623 floor-data-gas and initcode-size checks. When one of
// those checks fails, geth returns an error without refunding and without
// incrementing the nonce — safe in vanilla Ethereum only because the whole
// transition is then discarded. Flow's emulator used to wrap the error as an
// "invalid result" and BlockView.RunTransaction / BatchRunTransactions would
// commit the state delta unconditionally (no res.Invalid() guard, unlike
// mintTo and withdrawFrom in the same file).
//
// RunTransaction and BatchRunTransactions now discard the state delta via
// proc.state.Reset() when res.Invalid(), mirroring mintTo/withdrawFrom.
// These tests pin that behavior: the result is still classified invalid,
// but no balance is burned and replaying the identical signed RLP burns
// nothing.
//
// End-to-end coverage through the public Cadence EVM.run entry point
// (including event-invisibility) lives in fvm/evm/invalid_tx_burn_test.go.

import (
	"bytes"
	"math/big"
	"testing"

	gethCore "github.com/ethereum/go-ethereum/core"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var invalidTxGasPrice = big.NewInt(1_000_000_000)

// fundTestEOA creates the well-known test EOA and funds it via a deposit.
func fundTestEOA(
	t *testing.T,
	backend *testutils.TestBackend,
	rootAddr flow.Address,
) *testutils.EOATestAccount {
	account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
	RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
		RunWithNewBlockView(t, em, func(blk types.BlockView) {
			_, err := blk.DirectCall(types.NewDepositCall(
				testutils.RandomAddress(t),
				account.Address(),
				types.MakeBigIntInFlow(1000),
				0,
			))
			require.NoError(t, err)
		})
	})
	return account
}

func balanceOf(
	t *testing.T,
	backend *testutils.TestBackend,
	rootAddr flow.Address,
	addr types.Address,
) *big.Int {
	var bal *big.Int
	RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
		RunWithNewReadOnlyBlockView(t, em, func(blk types.ReadOnlyBlockView) {
			var err error
			bal, err = blk.BalanceOf(addr)
			require.NoError(t, err)
		})
	})
	return bal
}

func nonceOf(
	t *testing.T,
	backend *testutils.TestBackend,
	rootAddr flow.Address,
	addr types.Address,
) uint64 {
	var nonce uint64
	RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
		RunWithNewReadOnlyBlockView(t, em, func(blk types.ReadOnlyBlockView) {
			var err error
			nonce, err = blk.NonceOf(addr)
			require.NoError(t, err)
		})
	})
	return nonce
}

// newBlockViewWithCoinbase opens a block view with an explicit gas fee
// collector so tests can prove no funds are credited to it.
func newBlockViewWithCoinbase(
	t *testing.T,
	em *emulator.Emulator,
) (types.BlockView, types.BlockContext) {
	ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
	ctx.GasFeeCollector = types.NewAddressFromString("coinbase")
	blk, err := em.NewBlockView(ctx)
	require.NoError(t, err)
	return blk, ctx
}

// A signed transaction with gasLimit one below the intrinsic gas floor passes
// geth's preCheck (nonce, funds, gas cap, fee cap) but fails the intrinsic-gas
// check that runs AFTER buyGas() has already deducted gasLimit*gasPrice.
// RunTransaction must discard that deduction.
func TestRunTransaction_IntrinsicGasFailureBurnsNothing(t *testing.T) {
	testutils.RunWithTestBackend(t, flow.Testnet, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			account := fundTestEOA(t, backend, rootAddr)
			signer := account.Address()

			const gasLimit = gethParams.TxGas - 1 // 20,999: one below intrinsic

			tx := account.SignTx(t, gethTypes.NewTransaction(
				0, // nonce
				testutils.RandomAddress(t).ToCommon(),
				big.NewInt(0),
				gasLimit,
				invalidTxGasPrice,
				nil,
			))

			balanceBefore := balanceOf(t, backend, rootAddr, signer)

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				blk, ctx := newBlockViewWithCoinbase(t, em)

				res, err := blk.RunTransaction(tx)
				require.NoError(t, err)

				// the transaction is still correctly classified as invalid...
				require.True(t, res.Invalid())
				require.ErrorIs(t, res.ValidationError, gethCore.ErrIntrinsicGas)

				// ...but its state delta (the buyGas deduction) is discarded
				RunWithNewReadOnlyBlockView(t, em, func(ro types.ReadOnlyBlockView) {
					bal, err := ro.BalanceOf(signer)
					require.NoError(t, err)
					require.Equal(t,
						balanceBefore,
						bal,
						"invalid tx must not commit the buyGas() deduction",
					)

					nonce, err := ro.NonceOf(signer)
					require.NoError(t, err)
					require.Zero(t, nonce)

					// nothing is credited to the fee collector either
					coinbaseBal, err := ro.BalanceOf(ctx.GasFeeCollector)
					require.NoError(t, err)
					require.Zero(t, coinbaseBal.Uint64())
				})
			})

			// replay: the identical signed transaction is still nonce-valid;
			// it must burn nothing
			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				blk, _ := newBlockViewWithCoinbase(t, em)

				res, err := blk.RunTransaction(tx)
				require.NoError(t, err)
				require.True(t, res.Invalid())
				require.ErrorIs(t, res.ValidationError, gethCore.ErrIntrinsicGas)

				RunWithNewReadOnlyBlockView(t, em, func(ro types.ReadOnlyBlockView) {
					bal, err := ro.BalanceOf(signer)
					require.NoError(t, err)
					require.Equal(t,
						balanceBefore,
						bal,
						"replayed invalid tx must not commit the buyGas() deduction",
					)

					nonce, err := ro.NonceOf(signer)
					require.NoError(t, err)
					require.Zero(t, nonce)
				})
			})

		})
	})
}

// The same discard-on-invalid behavior applies to BatchRunTransactions:
// invalid batch entries must be discarded, not committed.
func TestBatchRunTransactions_IntrinsicGasFailureBurnsNothing(t *testing.T) {
	testutils.RunWithTestBackend(t, flow.Testnet, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			account := fundTestEOA(t, backend, rootAddr)
			signer := account.Address()

			const gasLimit = gethParams.TxGas - 1

			// the SAME signed tx twice: because the nonce is never incremented,
			// both copies pass the nonce check inside a single batch
			tx := account.SignTx(t, gethTypes.NewTransaction(
				0,
				testutils.RandomAddress(t).ToCommon(),
				big.NewInt(0),
				gasLimit,
				invalidTxGasPrice,
				nil,
			))
			batch := []*gethTypes.Transaction{tx, tx}

			balanceBefore := balanceOf(t, backend, rootAddr, signer)

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				blk, _ := newBlockViewWithCoinbase(t, em)

				results, err := blk.BatchRunTransactions(batch)
				require.NoError(t, err)
				require.Len(t, results, 2)
				for _, res := range results {
					require.True(t, res.Invalid())
					require.ErrorIs(t, res.ValidationError, gethCore.ErrIntrinsicGas)
				}

				RunWithNewReadOnlyBlockView(t, em, func(ro types.ReadOnlyBlockView) {
					bal, err := ro.BalanceOf(signer)
					require.NoError(t, err)
					require.Equal(t,
						balanceBefore,
						bal,
						"batch run must not commit buyGas() deductions",
					)

					nonce, err := ro.NonceOf(signer)
					require.NoError(t, err)
					require.Zero(t, nonce)
				})

			})
		})
	})
}

// The intrinsic-gas check is not the only post-buyGas trap: under Prague
// rules a transaction whose gasLimit covers the intrinsic gas but not the
// EIP-7623 floor data gas fails AFTER buyGas() as well, hitting the same
// discard-on-invalid path. (The third check in the same trap, initcode size /
// ErrMaxInitCodeSizeExceeded, is not separately reproduced here.)
func TestRunTransaction_FloorDataGasFailureBurnsNothing(t *testing.T) {
	testutils.RunWithTestBackend(t, flow.Testnet, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			account := fundTestEOA(t, backend, rootAddr)
			signer := account.Address()

			// calldata heavy in non-zero bytes: floor data gas exceeds
			// intrinsic gas (21000 + 16/byte) for the same payload
			// (Amsterdam/EIP-7976 prices the floor at 21000 + 64/byte)
			data := bytes.Repeat([]byte{0x01}, 100)
			rules := emulator.DefaultChainConfig.Rules(blockNumber, true, 0)
			intrinsic, err := gethCore.IntrinsicGas(data, nil, nil, false, rules, gethParams.CostPerStateByte)
			require.NoError(t, err)
			floor, err := gethCore.FloorDataGas(rules, data, nil)
			require.NoError(t, err)
			require.Greater(t, floor, intrinsic.Sum(), "test parameters must straddle the two checks")

			// passes the intrinsic-gas check (equality), fails the floor check
			gasLimit := intrinsic.Sum()

			tx := account.SignTx(t, gethTypes.NewTransaction(
				0,
				testutils.RandomAddress(t).ToCommon(),
				big.NewInt(0),
				gasLimit,
				invalidTxGasPrice,
				data,
			))

			balanceBefore := balanceOf(t, backend, rootAddr, signer)

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				blk, _ := newBlockViewWithCoinbase(t, em)

				res, err := blk.RunTransaction(tx)
				require.NoError(t, err)
				require.True(t, res.Invalid())
				require.ErrorIs(t, res.ValidationError, gethCore.ErrFloorDataGas)

				RunWithNewReadOnlyBlockView(t, em, func(ro types.ReadOnlyBlockView) {
					bal, err := ro.BalanceOf(signer)
					require.NoError(t, err)
					require.Equal(t,
						balanceBefore,
						bal,
						"EIP-7623 floor-gas failure must not commit the buyGas() deduction",
					)

					nonce, err := ro.NonceOf(signer)
					require.NoError(t, err)
					require.Zero(t, nonce)
				})
			})
		})
	})
}
