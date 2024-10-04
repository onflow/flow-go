package sync_test

import (
	"math/big"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/debug"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TODO: add all sorts of transactions (height query, precompiled, ...)

func TestChainReplay(t *testing.T) {

	// setup transactions
	var allEvents flow.EventsList
	const chainID = flow.Emulator
	evmContract := evm.ContractAccountAddress(chainID)
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			testutils.RunWithDeployedContract(t,
				testutils.GetStorageTestContract(t),
				backend, rootAddr, func(testContract *testutils.TestContract) {
					testutils.RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *testutils.EOATestAccount) {
						handler := setupHandler(t, chainID, backend, rootAddr)
						gasFeeCollector := testutils.RandomAddress(t)
						for i := 0; i < 10; i++ {
							tx := testAccount.PrepareSignAndEncodeTx(t,
								testContract.DeployedAt.ToCommon(),
								testContract.MakeCallData(t, "store", big.NewInt(int64(i))),
								big.NewInt(0),
								uint64(100_000),
								big.NewInt(1),
							)
							rs := handler.Run(tx, gasFeeCollector)
							require.Equal(t, types.ErrorCode(0), rs.ErrorCode)
						}

						// call to block number - assert block number
						// TODO: here

						handler.CommitBlockProposal()
						allEvents = backend.Events()
					})
				})
		})
	})

	var blockEventPayload *events.BlockEventPayload
	txEventPayloads := make([]events.TransactionEventPayload, len(allEvents)-1)
	for i, event := range allEvents {
		// last event is block event
		if i == len(allEvents)-1 {
			blockEventPayload = testutils.BlockEventToPayload(t, event, evmContract)
			continue
		}
		txEventPayloads[i] = *testutils.TxEventToPayload(t, event, evmContract)
	}
	// How do you expose the init estate

	sp := storage.NewInMemoryStorageProvider()
	cr := sync.NewChainReplayer(chainID, sp, zerolog.Logger{}, nil, true)

	_, err := cr.OnBlockReceived(txEventPayloads, blockEventPayload)
	require.NoError(t, err)
	t.Fatal("XX")

	// transactions that checks the value of the previous call and increment it

}

func setupHandler(t testing.TB,
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
