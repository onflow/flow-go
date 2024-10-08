package sync_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/debug"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func TestChainReplay(t *testing.T) {

	const chainID = flow.Emulator
	var snapshot *TestValueStore
	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			RunWithDeployedContract(t,
				GetStorageTestContract(t), backend, rootAddr, func(testContract *TestContract) {
					RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {
						handler := setupHandler(chainID, backend, rootAddr)

						// clone state before apply transactions
						snapshot = backend.Clone()
						gasFeeCollector := RandomAddress(t)

						totalTxCount := 0

						// case: check sequential updates to a slot
						for i := 0; i < 5; i++ {
							tx := testAccount.PrepareSignAndEncodeTx(t,
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "checkThenStore", big.NewInt(int64(i)), big.NewInt(int64(i+1))),
								big.NewInt(0),
								uint64(100_000),
								big.NewInt(1),
							)
							rs := handler.Run(tx, gasFeeCollector)
							require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
							totalTxCount += 2 // one for tx, one for gas refund
						}

						// case: add batch run BatchRun
						batchSize := 4
						txBatch := make([][]byte, batchSize)
						for i := 0; i < batchSize; i++ {
							txBatch[i] = testAccount.PrepareSignAndEncodeTx(t,
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "store", big.NewInt(int64(i))),
								big.NewInt(0),
								uint64(100_000),
								big.NewInt(1),
							)
						}
						rss := handler.BatchRun(txBatch, gasFeeCollector)
						for _, rs := range rss {
							require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						}
						totalTxCount += batchSize + 1 // plus one for gas refund

						// case: fetching evm block number
						tx := testAccount.PrepareSignAndEncodeTx(t,
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t, "checkBlockNumber", big.NewInt(1)),
							big.NewInt(0),
							uint64(100_000),
							big.NewInt(1),
						)
						rs := handler.Run(tx, gasFeeCollector)
						require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						totalTxCount += 2 // one for tx, one for gas refund

						// case: making a call to the cadence arch
						expectedFlowHeight := uint64(3)
						tx = testAccount.PrepareSignAndEncodeTx(t,
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t, "verifyArchCallToFlowBlockHeight", expectedFlowHeight),
							big.NewInt(0),
							uint64(2_000_000),
							big.NewInt(1),
						)
						rs = handler.Run(tx, gasFeeCollector)
						require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						totalTxCount += 2 // one for tx, one for gas refund

						// case: fetch evm block hash - last block
						expected := types.GenesisBlockHash(chainID)
						tx = testAccount.PrepareSignAndEncodeTx(t,
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t, "checkBlockHash", big.NewInt(0), expected),
							big.NewInt(0),
							uint64(100_000),
							big.NewInt(1),
						)
						rs = handler.Run(tx, gasFeeCollector)
						require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						totalTxCount += 2 // one for tx, one for gas refund

						// case: fetch evm block hash - current block
						expected = gethCommon.Hash{}
						tx = testAccount.PrepareSignAndEncodeTx(t,
							testContract.DeployedAt.ToCommon(),
							testContract.MakeCallData(t, "checkBlockHash", big.NewInt(1), expected),
							big.NewInt(0),
							uint64(100_000),
							big.NewInt(1),
						)
						rs = handler.Run(tx, gasFeeCollector)
						require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						totalTxCount += 2 // one for tx, one for gas refund

						// case: coa operations
						addr := handler.DeployCOA(100)
						totalTxCount += 1

						coa := handler.AccountByAddress(addr, true)
						coa.Deposit(types.NewFlowTokenVault(types.MakeABalanceInFlow(10)))
						totalTxCount += 1
						coa.Withdraw(types.NewBalance(types.MakeABalanceInFlow(4)))
						totalTxCount += 1

						expectedBalance := (*big.Int)(types.MakeABalanceInFlow(6))
						rs = coa.Call(
							testContract.DeployedAt,
							testContract.MakeCallData(t, "checkBalance", addr.ToCommon(), expectedBalance),
							100_000,
							types.EmptyBalance,
						)
						require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						totalTxCount += 1

						rs = coa.Deploy(testContract.ByteCode, math.MaxUint64, types.EmptyBalance)
						require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						totalTxCount += 1

						// commit block
						handler.CommitBlockProposal()

						// prepare events
						txEventPayloads, blockEventPayload := prepareEvents(t, chainID, backend.Events())

						// because we are doing direct calls, there is no extra
						// events (e.g. COA created) events emitted.
						require.Len(t, txEventPayloads, totalTxCount)

						// check replay
						sp := newTestStorageProvider(snapshot, 1)
						cr := sync.NewReplayer(chainID, rootAddr, sp, zerolog.Logger{}, nil, true)
						_, err := cr.OnBlockReceived(txEventPayloads, blockEventPayload)
						require.NoError(t, err)

						// TODO: verify the state delta
						// currently the backend storage doesn't work well with this
						// changes needed to make this work, which is left for future PRs
						//
						// for k, v := range result.StorageRegisterUpdates() {
						// 	ret, err := backend.GetValue([]byte(k.Owner), []byte(k.Key))
						// 	require.NoError(t, err)
						// 	require.Equal(t, ret[:], v[:])
						// }
					})
				})
		})
	})
}

func prepareEvents(
	t *testing.T,
	chainID flow.ChainID,
	allEvents flow.EventsList) (
	[]events.TransactionEventPayload,
	*events.BlockEventPayload,
) {
	evmContract := evm.ContractAccountAddress(chainID)
	var blockEventPayload *events.BlockEventPayload
	txEventPayloads := make([]events.TransactionEventPayload, len(allEvents)-1)
	for i, event := range allEvents {
		// last event is block event
		if i == len(allEvents)-1 {
			blockEventPayload = BlockEventToPayload(t, event, evmContract)
			continue
		}
		txEventPayloads[i] = *TxEventToPayload(t, event, evmContract)
	}
	return txEventPayloads, blockEventPayload
}

func setupHandler(
	chainID flow.ChainID,
	backend types.Backend,
	rootAddr flow.Address,
) *handler.ContractHandler {
	return handler.NewContractHandler(
		chainID,
		rootAddr,
		common.MustBytesToAddress(systemcontracts.SystemContractsForChain(chainID).FlowToken.Address.Bytes()),
		rootAddr,
		handler.NewBlockStore(chainID, backend, rootAddr),
		handler.NewAddressAllocator(),
		backend,
		emulator.NewEmulator(backend, rootAddr),
		debug.NopTracer,
	)
}

// TestStorageProvider constructs a new
// storage provider that only provides
// storage for an specific height
type testStorageProvider struct {
	storage types.BackendStorage
	height  uint64
}

var _ types.StorageProvider = &testStorageProvider{}

// NewTestStorageProvider constructs a new TestStorageProvider
func newTestStorageProvider(
	initSnapshot types.BackendStorageSnapshot,
	height uint64,
) *testStorageProvider {
	return &testStorageProvider{
		storage: storage.NewEphemeralStorage(storage.NewReadOnlyStorage(initSnapshot)),
		height:  height,
	}
}

// GetSnapshotAt returns the snapshot at specific block height
func (sp *testStorageProvider) GetSnapshotAt(height uint64) (types.BackendStorageSnapshot, error) {
	if height != sp.height {
		return nil, fmt.Errorf("storage for the given height (%d) is not available", height)
	}
	return sp.storage, nil
}
