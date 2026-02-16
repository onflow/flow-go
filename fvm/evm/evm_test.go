package evm_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/impl"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEVMRun(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()

	t.Run("testing EVM.run (happy case)", func(t *testing.T) {

		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
							assert(res.deployedContract == nil, message: "unexpected deployed contract")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(1),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)
				snapshot = snapshot.Append(state)

				// assert event fields are correct
				require.Len(t, output.Events, 2)
				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)
				require.NoError(t, err)

				// fee transfer event
				feeTransferEvent := output.Events[1]
				feeTranferEventPayload := TxEventToPayload(t, feeTransferEvent, sc.EVMContract.Address)
				require.NoError(t, err)
				require.Equal(t, uint16(types.ErrCodeNoError), feeTranferEventPayload.ErrorCode)
				require.Equal(t, uint16(1), feeTranferEventPayload.Index)
				require.Equal(t, uint64(21000), feeTranferEventPayload.GasConsumed)

				// commit block
				blockEventPayload, snapshot := callEVMHeartBeat(t,
					ctx,
					vm,
					snapshot)

				require.NotEmpty(t, blockEventPayload.Hash)
				require.Equal(t, uint64(64785), blockEventPayload.TotalGasUsed)
				require.NotEmpty(t, blockEventPayload.Hash)

				txHashes := types.TransactionHashes{txEventPayload.Hash, feeTranferEventPayload.Hash}
				require.Equal(t,
					txHashes.RootHash(),
					blockEventPayload.TransactionHashRoot,
				)
				require.NotEmpty(t, blockEventPayload.ReceiptRoot)

				require.Equal(t, innerTxBytes, txEventPayload.Payload)
				require.Equal(t, uint16(types.ErrCodeNoError), txEventPayload.ErrorCode)
				require.Equal(t, uint16(0), txEventPayload.Index)
				require.Equal(t, blockEventPayload.Height, txEventPayload.BlockHeight)
				require.Equal(t, blockEventPayload.TotalGasUsed-feeTranferEventPayload.GasConsumed, txEventPayload.GasConsumed)
				require.Empty(t, txEventPayload.ContractAddress)

				// append the state
				snapshot = snapshot.Append(state)

				// check coinbase balance
				coinbaseBalance = getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Equal(t, types.BalanceToBigInt(coinbaseBalance).Uint64(), txEventPayload.GasConsumed)

				// query the value
				code = []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)

						assert(res.status == EVM.Status.successful, message: "unexpected status")
						assert(res.errorCode == 0, message: "unexpected error code")
						
						return res
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.Nil(t, res.DeployedContractAddress)
				require.Equal(t, uint64(23_520), res.GasConsumed)
				require.Equal(t, uint64(23_520), res.MaxGasConsumed)
				require.Equal(t, num, new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	t.Run("testing EVM.run (failed)", func(t *testing.T) {
		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.failed, message: "unexpected status")
							// ExecutionErrCodeExecutionReverted
							assert(res.errorCode == %d, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
					types.ExecutionErrCodeExecutionReverted,
				))

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "storeButRevert", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				snapshot = snapshot.Append(state)

				// query the value
				code = []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.Equal(t, int64(0), new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	t.Run("testing EVM.run (with event emitted)", func(t *testing.T) {
		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "storeWithLog", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)

				require.NotEmpty(t, txEventPayload.Hash)

				var logs []*gethTypes.Log
				err = rlp.DecodeBytes(txEventPayload.Logs, &logs)
				require.NoError(t, err)
				require.Len(t, logs, 1)
				log := logs[0]
				last := log.Topics[len(log.Topics)-1] // last topic is the value set in the store method
				assert.Equal(t, num, last.Big().Int64())
			})
	})

	t.Run("testing EVM.run execution reverted with assert error", func(t *testing.T) {

		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.failed, message: "unexpected status")
							assert(res.errorCode == 306, message: "unexpected error code")
							assert(res.deployedContract == nil, message: "unexpected deployed contract")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "assertError"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(1),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// assert event fields are correct
				require.Len(t, output.Events, 2)
				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)
				require.NoError(t, err)

				assert.Equal(
					t,
					"execution reverted: Assert Error Message",
					txEventPayload.ErrorMessage,
				)
			},
		)
	})

	t.Run("testing EVM.run execution reverted with custom error", func(t *testing.T) {

		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.failed, message: "unexpected status")
							assert(res.errorCode == 306, message: "unexpected error code")
							assert(res.deployedContract == nil, message: "unexpected deployed contract")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "customError"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(1),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// assert event fields are correct
				require.Len(t, output.Events, 2)
				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)
				require.NoError(t, err)

				// Unlike assert errors, custom errors cannot be further examined
				// or ABI decoded, as we do not have access to the contract's ABI.
				assert.Equal(
					t,
					"execution reverted",
					txEventPayload.ErrorMessage,
				)
			},
		)
	})

	t.Run("testing EVM.run failed with gas limit validation error", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)
							assert(res.status == EVM.Status.invalid, message: "unexpected status")
							assert(res.errorCode == 100, message: "unexpected error code: \(res.errorCode)")
							assert(res.errorMessage == "transaction gas limit too high (cap: 16777216, tx: 16777220)")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(16_777_220), // max is 16,777,216
					big.NewInt(1),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// assert no events were produced from an invalid EVM transaction
				require.Len(t, output.Events, 0)
			})
	})

	t.Run("testing EVM.run with max gas limit cap", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)
							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code: \(res.errorCode)")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(16_777_216),
					big.NewInt(1),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)
				snapshot = snapshot.Append(state)

				// assert event fields are correct
				require.Len(t, output.Events, 2)
				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)
				require.NoError(t, err)

				// fee transfer event
				feeTransferEvent := output.Events[1]
				feeTranferEventPayload := TxEventToPayload(t, feeTransferEvent, sc.EVMContract.Address)
				require.NoError(t, err)
				require.Equal(t, uint16(types.ErrCodeNoError), feeTranferEventPayload.ErrorCode)
				require.Equal(t, uint16(1), feeTranferEventPayload.Index)
				require.Equal(t, uint64(21000), feeTranferEventPayload.GasConsumed)

				// commit block
				blockEventPayload, _ := callEVMHeartBeat(t,
					ctx,
					vm,
					snapshot,
				)

				require.NotEmpty(t, blockEventPayload.Hash)
				require.Equal(t, uint64(64785), blockEventPayload.TotalGasUsed)
				require.NotEmpty(t, blockEventPayload.Hash)

				txHashes := types.TransactionHashes{txEventPayload.Hash, feeTranferEventPayload.Hash}
				require.Equal(t,
					txHashes.RootHash(),
					blockEventPayload.TransactionHashRoot,
				)
				require.NotEmpty(t, blockEventPayload.ReceiptRoot)

				require.Equal(t, innerTxBytes, txEventPayload.Payload)
				require.Equal(t, uint16(types.ErrCodeNoError), txEventPayload.ErrorCode)
				require.Equal(t, uint16(0), txEventPayload.Index)
				require.Equal(t, blockEventPayload.Height, txEventPayload.BlockHeight)
				require.Equal(t, blockEventPayload.TotalGasUsed-feeTranferEventPayload.GasConsumed, txEventPayload.GasConsumed)
				require.Empty(t, txEventPayload.ContractAddress)
			})
	})
}

