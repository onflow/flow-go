package evm_test

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/cadence/encoding/ccf"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethParams "github.com/onflow/go-ethereum/params"
	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/crypto"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/testutils"
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

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// assert event fiedls are correct
				require.Len(t, output.Events, 2)

				blockEvent := output.Events[1]

				assert.Equal(
					t,
					common.NewAddressLocation(
						nil,
						common.Address(sc.EVMContract.Address),
						string(types.EventTypeBlockExecuted),
					).ID(),
					string(blockEvent.Type),
				)

				ev, err := ccf.Decode(nil, blockEvent.Payload)
				require.NoError(t, err)
				cadenceEvent, ok := ev.(cadence.Event)
				require.True(t, ok)

				blockEventPayload, err := types.DecodeBlockEventPayload(cadenceEvent)
				require.NoError(t, err)
				require.NotEmpty(t, blockEventPayload.Hash)

				txEvent := output.Events[0]

				assert.Equal(
					t,
					common.NewAddressLocation(
						nil,
						common.Address(sc.EVMContract.Address),
						string(types.EventTypeTransactionExecuted),
					).ID(),
					string(txEvent.Type),
				)

				ev, err = ccf.Decode(nil, txEvent.Payload)
				require.NoError(t, err)
				cadenceEvent, ok = ev.(cadence.Event)
				require.True(t, ok)

				txEventPayload, err := types.DecodeTransactionEventPayload(cadenceEvent)
				require.NoError(t, err)
				require.NotEmpty(t, txEventPayload.Hash)
				require.Equal(t, hex.EncodeToString(innerTxBytes), txEventPayload.Payload)
				require.Equal(t, uint16(types.ErrCodeNoError), txEventPayload.ErrorCode)
				require.Equal(t, uint16(0), txEventPayload.Index)
				require.Equal(t, blockEventPayload.Hash, txEventPayload.BlockHash)
				require.Equal(t, blockEventPayload.Height, txEventPayload.BlockHeight)
				require.Equal(t, blockEventPayload.TotalGasUsed, txEventPayload.GasConsumed)
				require.Equal(t, uint64(43785), blockEventPayload.TotalGasUsed)
				require.Empty(t, txEventPayload.ContractAddress)

				// append the state
				snapshot = snapshot.Append(state)

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
					ConvertToCadence(innerTxBytes),
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

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.Nil(t, res.DeployedContractAddress)
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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

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
					ConvertToCadence(innerTxBytes),
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

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				txEvent := output.Events[0]
				ev, err := ccf.Decode(nil, txEvent.Payload)
				require.NoError(t, err)
				cadenceEvent, ok := ev.(cadence.Event)
				require.True(t, ok)

				event, err := types.DecodeTransactionEventPayload(cadenceEvent)
				require.NoError(t, err)
				require.NotEmpty(t, event.Hash)

				encodedLogs, err := hex.DecodeString(event.Logs)
				require.NoError(t, err)

				var logs []*gethTypes.Log
				err = rlp.DecodeBytes(encodedLogs, &logs)
				require.NoError(t, err)
				require.Len(t, logs, 1)
				log := logs[0]
				last := log.Topics[len(log.Topics)-1] // last topic is the value set in the store method
				assert.Equal(t, num, last.Big().Int64())
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
						big.NewInt(0),
					)

					// build txs argument
					txBytes[i] = cadence.NewArray(
						ConvertToCadence(tx),
					).WithType(stdlib.EVMTransactionBytesCadenceType)
				}

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(batchRunCode).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(txs)).
						AddArgument(json.MustEncode(coinbase)),
					0)

				state, output, err := vm.Run(ctx, tx, snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				require.Len(t, output.Events, batchCount+1) // +1 block executed
				for i, event := range output.Events {
					if i == batchCount { // last one is block executed
						continue
					}

					ev, err := ccf.Decode(nil, event.Payload)
					require.NoError(t, err)
					cadenceEvent, ok := ev.(cadence.Event)
					require.True(t, ok)

					event, err := types.DecodeTransactionEventPayload(cadenceEvent)
					require.NoError(t, err)

					encodedLogs, err := hex.DecodeString(event.Logs)
					require.NoError(t, err)

					var logs []*gethTypes.Log
					err = rlp.DecodeBytes(encodedLogs, &logs)
					require.NoError(t, err)

					require.Len(t, logs, 1)

					log := logs[0]
					last := log.Topics[len(log.Topics)-1] // last topic is the value set in the store method
					assert.Equal(t, storedValues[i], last.Big().Int64())
				}

				// last one is block executed, make sure TotalGasUsed is non-zero
				blockEvent := output.Events[batchCount]

				assert.Equal(
					t,
					common.NewAddressLocation(
						nil,
						common.Address(sc.EVMContract.Address),
						string(types.EventTypeBlockExecuted),
					).ID(),
					string(blockEvent.Type),
				)

				ev, err := ccf.Decode(nil, blockEvent.Payload)
				require.NoError(t, err)
				cadenceEvent, ok := ev.(cadence.Event)
				require.True(t, ok)

				blockEventPayload, err := types.DecodeBlockEventPayload(cadenceEvent)
				require.NoError(t, err)
				require.NotEmpty(t, blockEventPayload.Hash)
				require.Equal(t, uint64(155513), blockEventPayload.TotalGasUsed)

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
					ConvertToCadence(innerTxBytes),
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
				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
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
						ConvertToCadence(tx),
					).WithType(stdlib.EVMTransactionBytesCadenceType)
				}

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(batchRunCode).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(txs)).
						AddArgument(json.MustEncode(coinbase)),
					0)

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
					ConvertToCadence(innerTxBytes),
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
				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
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
									assert(res.errorCode == 301, message: "unexpected error code")
									assert(res.errorMessage == "out of gas", message: "unexpected error msg")
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
						ConvertToCadence(tx),
					).WithType(stdlib.EVMTransactionBytesCadenceType)
				}

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				txs := cadence.NewArray(txBytes).
					WithType(cadence.NewVariableSizedArrayType(
						stdlib.EVMTransactionBytesCadenceType,
					))

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(batchRunCode).
						AddArgument(json.MustEncode(txs)).
						AddArgument(json.MustEncode(coinbase)),
					0)

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
					ConvertToCadence(innerTxBytes),
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
				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Empty(t, res.ErrorMessage)
				require.Equal(t, num, new(big.Int).SetBytes(res.ReturnedData).Int64())
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
				ConvertToCadence(testAccount.Address().Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType)

			innerTx := cadence.NewArray(
				ConvertToCadence(innerTxBytes),
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

			res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
			require.NoError(t, err)
			require.Equal(t, types.StatusSuccessful, res.Status)
			require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
			require.Empty(t, res.ErrorMessage)
			require.Equal(t, ctx.BlockHeader.Timestamp.Unix(), new(big.Int).SetBytes(res.ReturnedData).Int64())

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

			tx := fvm.Transaction(
				flow.NewTransactionBody().
					SetScript(code).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(cadence.NewArray(
						ConvertToCadence(addr.Bytes()),
					).WithType(stdlib.EVMAddressBytesCadenceType))),
				0)

			execSnap, output, err := vm.Run(
				ctx,
				tx,
				snapshot)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshot = snapshot.Append(execSnap)

			expectedBalance := types.OneFlowBalance
			bal := getEVMAccountBalance(t, ctx, vm, snapshot, addr)
			require.Equal(t, expectedBalance, bal)

			// block executed event, make sure TotalGasUsed is non-zero
			blockEvent := output.Events[3]

			assert.Equal(
				t,
				common.NewAddressLocation(
					nil,
					common.Address(sc.EVMContract.Address),
					string(types.EventTypeBlockExecuted),
				).ID(),
				string(blockEvent.Type),
			)

			ev, err := ccf.Decode(nil, blockEvent.Payload)
			require.NoError(t, err)
			cadenceEvent, ok := ev.(cadence.Event)
			require.True(t, ok)

			blockEventPayload, err := types.DecodeBlockEventPayload(cadenceEvent)
			require.NoError(t, err)
			require.NotEmpty(t, blockEventPayload.Hash)
			require.Equal(t, uint64(21000), blockEventPayload.TotalGasUsed)
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

	t.Run("test coa withdraw", func(t *testing.T) {
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
				fun main(): UFix64 {
					let admin = getAuthAccount<auth(BorrowValue) &Account>(%s)
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
					ConvertToCadence(testutils.RandomAddress(t).Bytes()),
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
							ConvertToCadence(testContract.ByteCode),
						).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
					))

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Empty(t, res.ErrorMessage)
				require.NotNil(t, res.DeployedContractAddress)
				// we strip away first few bytes because they contain deploy code
				require.Equal(t, testContract.ByteCode[17:], []byte(res.ReturnedData))
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
		testContract *TestContract,
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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType),
			),
		)
		_, output, err := vm.Run(
			ctx,
			script,
			snapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		result, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
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

				limit := uint64(math.MaxUint64 - 1)
				tx := gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result := dryRunTx(t, tx, ctx, vm, snapshot, testContract)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Greater(t, result.GasConsumed, uint64(0))
				require.Less(t, result.GasConsumed, limit)

				// gas limit too low, but still bigger than intrinsic gas value
				limit = uint64(21216)
				tx = gethTypes.NewTransaction(
					0,
					testContract.DeployedAt.ToCommon(),
					big.NewInt(0),
					limit,
					big.NewInt(0),
					data,
				)
				result = dryRunTx(t, tx, ctx, vm, snapshot, testContract)
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
				dryRunResult := dryRunTx(t, tx, ctx, vm, snapshot, testContract)

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

				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					data,
					big.NewInt(0),
					dryRunResult.GasConsumed, // use the gas estimation from Evm.dryRun
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
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

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				// Make sure that gas consumed from `EVM.dryRun` is bigger
				// than the actual gas consumption of the equivalent
				// `EVM.run`.
				totalGas := emulator.AddOne64th(res.GasConsumed) + gethParams.SstoreSentryGasEIP2200
				require.Equal(
					t,
					totalGas,
					dryRunResult.GasConsumed,
				)
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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

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
				dryRunResult := dryRunTx(t, tx1, ctx, vm, snapshot, testContract)

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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase = cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
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

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				// Make sure that gas consumed from `EVM.dryRun` is bigger
				// than the actual gas consumption of the equivalent
				// `EVM.run`.
				totalGas := emulator.AddOne64th(res.GasConsumed) + gethParams.SstoreSentryGasEIP2200
				require.Equal(
					t,
					totalGas,
					dryRunResult.GasConsumed,
				)
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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

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
				dryRunResult := dryRunTx(t, tx1, ctx, vm, snapshot, testContract)

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

				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					data,
					big.NewInt(0),
					dryRunResult.GasConsumed, // use the gas estimation from Evm.dryRun
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase = cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
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

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				//require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				// Make sure that gas consumed from `EVM.dryRun` is bigger
				// than the actual gas consumption of the equivalent
				// `EVM.run`.
				totalGas := emulator.AddOne64th(res.GasConsumed) + gethParams.SstoreSentryGasEIP2200 + gethParams.SstoreClearsScheduleRefundEIP3529
				require.Equal(
					t,
					totalGas,
					dryRunResult.GasConsumed,
				)
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

				result := dryRunTx(t, tx, ctx, vm, snapshot, testContract)
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
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
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

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
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

				result := dryRunTx(t, tx, ctx, vm, snapshot, testContract)
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

				result := dryRunTx(t, tx, ctx, vm, snapshot, testContract)
				assert.Equal(t, types.ValidationErrCodeInsufficientFunds, result.ErrorCode)
				assert.Equal(t, types.StatusInvalid, result.Status)
				assert.Equal(t, types.InvalidTransactionGasCost, int(result.GasConsumed))
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
							ConvertToCadence(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(testAccount.Address().Bytes()),
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
							ConvertToCadence(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(testAccount.Address().Bytes()),
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
				source := []byte{91, 161, 206, 171, 100, 17, 141, 44} // coresponding out to the above entropy

				// we must record a new heartbeat with a fixed block, we manually execute a transaction to do so,
				// since doing this automatically would require a block computer and whole execution setup
				height := uint64(1)
				block1 := unittest.BlockFixture()
				block1.Header.Height = height
				ctx.BlockHeader = block1.Header
				ctx.EntropyProvider = testutil.EntropyProviderFixture(entropy) // fix the entropy

				txBody := flow.NewTransactionBody().
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
					AddAuthorizer(sc.FlowServiceAccount.Address)

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
				block1.Header.Height = 2
				ctx.BlockHeader = block1.Header

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
							ConvertToCadence(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(testAccount.Address().Bytes()),
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
				block1 := unittest.BlockFixture()
				block1.Header.Height = height
				ctx.BlockHeader = block1.Header

				txBody := flow.NewTransactionBody().
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
					AddAuthorizer(sc.FlowServiceAccount.Address)

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
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						let res = EVM.run(tx: tx, coinbase: coinbase)
					}
                    `,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				// we fake progressing to new block height since random beacon does the check the
				// current height (2) is bigger than the height requested (1)
				block1.Header.Height = 2
				ctx.BlockHeader = block1.Header

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
							ConvertToCadence(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(testAccount.Address().Bytes()),
						).WithType(stdlib.EVMAddressBytesCadenceType),
					),
				)
				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				// make sure the error is correct
				require.ErrorContains(t, output.Err, "Source of randomness not yet recorded")
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
							ConvertToCadence(innerTxBytes),
						).WithType(
							stdlib.EVMTransactionBytesCadenceType,
						)),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(
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
							ConvertToCadence(innerTxBytes),
						).WithType(
							stdlib.EVMTransactionBytesCadenceType,
						)),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(
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

	tx := fvm.Transaction(
		flow.NewTransactionBody().
			SetScript(code).
			AddAuthorizer(sc.FlowServiceAccount.Address),
		0)

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

	tx := fvm.Transaction(
		flow.NewTransactionBody().
			SetScript(script).
			AddAuthorizer(coaOwner).
			AddArgument(json.MustEncode(cadence.UFix64(initialFund))),
		0)
	es, output, err := vm.Run(ctx, tx, snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	snap = snap.Append(es)

	// 3rd event is the cadence owned account created event
	coaAddress, err := types.COAAddressFromFlowCOACreatedEvent(sc.EVMContract.Address, output.Events[2])
	require.NoError(t, err)

	return coaAddress, snap
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
				ConvertToCadence(address.Bytes()),
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
				ConvertToCadence(address.Bytes()),
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
	f func(
		fvm.Context,
		fvm.VM,
		snapshot.SnapshotTree,
		*TestContract,
		*EOATestAccount,
	),
) {
	rootAddr := evm.StorageAccountAddress(chain.ChainID())

	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithDeployedContract(t, GetStorageTestContract(t), backend, rootAddr, func(testContract *TestContract) {
			RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {

				blocks := new(envMock.Blocks)
				block1 := unittest.BlockFixture()
				blocks.On("ByHeightFrom",
					block1.Header.Height,
					block1.Header,
				).Return(block1.Header, nil)

				opts := []fvm.Option{
					fvm.WithChain(chain),
					fvm.WithBlockHeader(block1.Header),
					fvm.WithAuthorizationChecksEnabled(false),
					fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
					fvm.WithEntropyProvider(testutil.EntropyProviderFixture(nil)),
					fvm.WithRandomSourceHistoryCallAllowed(true),
					fvm.WithBlocks(blocks),
					fvm.WithCadenceLogging(true),
				}
				ctx := fvm.NewContext(opts...)

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
					fvm.NewContextFromParent(ctx, fvm.WithEVMEnabled(true)),
					vm,
					snapshotTree,
					testContract,
					testAccount,
				)
			})
		})
	})
}