func TestEVMBatchRun(t *testing.T) {
	chain := flow.Emulator.Chain()

	// run a batch of valid transactions which update a value on the contract
	// after the batch is run check that the value updated on the contract matches
	// the last transaction update in the batch.
	t.Run("Batch run multiple valid transactions", func(t *testing.T) {
		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				batchRunCode := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(txs: [[UInt8]], coinbaseBytes: [UInt8; 20]) {
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let batchResults = EVM.batchRun(txs: txs, coinbase: coinbase)
							
							assert(batchResults.length == txs.length, message: "invalid result length")
							for res in batchResults {
								assert(res.status == EVM.Status.successful, message: "unexpected status")
								assert(res.errorCode == 0, message: "unexpected error code")
							}
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				batchCount := 5
				var storedValues []int64
				txBytes := make([]cadence.Value, batchCount)
				for i := 0; i < batchCount; i++ {
					num := int64(i)
					storedValues = append(storedValues, num)
					// prepare batch of transaction payloads
					tx := testAccount.PrepareSignAndEncodeTx(t,
						testContract.DeployedAt.ToCommon(),
						testContract.MakeCallData(t, "storeWithLog", big.NewInt(num)),
						big.NewInt(0),
						uint64(100_000),
						big.NewInt(1),
					)

					// build txs argument
					txBytes[i] = cadence.NewArray(
						unittest.BytesToCdcUInt8(tx),
					).WithType(stdlib.EVMTransactionBytesCadenceType)
				}

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(batchRunCode).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(txs)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(ctx, tx, snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// append the state
				snapshot = snapshot.Append(state)

				require.Len(t, output.Events, batchCount+1)
				txHashes := make(types.TransactionHashes, 0)
				totalGasUsed := uint64(0)
				for i, event := range output.Events {
					if i == batchCount { // skip last one
						continue
					}

					ev, err := ccf.Decode(nil, event.Payload)
					require.NoError(t, err)
					cadenceEvent, ok := ev.(cadence.Event)
					require.True(t, ok)

					event, err := events.DecodeTransactionEventPayload(cadenceEvent)
					require.NoError(t, err)

					txHashes = append(txHashes, event.Hash)
					var logs []*gethTypes.Log
					err = rlp.DecodeBytes(event.Logs, &logs)
					require.NoError(t, err)

					require.Len(t, logs, 1)

					log := logs[0]
					last := log.Topics[len(log.Topics)-1] // last topic is the value set in the store method
					assert.Equal(t, storedValues[i], last.Big().Int64())
					totalGasUsed += event.GasConsumed
				}

				// last event is fee transfer event
				feeTransferEvent := output.Events[batchCount]
				feeTranferEventPayload := TxEventToPayload(t, feeTransferEvent, sc.EVMContract.Address)
				require.NoError(t, err)
				require.Equal(t, uint16(types.ErrCodeNoError), feeTranferEventPayload.ErrorCode)
				require.Equal(t, uint16(batchCount), feeTranferEventPayload.Index)
				require.Equal(t, uint64(21000), feeTranferEventPayload.GasConsumed)
				txHashes = append(txHashes, feeTranferEventPayload.Hash)

				// check coinbase balance (note the gas price is 1)
				coinbaseBalance = getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Equal(t, types.BalanceToBigInt(coinbaseBalance).Uint64(), totalGasUsed)

				// commit block
				blockEventPayload, snapshot := callEVMHeartBeat(t,
					ctx,
					vm,
					snapshot)

				require.NotEmpty(t, blockEventPayload.Hash)
				require.Equal(t, uint64(176_513), blockEventPayload.TotalGasUsed)
				require.Equal(t,
					txHashes.RootHash(),
					blockEventPayload.TransactionHashRoot,
				)

				// retrieve the values
				retrieveCode := []byte(fmt.Sprintf(
					`
						import EVM from %s
						access(all)
						fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							return EVM.run(tx: tx, coinbase: coinbase)
						}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(retrieveCode).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				// make sure the retrieved value is the same as the last value
				// that was stored by transaction batch
				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.Equal(t, storedValues[len(storedValues)-1], new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	// run batch with one invalid transaction that has an invalid nonce
	// this should produce invalid result on that specific transaction
	// but other transaction should successfuly update the value on the contract
	t.Run("Batch run with one invalid transaction", func(t *testing.T) {
		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// we make transaction at specific index invalid to fail
				const failedTxIndex = 3
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				batchRunCode := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(txs: [[UInt8]], coinbaseBytes: [UInt8; 20]) {
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let batchResults = EVM.batchRun(txs: txs, coinbase: coinbase)
							
							assert(batchResults.length == txs.length, message: "invalid result length")
							for i, res in batchResults {
								if i != %d {
									assert(res.status == EVM.Status.successful, message: "unexpected status")
									assert(res.errorCode == 0, message: "unexpected error code")
								} else {
									assert(res.status == EVM.Status.invalid, message: "unexpected status")
									assert(res.errorCode == 201, message: "unexpected error code")
								}
							}
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
					failedTxIndex,
				))

				batchCount := 5
				var num int64
				txBytes := make([]cadence.Value, batchCount)
				for i := 0; i < batchCount; i++ {
					num = int64(i)

					if i == failedTxIndex {
						// make one transaction in the batch have an invalid nonce
						testAccount.SetNonce(testAccount.Nonce() - 1)
					}
					// prepare batch of transaction payloads
					tx := testAccount.PrepareSignAndEncodeTx(t,
						testContract.DeployedAt.ToCommon(),
						testContract.MakeCallData(t, "store", big.NewInt(num)),
						big.NewInt(0),
						uint64(100_000),
						big.NewInt(0),
					)

					// build txs argument
					txBytes[i] = cadence.NewArray(
						unittest.BytesToCdcUInt8(tx),
					).WithType(stdlib.EVMTransactionBytesCadenceType)
				}

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(batchRunCode).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(txs)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(ctx, tx, snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// append the state
				snapshot = snapshot.Append(state)

				// retrieve the values
				retrieveCode := []byte(fmt.Sprintf(
					`
						import EVM from %s
						access(all)
						fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							return EVM.run(tx: tx, coinbase: coinbase)
						}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(retrieveCode).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				// make sure the retrieved value is the same as the last value
				// that was stored by transaction batch
				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.Equal(t, num, new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	// fail every other transaction with gas set too low for execution to succeed
	// but high enough to pass intristic gas check, then check the updated values on the
	// contract to match the last successful transaction execution
	t.Run("Batch run with with failed transactions", func(t *testing.T) {
		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				batchRunCode := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(txs: [[UInt8]], coinbaseBytes: [UInt8; 20]) {
						execute {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let batchResults = EVM.batchRun(txs: txs, coinbase: coinbase)

							log("results")
							log(batchResults)
							assert(batchResults.length == txs.length, message: "invalid result length")

							for i, res in batchResults {
								if i %% 2 != 0 {
									assert(res.status == EVM.Status.successful, message: "unexpected success status")
									assert(res.errorCode == 0, message: "unexpected error code")
									assert(res.errorMessage == "", message: "unexpected error msg")
								} else {
									assert(res.status == EVM.Status.failed, message: "unexpected failed status")
									assert(res.errorCode == 400, message: "unexpected error code")
								}
							}
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				batchCount := 6
				var num int64
				txBytes := make([]cadence.Value, batchCount)
				for i := 0; i < batchCount; i++ {
					gas := uint64(100_000)
					if i%2 == 0 {
						// fail with too low gas limit
						gas = 22_000
					} else {
						// update number with only valid transactions
						num = int64(i)
					}

					// prepare batch of transaction payloads
					tx := testAccount.PrepareSignAndEncodeTx(t,
						testContract.DeployedAt.ToCommon(),
						testContract.MakeCallData(t, "store", big.NewInt(num)),
						big.NewInt(0),
						gas,
						big.NewInt(0),
					)

					// build txs argument
					txBytes[i] = cadence.NewArray(
						unittest.BytesToCdcUInt8(tx),
					).WithType(stdlib.EVMTransactionBytesCadenceType)
				}

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(batchRunCode).
					SetPayer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(txs)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(ctx, tx, snapshot)

				require.NoError(t, err)
				require.NoError(t, output.Err)
				//require.NotEmpty(t, state.WriteSet)

				// append the state
				snapshot = snapshot.Append(state)

				// retrieve the values
				retrieveCode := []byte(fmt.Sprintf(
					`
						import EVM from %s
						access(all)
						fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							return EVM.run(tx: tx, coinbase: coinbase)
						}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(retrieveCode).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				// make sure the retrieved value is the same as the last value
				// that was stored by transaction batch
				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Empty(t, res.ErrorMessage)
				require.Equal(t, num, new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	// run a batch of two transactions. The sum of their gas usage would overflow an uint46
	// so the batch run should fail with an overflow error.
	t.Run("Batch run evm gas overflow", func(t *testing.T) {
		t.Parallel()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				batchRunCode := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(txs: [[UInt8]], coinbaseBytes: [UInt8; 20]) {
						execute {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let batchResults = EVM.batchRun(txs: txs, coinbase: coinbase)
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				batchCount := 2
				txBytes := make([]cadence.Value, batchCount)

				tx := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "storeWithLog", big.NewInt(0)),
					big.NewInt(0),
					uint64(200_000),
					big.NewInt(1),
				)

				txBytes[0] = cadence.NewArray(
					unittest.BytesToCdcUInt8(tx),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				tx = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "storeWithLog", big.NewInt(1)),
					big.NewInt(0),
					math.MaxUint64-uint64(100_000),
					big.NewInt(1),
				)

				txBytes[1] = cadence.NewArray(
					unittest.BytesToCdcUInt8(tx),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(batchRunCode).
					SetPayer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(txs)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				state, output, err := vm.Run(ctx, fvm.Transaction(txBody, 0), snapshot)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.ErrorContains(t, output.Err, "insufficient computation")
				require.Empty(t, state.WriteSet)
			})
	})
}

func TestEVMBlockData(t *testing.T) {
	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {

			// query the block timestamp
			code := []byte(fmt.Sprintf(
				`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
				`,
				sc.EVMContract.Address.HexWithPrefix(),
			))

			innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
				testContract.DeployedAt.ToCommon(),
				testContract.MakeCallData(t, "blockTime"),
				big.NewInt(0),
				uint64(100_000),
				big.NewInt(0),
			)

			coinbase := cadence.NewArray(
				unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType)

			innerTx := cadence.NewArray(
				unittest.BytesToCdcUInt8(innerTxBytes),
			).WithType(stdlib.EVMTransactionBytesCadenceType)

			script := fvm.Script(code).WithArguments(
				json.MustEncode(innerTx),
				json.MustEncode(coinbase),
			)

			_, output, err := vm.Run(
				ctx,
				script,
				snapshot)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
			require.NoError(t, err)
			require.Equal(t, types.StatusSuccessful, res.Status)
			require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
			require.Empty(t, res.ErrorMessage)
			require.Equal(t, ctx.BlockHeader.Timestamp/1000, new(big.Int).SetBytes(res.ReturnedData).Uint64()) // EVM reports block time as Unix time in seconds

		})
}

func TestEVMAddressDeposit(t *testing.T) {

	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {

			code := []byte(fmt.Sprintf(
				`
				import EVM from %s
				import FlowToken from %s

				transaction(addr: [UInt8; 20]) {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage
							.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!

						let minter <- admin.createNewMinter(allowedAmount: 1.0)
						let vault <- minter.mintTokens(amount: 1.0)
						destroy minter

						let address = EVM.EVMAddress(bytes: addr)
						address.deposit(from: <-vault)
					}
				}
			`,
				sc.EVMContract.Address.HexWithPrefix(),
				sc.FlowToken.Address.HexWithPrefix(),
			))

			addr := RandomAddress(t)

			txBody, err := flow.NewTransactionBodyBuilder().
				SetScript(code).
				SetPayer(sc.FlowServiceAccount.Address).
				AddAuthorizer(sc.FlowServiceAccount.Address).
				AddArgument(json.MustEncode(cadence.NewArray(
					unittest.BytesToCdcUInt8(addr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType))).
				Build()
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			execSnap, output, err := vm.Run(
				ctx,
				tx,
				snapshot)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshot = snapshot.Append(execSnap)

			expectedBalance := types.OneFlowBalance()
			bal := getEVMAccountBalance(t, ctx, vm, snapshot, addr)
			require.Equal(t, expectedBalance, bal)

			// tx executed event
			txEvent := output.Events[2]
			txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)

			// deposit event
			depositEvent := output.Events[3]
			depEv, err := events.FlowEventToCadenceEvent(depositEvent)
			require.NoError(t, err)

			depEvPayload, err := events.DecodeFLOWTokensDepositedEventPayload(depEv)
			require.NoError(t, err)

			require.Equal(t, types.OneFlow(), depEvPayload.BalanceAfterInAttoFlow.Value)

			// commit block
			blockEventPayload, _ := callEVMHeartBeat(t,
				ctx,
				vm,
				snapshot)

			require.NotEmpty(t, blockEventPayload.Hash)
			require.Equal(t, uint64(21000), blockEventPayload.TotalGasUsed)

			txHashes := types.TransactionHashes{txEventPayload.Hash}
			require.Equal(t,
				txHashes.RootHash(),
				blockEventPayload.TransactionHashRoot,
			)
		})
}

func TestCOAAddressDeposit(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {
			code := []byte(fmt.Sprintf(
				`
				import EVM from %s
				import FlowToken from %s

				access(all)
				fun main() {
					let admin = getAuthAccount<auth(BorrowValue) &Account>(%s)
						.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
					let minter <- admin.createNewMinter(allowedAmount: 1.23)
					let vault <- minter.mintTokens(amount: 1.23)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)
					destroy cadenceOwnedAccount
				}
                `,
				sc.EVMContract.Address.HexWithPrefix(),
				sc.FlowToken.Address.HexWithPrefix(),
				sc.FlowServiceAccount.Address.HexWithPrefix(),
			))

			script := fvm.Script(code)

			_, output, err := vm.Run(
				ctx,
				script,
				snapshot)
			require.NoError(t, err)
			require.NoError(t, output.Err)

		})
}

func TestCadenceOwnedAccountFunctionalities(t *testing.T) {
	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	t.Run("test coa setup", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// create a flow account
				flowAccount, _, snapshot := createAndFundFlowAccount(
					t,
					ctx,
					vm,
					snapshot,
				)

				var coaAddress types.Address

				initNonce := uint64(1)
				// 10 Flow in UFix64
				initBalanceInUFix64 := uint64(1_000_000_000)
				initBalance := types.NewBalanceFromUFix64(cadence.UFix64(initBalanceInUFix64))

				coaAddress, snapshot = setupCOA(
					t,
					ctx,
					vm,
					snapshot,
					flowAccount,
					initBalanceInUFix64)

				bal := getEVMAccountBalance(
					t,
					ctx,
					vm,
					snapshot,
					coaAddress)
				require.Equal(t, initBalance, bal)

				nonce := getEVMAccountNonce(
					t,
					ctx,
					vm,
					snapshot,
					coaAddress)
				require.Equal(t, initNonce, nonce)
			})
	})

	t.Run("test coa withdraw with rounding error", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s

				transaction() {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage.borrow<&FlowToken.Administrator>(
							from: /storage/flowTokenAdmin
						)!

						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter

						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)

						// since 1e10 attoFlow is the minimum withdrawable amount,
						// verify any amount below 1e10 can not be withdrawn.
						let bal = EVM.Balance(attoflow: 9999999999)
						let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
						let balance = vault2.balance
						destroy cadenceOwnedAccount
						destroy vault2
					}
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)
				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.ErrorContains(
					t,
					output.Err,
					types.ErrWithdrawBalanceRounding.Error(),
				)
			},
		)
	})

	t.Run("test coa withdraw with minimum allowed transfer", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s
				transaction() {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage.borrow<&FlowToken.Administrator>(
							from: /storage/flowTokenAdmin
						)!

						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter

						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)

						let bal = EVM.Balance(attoflow: 10000000000)
						let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
						let balance = vault2.balance
						assert(balance == 0.00000001, message: "mismatching vault balance")
						destroy cadenceOwnedAccount
						destroy vault2
					}
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)
				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				withdrawEvent := output.Events[7]

				ev, err := events.FlowEventToCadenceEvent(withdrawEvent)
				require.NoError(t, err)

				evPayload, err := events.DecodeFLOWTokensWithdrawnEventPayload(ev)
				require.NoError(t, err)

				// 2.34000000 - 0.00000001 = 2.33999999
				expectedBalanceAfterWithdraw := big.NewInt(2_339_999_990_000_000_000)
				require.Equal(t, expectedBalanceAfterWithdraw, evPayload.BalanceAfterInAttoFlow.Value)
			},
		)
	})

	t.Run("test coa withdraw with success", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s
				transaction() {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage.borrow<&FlowToken.Administrator>(
							from: /storage/flowTokenAdmin
						)!

						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter

						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)

						let bal = EVM.Balance(attoflow: 1230000780000000000)
						let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
						let balance = vault2.balance
						assert(balance == 1.23000078, message: "mismatching vault balance")
						destroy cadenceOwnedAccount
						destroy vault2
					}
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)
				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				withdrawEvent := output.Events[7]

				ev, err := events.FlowEventToCadenceEvent(withdrawEvent)
				require.NoError(t, err)

				evPayload, err := events.DecodeFLOWTokensWithdrawnEventPayload(ev)
				require.NoError(t, err)

				// 2.34000000 - 1.23000078 = 1.10999922
				expectedBalanceAfterWithdraw := big.NewInt(1_109_999_220_000_000_000)
				require.Equal(t, expectedBalanceAfterWithdraw, evPayload.BalanceAfterInAttoFlow.Value)
			},
		)
	})

	t.Run("test coa withdraw with fraction-only amount", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s
				transaction() {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage.borrow<&FlowToken.Administrator>(
							from: /storage/flowTokenAdmin
						)!

						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter

						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)

						let bal = EVM.Balance(attoflow: 230050780900000000)
						let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
						let balance = vault2.balance
						assert(balance == 0.23005078, message: "mismatching vault balance")
						destroy cadenceOwnedAccount
						destroy vault2
					}
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)
				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				withdrawEvent := output.Events[7]

				ev, err := events.FlowEventToCadenceEvent(withdrawEvent)
				require.NoError(t, err)

				evPayload, err := events.DecodeFLOWTokensWithdrawnEventPayload(ev)
				require.NoError(t, err)

				// 2.34000000 - 0.2300078 = 2.10994922
				expectedBalanceAfterWithdraw := big.NewInt(2_109_949_220_000_000_000)
				require.Equal(t, expectedBalanceAfterWithdraw, evPayload.BalanceAfterInAttoFlow.Value)
			},
		)
	})

	t.Run("test coa withdraw with value bigger than uint256", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s
				transaction() {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage.borrow<&FlowToken.Administrator>(
							from: /storage/flowTokenAdmin
						)!

						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter

						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)

						let bal = EVM.Balance(attoflow: 115792089237316195423570985008687907853269984665640564039457584007913129639936)
						let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
						let balance = vault2.balance
						assert(balance == 1.23000078, message: "mismatching vault balance")
						destroy cadenceOwnedAccount
						destroy vault2
					}
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)
				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.Error(t, output.Err)
				require.ErrorContains(
					t,
					output.Err,
					types.ErrInvalidBalance.Error(),
				)
			},
		)
	})

	t.Run("test coa withdraw with remainder truncation", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s
				transaction() {
					prepare(account: auth(BorrowValue) &Account) {
						let admin = account.storage.borrow<&FlowToken.Administrator>(
							from: /storage/flowTokenAdmin
						)!

						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter

						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)

						let bal = EVM.Balance(attoflow: 1230000789912345678)
						let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
						let balance = vault2.balance
						assert(balance == 1.23000078, message: "mismatching vault balance")
						destroy cadenceOwnedAccount
						destroy vault2
					}
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)
				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				withdrawEvent := output.Events[7]

				ev, err := events.FlowEventToCadenceEvent(withdrawEvent)
				require.NoError(t, err)

				evPayload, err := events.DecodeFLOWTokensWithdrawnEventPayload(ev)
				require.NoError(t, err)

				// 2.34000000 - 1.23000078 = 1.10999922
				expectedBalanceAfterWithdraw := big.NewInt(1_109_999_220_000_000_000)
				require.Equal(t, expectedBalanceAfterWithdraw, evPayload.BalanceAfterInAttoFlow.Value)
			},
		)
	})

	t.Run("test coa transfer", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s

				access(all)
				fun main(address: [UInt8; 20]): UFix64 {
					let admin = getAuthAccount<auth(BorrowValue) &Account>(%s)
						.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!

					let minter <- admin.createNewMinter(allowedAmount: 2.34)
					let vault <- minter.mintTokens(amount: 2.34)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)

					let bal = EVM.Balance(attoflow: 0)
					bal.setFLOW(flow: 1.23)

					let recipientEVMAddress = EVM.EVMAddress(bytes: address)

					let res = cadenceOwnedAccount.call(
						to: recipientEVMAddress,
						data: [],
						gasLimit: 100_000,
						value: bal,
					)

					assert(res.status == EVM.Status.successful, message: "transfer call was not successful")

					destroy cadenceOwnedAccount
					return recipientEVMAddress.balance().inFLOW()
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(),
				))

				addr := cadence.NewArray(
					unittest.BytesToCdcUInt8(RandomAddress(t).Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(addr),
				)

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				require.Equal(t, uint64(123000000), uint64(output.Value.(cadence.UFix64)))
			})
	})

	t.Run("test coa deposit and withdraw in a single transaction", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
				import EVM from %s
				import FlowToken from %s

				access(all)
				fun main(): UFix64 {
					let admin = getAuthAccount<auth(Storage) &Account>(%s)
						.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!

					let minter <- admin.createNewMinter(allowedAmount: 2.34)
					let vault <- minter.mintTokens(amount: 2.34)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)

					let bal = EVM.Balance(attoflow: 0)
					bal.setFLOW(flow: 1.23)
					let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
					let balance = vault2.balance
					destroy cadenceOwnedAccount
					destroy vault2

					return balance
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(),
				))

				script := fvm.Script(code)

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})

	t.Run("test coa deploy", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					import FlowToken from %s
	
					access(all)
					fun main(code: [UInt8]): EVM.Result {
						let admin = getAuthAccount<auth(Storage) &Account>(%s)
							.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter
	
						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)
	
						let res = cadenceOwnedAccount.deploy(
							code: code,
							gasLimit: 2_000_000,
							value: EVM.Balance(attoflow: 1230000000000000000)
						)
						destroy cadenceOwnedAccount
						return res
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(),
				))

				script := fvm.Script(code).
					WithArguments(json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testContract.ByteCode),
						).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
					))

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.NotNil(t, res.DeployedContractAddress)
				// we strip away first few bytes because they contain deploy code
				require.Equal(t, testContract.ByteCode[17:], []byte(res.ReturnedData))
			})
	})

	t.Run("test coa dryCall", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: auth(Storage) &Account ) {
							let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
							account.storage.save(<- cadenceOwnedAccount, to: /storage/evmCOA)

							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(42)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				assert.Len(t, output.Events, 3)
				assert.Len(t, state.UpdatedRegisterIDs(), 13)
				assert.Equal(
					t,
					flow.EventType("A.f8d6e0586b0a20c7.EVM.TransactionExecuted"),
					output.Events[0].Type,
				)
				assert.Equal(
					t,
					flow.EventType("A.f8d6e0586b0a20c7.EVM.CadenceOwnedAccountCreated"),
					output.Events[1].Type,
				)
				assert.Equal(
					t,
					flow.EventType("A.f8d6e0586b0a20c7.EVM.TransactionExecuted"),
					output.Events[2].Type,
				)
				snapshot = snapshot.Append(state)

				code = []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(data: [UInt8], to: String, gasLimit: UInt64, value: UInt){
						prepare(account: auth(Storage) &Account) {
							let coa = account.storage.borrow<&EVM.CadenceOwnedAccount>(
								from: /storage/evmCOA
							) ?? panic("could not borrow COA reference!")
							let res = coa.dryCall(
								to: EVM.addressFromString(to),
								data: data,
								gasLimit: gasLimit,
								value: EVM.Balance(attoflow: value)
							)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")

							let values = EVM.decodeABI(types: [Type<UInt256>()], data: res.data)
							assert(values.length == 1)

							let number = values[0] as! UInt256
							assert(number == 42, message: String.encodeHex(res.data))
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				data := json.MustEncode(
					cadence.NewArray(
						unittest.BytesToCdcUInt8(testContract.MakeCallData(t, "retrieve")),
					).WithType(stdlib.EVMTransactionBytesCadenceType),
				)
				toAddress, err := cadence.NewString(testContract.DeployedAt.ToCommon().Hex())
				require.NoError(t, err)
				to := json.MustEncode(toAddress)

				txBody, err = flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(data).
					AddArgument(to).
					AddArgument(json.MustEncode(cadence.NewUInt64(50_000))).
					AddArgument(json.MustEncode(cadence.NewUInt(0))).
					Build()
				require.NoError(t, err)

				tx = fvm.Transaction(txBody, 0)

				state, output, err = vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				assert.Len(t, output.Events, 0)
				assert.Len(t, state.UpdatedRegisterIDs(), 0)
			})
	})

	t.Run("test coa deploy with max gas limit cap", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					import FlowToken from %s
					access(all)
					fun main(code: [UInt8]): EVM.Result {
						let admin = getAuthAccount<auth(Storage) &Account>(%s)
							.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter
						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)
						let res = cadenceOwnedAccount.deploy(
							code: code,
							gasLimit: 16_777_216,
							value: EVM.Balance(attoflow: 1230000000000000000)
						)
						destroy cadenceOwnedAccount
						return res
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(),
				))

				script := fvm.Script(code).
					WithArguments(json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testContract.ByteCode),
						).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
					))

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.NotNil(t, res.DeployedContractAddress)
				// we strip away first few bytes because they contain deploy code
				require.Equal(t, testContract.ByteCode[17:], []byte(res.ReturnedData))
			})
	})

	t.Run("test coa deploy with bigger than max gas limit cap", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					import FlowToken from %s
					access(all)
					fun main(code: [UInt8]): EVM.Result {
						let admin = getAuthAccount<auth(Storage) &Account>(%s)
							.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
						let minter <- admin.createNewMinter(allowedAmount: 2.34)
						let vault <- minter.mintTokens(amount: 2.34)
						destroy minter
						let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
						cadenceOwnedAccount.deposit(from: <-vault)
						let res = cadenceOwnedAccount.deploy(
							code: code,
							gasLimit: 16_777_226,
							value: EVM.Balance(attoflow: 1230000000000000000)
						)
						destroy cadenceOwnedAccount
						return res
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(),
				))

				script := fvm.Script(code).
					WithArguments(json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testContract.ByteCode),
						).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
					))

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusInvalid, res.Status)
				require.Equal(t, types.ValidationErrCodeMisc, res.ErrorCode)
				require.Equal(
					t,
					"transaction gas limit too high (cap: 16777216, tx: 16777226)",
					res.ErrorMessage,
				)
				require.Nil(t, res.DeployedContractAddress)
				// we strip away first few bytes because they contain deploy code
				require.Empty(t, []byte(res.ReturnedData))
			})
	})
}

func TestDryRun(t *testing.T) {
	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	evmAddress := sc.EVMContract.Address.HexWithPrefix()

	dryRunTx := func(
		t *testing.T,
		tx *gethTypes.Transaction,
		ctx fvm.Context,
		vm fvm.VM,
		snapshot snapshot.SnapshotTree,
	) *types.ResultSummary {
		code := []byte(fmt.Sprintf(`
			import EVM from %s

			access(all)
			fun main(tx: [UInt8]): EVM.Result {
				return EVM.dryRun(
					tx: tx, 
					from: EVM.EVMAddress(bytes: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19])
				)
			}`,
			evmAddress,
		))

		innerTxBytes, err := tx.MarshalBinary()
		require.NoError(t, err)

		script := fvm.Script(code).WithArguments(
			json.MustEncode(
				cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType),
			),
		)
		_, output, err := vm.Run(
			ctx,
			script,
			snapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		result, err := impl.ResultSummaryFromEVMResultValue(output.Value)
		require.NoError(t, err)
		return result
	}

	// this test checks that gas limit is correctly used and gas usage correctly reported
	t.Run("test dry run storing a value with different gas limits", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				data := testContract.MakeCallData(t, "store", big.NewInt(1337))

				// EVM.dryRun must not be limited by the `gethParams.MaxTxGas`
				limit := gethParams.MaxTxGas + 1_000
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result := dryRunTx(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))
				require.Less(t, result.GasConsumed, limit)

				// gas limit too low, but still bigger than intrinsic gas value
				limit = uint64(24_216)
				tx = gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result = dryRunTx(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ExecutionErrCodeOutOfGas, result.ErrorCode)
				require.Equal(t, types.StatusFailed, result.Status)
				require.Equal(t, result.GasConsumed, limit) // burn it all!!!
			})
	})

	t.Run("test dry run store current value", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				data := testContract.MakeCallData(t, "store", big.NewInt(0))
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
					data,
				)
				dryRunResult := dryRunTx(t, tx, ctx, vm, snapshot)

				require.Equal(t, types.ErrCodeNoError, dryRunResult.ErrorCode)
				require.Equal(t, types.StatusSuccessful, dryRunResult.Status)
				require.Greater(t, dryRunResult.GasConsumed, uint64(0))

				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					evmAddress,
				))

				// Use the gas estimation from Evm.dryRun with some buffer
				gasLimit := dryRunResult.GasConsumed + gethParams.SstoreSentryGasEIP2200
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					data,
					big.NewInt(0),
					gasLimit,
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, res.GasConsumed, dryRunResult.GasConsumed)
			})
	})

	t.Run("test dry run store new value", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				data := testContract.MakeCallData(t, "store", big.NewInt(100))
				tx1 := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
					data,
				)
				dryRunResult := dryRunTx(t, tx1, ctx, vm, snapshot)

				require.Equal(t, types.ErrCodeNoError, dryRunResult.ErrorCode)
				require.Equal(t, types.StatusSuccessful, dryRunResult.Status)
				require.Greater(t, dryRunResult.GasConsumed, uint64(0))

				code = []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					evmAddress,
				))

				// Decrease nonce because we are Cadence using scripts, and not
				// transactions, which means that no state change is happening.
				testAccount.SetNonce(testAccount.Nonce() - 1)
				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					data,
					big.NewInt(0),
					dryRunResult.GasConsumed, // use the gas estimation from Evm.dryRun
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase = cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, res.GasConsumed, dryRunResult.GasConsumed)
			})
	})

	t.Run("test dry run clear current value", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(100)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				snapshot = snapshot.Append(state)

				data := testContract.MakeCallData(t, "store", big.NewInt(0))
				tx1 := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
					data,
				)
				dryRunResult := dryRunTx(t, tx1, ctx, vm, snapshot)

				require.Equal(t, types.ErrCodeNoError, dryRunResult.ErrorCode)
				require.Equal(t, types.StatusSuccessful, dryRunResult.Status)
				require.Greater(t, dryRunResult.GasConsumed, uint64(0))

				code = []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					evmAddress,
				))

				// use the gas estimation from Evm.dryRun with the necessary buffer gas
				gasLimit := dryRunResult.GasConsumed + gethParams.SstoreClearsScheduleRefundEIP3529
				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					data,
					big.NewInt(0),
					gasLimit,
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase = cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, res.GasConsumed, dryRunResult.GasConsumed)
			})
	})

	// this test makes sure the dry-run that updates the value on the contract
	// doesn't persist the change, and after when the value is read it isn't updated.
	t.Run("test dry run for any side-effects", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				updatedValue := int64(1337)
				data := testContract.MakeCallData(t, "store", big.NewInt(updatedValue))
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					uint64(1000000),
					big.NewInt(0),
					data,
				)

				result := dryRunTx(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))

				// query the value make sure it's not updated
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					evmAddress,
				))

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				// make sure the value we used in the dry-run is not the same as the value stored in contract
				require.NotEqual(t, updatedValue, new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	t.Run("test dry run contract deployment", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				tx := gethTypes.NewContractCreation(
					0,
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
					testContract.ByteCode,
				)

				result := dryRunTx(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))
				require.NotNil(t, result.ReturnedData)
				require.NotNil(t, result.DeployedContractAddress)
				require.NotEmpty(t, result.DeployedContractAddress.String())
			})
	})

	t.Run("test dry run validation error", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				tx := gethTypes.NewContractCreation(
					0,
					big.NewInt(100), // more than available
					uint64(1000000),
					big.NewInt(0),
					nil,
				)

				result := dryRunTx(t, tx, ctx, vm, snapshot)
				assert.Equal(t, types.ValidationErrCodeInsufficientFunds, result.ErrorCode)
				assert.Equal(t, types.StatusInvalid, result.Status)
				assert.Equal(t, types.InvalidTransactionGasCost, int(result.GasConsumed))
			})
	})
}

func TestDryCall(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	evmAddress := sc.EVMContract.Address.HexWithPrefix()

	dryCall := func(
		t *testing.T,
		tx *gethTypes.Transaction,
		ctx fvm.Context,
		vm fvm.VM,
		snapshot snapshot.SnapshotTree,
	) (*types.ResultSummary, *snapshot.ExecutionSnapshot) {
		code := []byte(fmt.Sprintf(`
			import EVM from %s

			access(all)
			fun main(data: [UInt8], to: String, gasLimit: UInt64, value: UInt): EVM.Result {
				return EVM.dryCall(
					from: EVM.EVMAddress(bytes: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 0, 0, 0, 0, 0, 0, 15]),
					to: EVM.addressFromString(to),
					data: data,
					gasLimit: gasLimit,
					value: EVM.Balance(attoflow: value)
				)
			}`,
			evmAddress,
		))

		require.NotNil(t, tx.To())
		to := tx.To().Hex()
		toAddress, err := cadence.NewString(to)
		require.NoError(t, err)

		script := fvm.Script(code).WithArguments(
			json.MustEncode(
				cadence.NewArray(
					unittest.BytesToCdcUInt8(tx.Data()),
				).WithType(stdlib.EVMTransactionBytesCadenceType),
			),
			json.MustEncode(toAddress),
			json.MustEncode(cadence.NewUInt64(tx.Gas())),
			json.MustEncode(cadence.NewUInt(uint(tx.Value().Uint64()))),
		)
		execSnapshot, output, err := vm.Run(
			ctx,
			script,
			snapshot,
		)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		require.Len(t, output.Events, 0)

		result, err := impl.ResultSummaryFromEVMResultValue(output.Value)
		require.NoError(t, err)
		return result, execSnapshot
	}

	// this test checks that gas limit is correctly used and gas usage correctly reported
	t.Run("test dryCall with different gas limits", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				data := testContract.MakeCallData(t, "store", big.NewInt(1337))

				limit := uint64(50_000)
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result, _ := dryCall(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))
				require.Less(t, result.GasConsumed, limit)

				// gas limit too low, but still bigger than intrinsic gas value
				limit = uint64(24_216)
				tx = gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result, _ = dryCall(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ExecutionErrCodeOutOfGas, result.ErrorCode)
				require.Equal(t, types.StatusFailed, result.Status)
				require.Equal(t, result.GasConsumed, limit)

				// EVM.dryCall must not be limited to `gethParams.MaxTxGas`
				limit = gethParams.MaxTxGas + 1_000
				tx = gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result, _ = dryCall(t, tx, ctx, vm, snapshot)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))
				require.Less(t, result.GasConsumed, limit)
			})
	})

	t.Run("test dryCall does not form EVM transactions", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(42)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(50_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				assert.Len(t, output.Events, 1)
				assert.Len(t, state.UpdatedRegisterIDs(), 4)
				assert.Equal(
					t,
					flow.EventType("A.f8d6e0586b0a20c7.EVM.TransactionExecuted"),
					output.Events[0].Type,
				)
				snapshot = snapshot.Append(state)

				code = []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(data: [UInt8], to: String, gasLimit: UInt64, value: UInt){
						prepare(account: &Account) {
							let res = EVM.dryCall(
								from: EVM.EVMAddress(bytes: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 0, 0, 0, 0, 0, 0, 15]),
								to: EVM.addressFromString(to),
								data: data,
								gasLimit: gasLimit,
								value: EVM.Balance(attoflow: value)
							)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")

							let values = EVM.decodeABI(types: [Type<UInt256>()], data: res.data)
							assert(values.length == 1)

							let number = values[0] as! UInt256
							assert(number == 42, message: String.encodeHex(res.data))
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				data := json.MustEncode(
					cadence.NewArray(
						unittest.BytesToCdcUInt8(testContract.MakeCallData(t, "retrieve")),
					).WithType(stdlib.EVMTransactionBytesCadenceType),
				)
				toAddress, err := cadence.NewString(testContract.DeployedAt.ToCommon().Hex())
				require.NoError(t, err)
				to := json.MustEncode(toAddress)

				txBody, err = flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(data).
					AddArgument(to).
					AddArgument(json.MustEncode(cadence.NewUInt64(50_000))).
					AddArgument(json.MustEncode(cadence.NewUInt(0))).
					Build()
				require.NoError(t, err)

				tx = fvm.Transaction(txBody, 0)

				state, output, err = vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				assert.Len(t, output.Events, 0)
				assert.Len(t, state.UpdatedRegisterIDs(), 0)
			})
	})

	// this test makes sure the dryCall that updates the value on the contract
	// doesn't persist the change, and after when the value is read it isn't updated.
	t.Run("test dryCall has no side-effects", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				updatedValue := int64(1337)
				data := testContract.MakeCallData(t, "store", big.NewInt(updatedValue))
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					uint64(1000000),
					big.NewInt(0),
					data,
				)

				result, state := dryCall(t, tx, ctx, vm, snapshot)
				require.Len(t, state.UpdatedRegisterIDs(), 0)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))

				// query the value make sure it's not updated
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					evmAddress,
				))

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				state, output, err := vm.Run(
					ctx,
					script,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.Len(t, state.UpdatedRegisterIDs(), 0)

				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				// make sure the value we used in the dryCall is not the same as the value stored in contract
				require.NotEqual(t, updatedValue, new(big.Int).SetBytes(res.ReturnedData).Int64())
			})
	})

	t.Run("test dryCall validation error", func(t *testing.T) {
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				data := testContract.MakeCallData(t, "store", big.NewInt(10337))
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(1000), // more than available
					uint64(35_000),
					big.NewInt(0),
					data,
				)

				result, _ := dryCall(t, tx, ctx, vm, snapshot)
				assert.Equal(t, types.ValidationErrCodeInsufficientFunds, result.ErrorCode)
				assert.Equal(t, types.StatusInvalid, result.Status)
				assert.Equal(t, types.InvalidTransactionGasCost, int(result.GasConsumed))

				// random function selector
				data = []byte{254, 234, 101, 199}
				tx = gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					uint64(25_000),
					big.NewInt(0),
					data,
				)

				result, _ = dryCall(t, tx, ctx, vm, snapshot)
				assert.Equal(t, types.ExecutionErrCodeExecutionReverted, result.ErrorCode)
				assert.Equal(t, types.StatusFailed, result.Status)
				assert.Equal(t, uint64(21331), result.GasConsumed)
			})
	})
}

func TestCadenceArch(t *testing.T) {
	t.Parallel()

	t.Run("testing calling Cadence arch - flow block height (happy case)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)
						assert(res.status == EVM.Status.successful, message: "test failed: ".concat(res.errorCode.toString()))
					}
                    `,
					sc.EVMContract.Address.HexWithPrefix(),
				))
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToFlowBlockHeight", ctx.BlockHeader.Height),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				script := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
						).WithType(stdlib.EVMAddressBytesCadenceType),
					),
				)
				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})

	t.Run("testing calling Cadence arch - revertible random", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): [UInt8] {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)
						assert(res.status == EVM.Status.successful, message: "test failed: ".concat(res.errorCode.toString()))
						return res.data
					}
                    `,
					sc.EVMContract.Address.HexWithPrefix(),
				))
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToRevertibleRandom"),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				script := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
						).WithType(stdlib.EVMAddressBytesCadenceType),
					),
				)
				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res := make([]byte, 8)
				vals := output.Value.(cadence.Array).Values
				vals = vals[len(vals)-8:] // only last 8 bytes is the value
				for i := range res {
					res[i] = byte(vals[i].(cadence.UInt8))
				}

				actualRand := binary.BigEndian.Uint64(res)
				// because PRG uses script ID and random source we can not predict the random
				// we can set the random source but since script ID is generated by hashing
				// script and args, and since arg is a signed transaction which always changes
				// we can't fix the value
				require.Greater(t, actualRand, uint64(0))
			})
	})

	t.Run("testing calling Cadence arch - random source (happy case)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				entropy := []byte{13, 37}
				// coresponding out to the above entropy
				source := []byte{0x5b, 0xa1, 0xce, 0xab, 0x64, 0x11, 0x8d, 0x2c, 0xd8, 0xae, 0x8c, 0xbb, 0xf7, 0x50, 0x5e, 0xf5, 0xdf, 0xad, 0xfc, 0xf7, 0x2d, 0x3a, 0x46, 0x78, 0xd5, 0xe5, 0x1d, 0xb7, 0xf2, 0xb8, 0xe5, 0xd6}

				// we must record a new heartbeat with a fixed block, we manually execute a transaction to do so,
				// since doing this automatically would require a block computer and whole execution setup
				height := uint64(1)
				block1 := unittest.BlockFixture(
					unittest.Block.WithHeight(height),
				)
				ctx.BlockHeader = block1.ToHeader()
				ctx.EntropyProvider = testutil.EntropyProviderFixture(entropy) // fix the entropy

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript([]byte(fmt.Sprintf(`
						import RandomBeaconHistory from %s

						transaction {
							prepare(serviceAccount: auth(Capabilities, Storage) &Account) {
								let randomBeaconHistoryHeartbeat = serviceAccount.storage.borrow<&RandomBeaconHistory.Heartbeat>(
									from: RandomBeaconHistory.HeartbeatStoragePath)
										?? panic("Couldn't borrow RandomBeaconHistory.Heartbeat Resource")
								randomBeaconHistoryHeartbeat.heartbeat(randomSourceHistory: randomSourceHistory())
							}
						}`, sc.RandomBeaconHistory.Address.HexWithPrefix())),
					).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)

				s, out, err := vm.Run(ctx, fvm.Transaction(txBody, 0), snapshot)
				require.NoError(t, err)
				require.NoError(t, out.Err)

				snapshot = snapshot.Append(s)

				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): [UInt8] {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)
						assert(res.status == EVM.Status.successful, message: "evm tx wrong status")
						return res.data
					}
                    `,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				// we fake progressing to new block height since random beacon does the check the
				// current height (2) is bigger than the height requested (1)
				block1.Height = 2
				ctx.BlockHeader = block1.ToHeader()

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToRandomSource", height),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				script := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
						).WithType(stdlib.EVMAddressBytesCadenceType),
					),
				)
				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res := make([]byte, environment.RandomSourceHistoryLength)
				vals := output.Value.(cadence.Array).Values
				require.Len(t, vals, environment.RandomSourceHistoryLength)

				for i := range res {
					res[i] = byte(vals[i].(cadence.UInt8))
				}
				require.Equal(t, source, res)
			})
	})

	t.Run("testing calling Cadence arch - random source (failed due to incorrect height)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// we must record a new heartbeat with a fixed block, we manually execute a transaction to do so,
				// since doing this automatically would require a block computer and whole execution setup
				height := uint64(1)
				block1 := unittest.BlockFixture(
					unittest.Block.WithHeight(height),
				)
				ctx.BlockHeader = block1.ToHeader()

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript([]byte(fmt.Sprintf(`
						import RandomBeaconHistory from %s

						transaction {
							prepare(serviceAccount: auth(Capabilities, Storage) &Account) {
								let randomBeaconHistoryHeartbeat = serviceAccount.storage.borrow<&RandomBeaconHistory.Heartbeat>(
									from: RandomBeaconHistory.HeartbeatStoragePath)
										?? panic("Couldn't borrow RandomBeaconHistory.Heartbeat Resource")
								randomBeaconHistoryHeartbeat.heartbeat(randomSourceHistory: randomSourceHistory())
							}
						}`, sc.RandomBeaconHistory.Address.HexWithPrefix())),
					).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)

				s, out, err := vm.Run(ctx, fvm.Transaction(txBody, 0), snapshot)
				require.NoError(t, err)
				require.NoError(t, out.Err)

				snapshot = snapshot.Append(s)

				height = 1337 // invalid
				// we make sure the transaction fails, due to requested height being invalid
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				// we fake progressing to new block height since random beacon does the check the
				// current height (2) is bigger than the height requested (1)
				block1.Height = 2
				ctx.BlockHeader = block1.ToHeader()

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToRandomSource", height),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				script := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(testAccount.Address().Bytes()),
						).WithType(stdlib.EVMAddressBytesCadenceType),
					),
				)
				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				// make sure the error is correct
				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)

				revertReason, err := abi.UnpackRevert(res.ReturnedData)
				require.NoError(t, err)
				require.Equal(t, "unsuccessful call to arch ", revertReason)
			})
	})

	t.Run("testing calling Cadence arch - COA ownership proof (happy case)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// create a flow account
				privateKey, err := testutil.GenerateAccountPrivateKey()
				require.NoError(t, err)

				snapshot, accounts, err := testutil.CreateAccounts(
					vm,
					snapshot,
					[]flow.AccountPrivateKey{privateKey},
					chain)
				require.NoError(t, err)
				flowAccount := accounts[0]

				// create/store/link coa
				coaAddress, snapshot := setupCOA(
					t,
					ctx,
					vm,
					snapshot,
					flowAccount,
					0,
				)

				data := RandomCommonHash(t)

				hasher, err := crypto.NewPrefixedHashing(privateKey.HashAlgo, "FLOW-V0.0-user")
				require.NoError(t, err)

				sig, err := privateKey.PrivateKey.Sign(data.Bytes(), hasher)
				require.NoError(t, err)

				validProof := types.COAOwnershipProof{
					KeyIndices:     []uint64{0},
					Address:        types.FlowAddress(flowAccount),
					CapabilityPath: "coa",
					Signatures:     []types.Signature{types.Signature(sig)},
				}

				encodedValidProof, err := validProof.Encode()
				require.NoError(t, err)

				// create transaction for proof verification
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)
						assert(res.status == EVM.Status.successful, message: "test failed: ".concat(res.errorCode.toString()))
					}
                	`,
					sc.EVMContract.Address.HexWithPrefix(),
				))
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToVerifyCOAOwnershipProof",
						true,
						coaAddress.ToCommon(),
						data,
						encodedValidProof),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				verifyScript := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(
							stdlib.EVMTransactionBytesCadenceType,
						)),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(
								testAccount.Address().Bytes(),
							),
						).WithType(
							stdlib.EVMAddressBytesCadenceType,
						),
					),
				)
				// run proof transaction
				_, output, err := vm.Run(
					ctx,
					verifyScript,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				invalidProof := types.COAOwnershipProof{
					KeyIndices:     []uint64{1000},
					Address:        types.FlowAddress(flowAccount),
					CapabilityPath: "coa",
					Signatures:     []types.Signature{types.Signature(sig)},
				}

				encodedInvalidProof, err := invalidProof.Encode()
				require.NoError(t, err)

				// invalid proof tx
				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToVerifyCOAOwnershipProof",
						true,
						coaAddress.ToCommon(),
						data,
						encodedInvalidProof),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)

				verifyScript = fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(
							stdlib.EVMTransactionBytesCadenceType,
						)),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(
								testAccount.Address().Bytes(),
							),
						).WithType(
							stdlib.EVMAddressBytesCadenceType,
						),
					),
				)
				// run proof transaction
				_, output, err = vm.Run(
					ctx,
					verifyScript,
					snapshot)
				require.NoError(t, err)
				require.Error(t, output.Err)
			})
	})

	t.Run("testing calling Cadence arch - COA ownership proof (index overflow)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction {
						let coa: @EVM.CadenceOwnedAccount

						prepare(signer: auth(Storage) &Account) {
							self.coa <- EVM.createCadenceOwnedAccount()
						}

						execute {
							let cadenceArchAddress = EVM.EVMAddress(
								bytes: [
									0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
									0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01
								]
							)

							var calldata: [UInt8] = []

							// Function selector for verifyCOAOwnershipProof = 0x5ee837e7
							calldata = calldata.concat([0x5e, 0xe8, 0x37, 0xe7])

							// Address parameter (32 bytes)
							var i = 0
							while i < 31 { calldata = calldata.concat([0x00]); i = i + 1 }
							calldata = calldata.concat([0x01])

							// bytes32 parameter (32 bytes)
							i = 0
							while i < 32 { calldata = calldata.concat([0x00]); i = i + 1 }

							// MALICIOUS offset: 0x7FFFFFFFFFFFFFFF (MaxInt64)
							// When ReadBytes does: index + 32, this overflows to negative
							// MaxInt64 + 32 = -9223372036854775777 (wraps around)
							i = 0
							while i < 24 { calldata = calldata.concat([0x00]); i = i + 1 }
							calldata = calldata.concat([0x7F]) // High byte = 0x7F
							calldata = calldata.concat([0xFF])
							calldata = calldata.concat([0xFF])
							calldata = calldata.concat([0xFF])
							calldata = calldata.concat([0xFF])
							calldata = calldata.concat([0xFF])
							calldata = calldata.concat([0xFF])
							calldata = calldata.concat([0xFF]) // = 0x7FFFFFFFFFFFFFFF

							// Length (32 bytes)
							i = 0
							while i < 31 { calldata = calldata.concat([0x00]); i = i + 1 }
							calldata = calldata.concat([0x20])

							// Dummy data (32 bytes)
							i = 0
							while i < 32 { calldata = calldata.concat([0x00]); i = i + 1 }

							let result = self.coa.call(
								to: cadenceArchAddress,
								data: calldata,
								gasLimit: 100_000,
								value: EVM.Balance(attoflow: 0)
							)
							assert(result.status == EVM.Status.failed, message: "unexpected status")
							assert(result.errorMessage == "input data is too small for decoding", message: result.errorMessage)

							destroy self.coa
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				_, output, err := vm.Run(ctx, tx, snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			},
		)
	})

	t.Run("testing calling Cadence arch - COA ownership proof (empty proof list)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// create a flow account
				privateKey, err := testutil.GenerateAccountPrivateKey()
				require.NoError(t, err)

				snapshot, accounts, err := testutil.CreateAccounts(
					vm,
					snapshot,
					[]flow.AccountPrivateKey{privateKey},
					chain)
				require.NoError(t, err)
				flowAccount := accounts[0]

				// create/store/link coa
				coaAddress, snapshot := setupCOA(
					t,
					ctx,
					vm,
					snapshot,
					flowAccount,
					0,
				)

				data := RandomCommonHash(t)

				emptyProofList, err := hex.DecodeString("c0") // empty RLP list
				require.NoError(t, err)

				// create transaction for proof verification
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToVerifyCOAOwnershipProof",
						true,
						coaAddress.ToCommon(),
						data,
						emptyProofList),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				verifyScript := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(innerTxBytes),
						).WithType(
							stdlib.EVMTransactionBytesCadenceType,
						)),
					json.MustEncode(
						cadence.NewArray(
							unittest.BytesToCdcUInt8(
								testAccount.Address().Bytes(),
							),
						).WithType(
							stdlib.EVMAddressBytesCadenceType,
						),
					),
				)

				// run proof transaction
				_, output, err := vm.Run(ctx, verifyScript, snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				// make sure the error is correct
				res, err := impl.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)

				revertReason, err := abi.UnpackRevert(res.ReturnedData)
				require.NoError(t, err)
				require.Equal(t, "unsuccessful call to arch", revertReason)
			},
		)
	})
}

func TestNativePrecompiles(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()

	t.Run("testing out of gas precompile call", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.failed, message: "unexpected status")
							assert(res.errorCode == 301, message: "unexpected error code: \(res.errorCode)")
							assert(res.errorMessage == "out of gas", message: "unexpected error message: \(res.errorMessage)")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				coinbaseAddr := types.Address{1, 2, 3}
				coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
				require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

				// The address below is the latest precompile on the Prague hard-fork:
				// https://github.com/ethereum/go-ethereum/blob/v1.16.3/core/vm/contracts.go#L140 .
				to := common.HexToAddress("0x00000000000000000000000000000000000000011")
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					to,
					nil,
					big.NewInt(1_000_000),
					uint64(21_000),
					big.NewInt(1),
				)

				innerTx := cadence.NewArray(
					unittest.BytesToCdcUInt8(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txBody, err := flow.NewTransactionBodyBuilder().
					SetScript(code).
					SetPayer(sc.FlowServiceAccount.Address).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(innerTx)).
					AddArgument(json.MustEncode(coinbase)).
					Build()
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot,
				)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// assert event fields are correct
				require.Len(t, output.Events, 2)
				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)
				require.Equal(t, uint16(types.ExecutionErrCodeOutOfGas), txEventPayload.ErrorCode)
				require.Equal(t, uint16(0), txEventPayload.Index)
				require.Equal(t, uint64(21000), txEventPayload.GasConsumed)
				require.NoError(t, err)
			})
	})
}

func TestEVMFileSystemContract(t *testing.T) {
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	runFileSystemContract := func(
		ctx fvm.Context,
		vm fvm.VM,
		snapshot snapshot.SnapshotTree,
		testContract *TestContract,
		testAccount *EOATestAccount,
		computeLimit uint64,
	) (
		*snapshot.ExecutionSnapshot,
		fvm.ProcedureOutput,
	) {
		code := []byte(fmt.Sprintf(
			`
					import EVM from %s

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: &Account) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)
						}
					}
					`,
			sc.EVMContract.Address.HexWithPrefix(),
		))

		coinbaseAddr := types.Address{1, 2, 3}
		coinbaseBalance := getEVMAccountBalance(t, ctx, vm, snapshot, coinbaseAddr)
		require.Zero(t, types.BalanceToBigInt(coinbaseBalance).Uint64())

		var buffer bytes.Buffer
		address := common.HexToAddress("0xea02F564664A477286B93712829180be4764fAe2")
		chunkHash := "0x2521660d04da85198d1cc71c20a69b7e875ebbf1682f6a5c6a3fec69068ccc13"
		index := int64(0)
		chunk := "T1ARYBYsPOJPVYoU7wVp+PYuXmdS6bgkLf2egcxa+1hP64wAxfkLXtnllU6DmEuj+Id4oWl1ZV4ftQ+ofQ3DQhOoxNlPZGOYbhoMLuzE"
		for i := 0; i <= 500; i++ {
			buffer.WriteString(chunk)
		}
		innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
			testContract.DeployedAt.ToCommon(),
			testContract.MakeCallData(
				t,
				"publishChunk",
				address,
				chunkHash,
				big.NewInt(index),
				buffer.String(),
			),
			big.NewInt(0),
			uint64(2_132_171),
			big.NewInt(1),
		)

		innerTx := cadence.NewArray(
			unittest.BytesToCdcUInt8(innerTxBytes),
		).WithType(stdlib.EVMTransactionBytesCadenceType)

		coinbase := cadence.NewArray(
			unittest.BytesToCdcUInt8(coinbaseAddr.Bytes()),
		).WithType(stdlib.EVMAddressBytesCadenceType)

		txBody, err := flow.NewTransactionBodyBuilder().
			SetScript(code).
			SetComputeLimit(computeLimit).
			AddAuthorizer(sc.FlowServiceAccount.Address).
			AddArgument(json.MustEncode(innerTx)).
			AddArgument(json.MustEncode(coinbase)).
			Build()
		require.NoError(t, err)

		tx := fvm.Transaction(
			txBody,
			0,
		)

		state, output, err := vm.Run(ctx, tx, snapshot)
		require.NoError(t, err)
		return state, output
	}

	t.Run("happy case", func(t *testing.T) {
		RunContractWithNewEnvironment(
			t,
			chain,
			GetFileSystemContract(t),
			func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				state, output := runFileSystemContract(ctx, vm, snapshot, testContract, testAccount, 10001)

				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)
				snapshot = snapshot.Append(state)

				// assert event fields are correct
				require.Len(t, output.Events, 2)
				txEvent := output.Events[0]
				txEventPayload := TxEventToPayload(t, txEvent, sc.EVMContract.Address)

				// fee transfer event
				feeTransferEvent := output.Events[1]
				feeTranferEventPayload := TxEventToPayload(t, feeTransferEvent, sc.EVMContract.Address)
				require.Equal(t, uint16(types.ErrCodeNoError), feeTranferEventPayload.ErrorCode)
				require.Equal(t, uint16(1), feeTranferEventPayload.Index)
				require.Equal(t, uint64(21000), feeTranferEventPayload.GasConsumed)
				//
				//// commit block
				blockEventPayload, _ := callEVMHeartBeat(t,
					ctx,
					vm,
					snapshot,
				)
				//
				require.NotEmpty(t, blockEventPayload.Hash)
				require.Equal(t, uint64(2_132_170), blockEventPayload.TotalGasUsed)

				txHashes := types.TransactionHashes{txEventPayload.Hash, feeTranferEventPayload.Hash}
				require.Equal(t,
					txHashes.RootHash(),
					blockEventPayload.TransactionHashRoot,
				)
				require.NotEmpty(t, blockEventPayload.ReceiptRoot)

				require.Equal(t, uint16(types.ErrCodeNoError), txEventPayload.ErrorCode)
				require.Equal(t, uint16(0), txEventPayload.Index)
				require.Equal(t, blockEventPayload.Height, txEventPayload.BlockHeight)
				require.Equal(t, blockEventPayload.TotalGasUsed-feeTranferEventPayload.GasConsumed, txEventPayload.GasConsumed)
				require.Empty(t, txEventPayload.ContractAddress)

				require.Greater(t, int(output.ComputationUsed), 900)
			},
			fvm.WithExecutionEffortWeights(
				environment.MainnetExecutionEffortWeights,
			),
		)
	})

	t.Run("insufficient FVM computation to execute EVM transaction", func(t *testing.T) {
		RunContractWithNewEnvironment(
			t,
			chain,
			GetFileSystemContract(t),
			func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				state, output := runFileSystemContract(ctx, vm, snapshot, testContract, testAccount, 500)
				snapshot = snapshot.Append(state)

				require.Len(t, output.Events, 0)

				//// commit block
				blockEventPayload, _ := callEVMHeartBeat(t,
					ctx,
					vm,
					snapshot,
				)
				//
				require.NotEmpty(t, blockEventPayload.Hash)
				require.Equal(t, uint64(0), blockEventPayload.TotalGasUsed)

				// only a small amount of computation was used due to the EVM transaction never being executed
				require.Less(t, int(output.ComputationUsed), 900)
			},
			fvm.WithExecutionEffortWeights(
				environment.MainnetExecutionEffortWeights,
			),
		)
	})
}

func createAndFundFlowAccount(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snapshot snapshot.SnapshotTree,
) (flow.Address, flow.AccountPrivateKey, snapshot.SnapshotTree) {

	privateKey, err := testutil.GenerateAccountPrivateKey()
	require.NoError(t, err)

	snapshot, accounts, err := testutil.CreateAccounts(
		vm,
		snapshot,
		[]flow.AccountPrivateKey{privateKey},
		ctx.Chain)
	require.NoError(t, err)
	flowAccount := accounts[0]

	// fund the account with 100 tokens
	sc := systemcontracts.SystemContractsForChain(ctx.Chain.ChainID())
	code := []byte(fmt.Sprintf(
		`
		import FlowToken from %s
		import FungibleToken from %s 

		transaction {
			prepare(account: auth(BorrowValue) &Account) {
			let admin = account.storage
				.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!

			let minter <- admin.createNewMinter(allowedAmount: 100.0)
			let vault <- minter.mintTokens(amount: 100.0)

			let receiverRef = getAccount(%s).capabilities
				.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
				?? panic("Could not borrow receiver reference to the recipient's Vault")
			receiverRef.deposit(from: <-vault)

			destroy minter
			}
		}
		`,
		sc.FlowToken.Address.HexWithPrefix(),
		sc.FungibleToken.Address.HexWithPrefix(),
		flowAccount.HexWithPrefix(),
	))

	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript(code).
		SetPayer(sc.FlowServiceAccount.Address).
		AddAuthorizer(sc.FlowServiceAccount.Address).
		Build()
	require.NoError(t, err)

	tx := fvm.Transaction(txBody, 0)

	es, output, err := vm.Run(ctx, tx, snapshot)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	snapshot = snapshot.Append(es)

	bal := getFlowAccountBalance(
		t,
		ctx,
		vm,
		snapshot,
		flowAccount)
	// 100 flow in ufix64
	require.Equal(t, uint64(10_000_000_000), bal)

	return flowAccount, privateKey, snapshot
}

func setupCOA(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	coaOwner flow.Address,
	initialFund uint64,
) (types.Address, snapshot.SnapshotTree) {

	sc := systemcontracts.SystemContractsForChain(ctx.Chain.ChainID())
	// create a COA and store it under flow account
	script := []byte(fmt.Sprintf(
		`
	import EVM from %s
	import FungibleToken from %s
	import FlowToken from %s

	transaction(amount: UFix64) {
		prepare(account: auth(Capabilities, Storage) &Account) {
			let cadenceOwnedAccount1 <- EVM.createCadenceOwnedAccount()
			
			let vaultRef = account.storage
                .borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
				?? panic("Could not borrow reference to the owner's Vault!")

			if amount > 0.0 {
				let vault <- vaultRef.withdraw(amount: amount) as! @FlowToken.Vault
				cadenceOwnedAccount1.deposit(from: <-vault)
			}
			
			account.storage.save<@EVM.CadenceOwnedAccount>(
				<-cadenceOwnedAccount1,
				to: /storage/coa
			)

			let cap = account.capabilities.storage
				.issue<&EVM.CadenceOwnedAccount>(/storage/coa)
			account.capabilities.publish(cap, at: /public/coa)
		}
	}
	`,
		sc.EVMContract.Address.HexWithPrefix(),
		sc.FungibleToken.Address.HexWithPrefix(),
		sc.FlowToken.Address.HexWithPrefix(),
	))

	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript(script).
		SetPayer(coaOwner).
		AddAuthorizer(coaOwner).
		AddArgument(json.MustEncode(cadence.UFix64(initialFund))).
		Build()
	require.NoError(t, err)

	tx := fvm.Transaction(txBody, 0)
	es, output, err := vm.Run(ctx, tx, snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	snap = snap.Append(es)

	// 3rd event is the cadence owned account created event
	coaAddress, err := types.COAAddressFromFlowCOACreatedEvent(sc.EVMContract.Address, output.Events[1])
	require.NoError(t, err)

	return coaAddress, snap
}

func callEVMHeartBeat(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
) (*events.BlockEventPayload, snapshot.SnapshotTree) {
	sc := systemcontracts.SystemContractsForChain(ctx.Chain.ChainID())

	heartBeatCode := []byte(fmt.Sprintf(
		`
	import EVM from %s
	transaction {
		prepare(serviceAccount: auth(BorrowValue) &Account) {
			let evmHeartbeat = serviceAccount.storage
				.borrow<&EVM.Heartbeat>(from: /storage/EVMHeartbeat)
				?? panic("Couldn't borrow EVM.Heartbeat Resource")
			evmHeartbeat.heartbeat()
		}
	}
	`,
		sc.EVMContract.Address.HexWithPrefix(),
	))
	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript(heartBeatCode).
		SetPayer(sc.FlowServiceAccount.Address).
		AddAuthorizer(sc.FlowServiceAccount.Address).
		Build()
	require.NoError(t, err)

	tx := fvm.Transaction(txBody, 0)

	state, output, err := vm.Run(ctx, tx, snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	require.NotEmpty(t, state.WriteSet)
	snap = snap.Append(state)

	// validate block event
	require.Len(t, output.Events, 1)
	blockEvent := output.Events[0]
	return BlockEventToPayload(t, blockEvent, sc.EVMContract.Address), snap
}

func getFlowAccountBalance(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	address flow.Address,
) uint64 {
	code := []byte(fmt.Sprintf(
		`
		access(all) fun main(): UFix64 {
			return getAccount(%s).balance
		}
		`,
		address.HexWithPrefix(),
	))

	script := fvm.Script(code)
	_, output, err := vm.Run(
		ctx,
		script,
		snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	val, ok := output.Value.(cadence.UFix64)
	require.True(t, ok)
	return uint64(val)
}

func getEVMAccountBalance(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	address types.Address,
) types.Balance {
	code := []byte(fmt.Sprintf(
		`
		import EVM from %s
		access(all)
		fun main(addr: [UInt8; 20]): UInt {
			return EVM.EVMAddress(bytes: addr).balance().inAttoFLOW()
		}
		`,
		systemcontracts.SystemContractsForChain(
			ctx.Chain.ChainID(),
		).EVMContract.Address.HexWithPrefix(),
	))

	script := fvm.Script(code).WithArguments(
		json.MustEncode(
			cadence.NewArray(
				unittest.BytesToCdcUInt8(address.Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType),
		),
	)
	_, output, err := vm.Run(
		ctx,
		script,
		snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	val, ok := output.Value.(cadence.UInt)
	require.True(t, ok)
	return val.Big()
}

func getEVMAccountNonce(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	address types.Address,
) uint64 {
	code := []byte(fmt.Sprintf(
		`
		import EVM from %s
		access(all)
		fun main(addr: [UInt8; 20]): UInt64 {
			return EVM.EVMAddress(bytes: addr).nonce()
		}
		`,
		systemcontracts.SystemContractsForChain(
			ctx.Chain.ChainID(),
		).EVMContract.Address.HexWithPrefix(),
	))

	script := fvm.Script(code).WithArguments(
		json.MustEncode(
			cadence.NewArray(
				unittest.BytesToCdcUInt8(address.Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType),
		),
	)
	_, output, err := vm.Run(
		ctx,
		script,
		snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	val, ok := output.Value.(cadence.UInt64)
	require.True(t, ok)
	return uint64(val)
}

func RunWithNewEnvironment(
	t *testing.T,
	chain flow.Chain,
	f func(fvm.Context, fvm.VM, snapshot.SnapshotTree, *TestContract, *EOATestAccount),
) {
	rootAddr := evm.StorageAccountAddress(chain.ChainID())
	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithDeployedContract(t, GetStorageTestContract(t), backend, rootAddr, func(testContract *TestContract) {
			RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {
				blocks := new(envMock.Blocks)
				block1 := unittest.BlockFixture()
				blocks.On("ByHeightFrom",
					block1.Height,
					block1.ToHeader(),
				).Return(block1.ToHeader(), nil)

				opts := []fvm.Option{
					fvm.WithBlockHeader(block1.ToHeader()),
					fvm.WithAuthorizationChecksEnabled(false),
					fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
					fvm.WithEntropyProvider(testutil.EntropyProviderFixture(nil)),
					fvm.WithRandomSourceHistoryCallAllowed(true),
					fvm.WithBlocks(blocks),
					fvm.WithCadenceLogging(true),
				}
				ctx := fvm.NewContext(chain, opts...)

				vm := fvm.NewVirtualMachine()
				snapshotTree := snapshot.NewSnapshotTree(backend)

				baseBootstrapOpts := []fvm.BootstrapProcedureOption{
					fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
				}

				executionSnapshot, _, err := vm.Run(
					ctx,
					fvm.Bootstrap(unittest.ServiceAccountPublicKey, baseBootstrapOpts...),
					snapshotTree)
				require.NoError(t, err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				f(
					ctx,
					vm,
					snapshotTree,
					testContract,
					testAccount,
				)
			})
		})
	})
}

func RunContractWithNewEnvironment(
	t *testing.T,
	chain flow.Chain,
	tc *TestContract,
	f func(fvm.Context, fvm.VM, snapshot.SnapshotTree, *TestContract, *EOATestAccount),
	bootstrapOpts ...fvm.BootstrapProcedureOption,
) {
	rootAddr := evm.StorageAccountAddress(chain.ChainID())

	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithDeployedContract(t, tc, backend, rootAddr, func(testContract *TestContract) {
			RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {

				blocks := new(envMock.Blocks)
				block1 := unittest.BlockFixture()
				header1 := block1.ToHeader()
				blocks.On("ByHeightFrom",
					header1.Height,
					header1,
				).Return(header1, nil)

				opts := []fvm.Option{
					fvm.WithBlockHeader(header1),
					fvm.WithAuthorizationChecksEnabled(false),
					fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
					fvm.WithEntropyProvider(testutil.EntropyProviderFixture(nil)),
					fvm.WithRandomSourceHistoryCallAllowed(true),
					fvm.WithBlocks(blocks),
					fvm.WithCadenceLogging(true),
				}
				ctx := fvm.NewContext(chain, opts...)

				vm := fvm.NewVirtualMachine()
				snapshotTree := snapshot.NewSnapshotTree(backend)

				baseBootstrapOpts := []fvm.BootstrapProcedureOption{
					fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
				}
				baseBootstrapOpts = append(baseBootstrapOpts, bootstrapOpts...)

				executionSnapshot, _, err := vm.Run(
					ctx,
					fvm.Bootstrap(unittest.ServiceAccountPublicKey, baseBootstrapOpts...),
					snapshotTree)
				require.NoError(t, err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				f(
					ctx,
					vm,
					snapshotTree,
					testContract,
					testAccount,
				)
			})
		})
	})
}
