package sync_test

// import (
// 	"math/big"
// 	"testing"

// 	"github.com/onflow/cadence/runtime/common"
// 	"github.com/onflow/flow-go/fvm/evm/debug"
// 	"github.com/onflow/flow-go/fvm/evm/emulator"
// 	"github.com/onflow/flow-go/fvm/evm/handler"
// 	"github.com/onflow/flow-go/fvm/evm/sync"
// 	"github.com/onflow/flow-go/fvm/evm/testutils"
// 	"github.com/onflow/flow-go/fvm/evm/types"
// 	"github.com/onflow/flow-go/fvm/systemcontracts"
// 	"github.com/onflow/flow-go/model/flow"
// 	"github.com/stretchr/testify/require"
// )

// // create a chain
// func TestChainReplay(t *testing.T) {

// 	// setup transactions
// 	var events flow.EventsList
// 	const chainID = flow.Emulator
// 	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
// 		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
// 			testutils.RunWithDeployedContract(t,
// 				testutils.GetStorageTestContract(t),
// 				backend, rootAddr, func(testContract *testutils.TestContract) {
// 					testutils.RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *testutils.EOATestAccount) {
// 						handler := setupHandler(t, chainID, backend, rootAddr)
// 						gasFeeCollector := testutils.RandomAddress(t)
// 						for i := 0; i < 10; i++ {
// 							tx := testAccount.PrepareSignAndEncodeTx(t,
// 								testContract.DeployedAt.ToCommon(),
// 								testContract.MakeCallData(t, "store", big.NewInt(int64(i))),
// 								big.NewInt(0),
// 								uint64(100_000),
// 								big.NewInt(1),
// 							)
// 							rs := handler.Run(tx, gasFeeCollector)
// 							require.Equal(t, rs.ErrorCode, 0)
// 						}
// 						handler.CommitBlockProposal()
// 						events = backend.Events()
// 					})
// 				})
// 		})
// 	})

// 	storage := testutils.GetSimpleValueStore()
// 	sync.NewChainReplayer(chainID, storage, logger)
// 	//

// 	// all sorts of transactions

// 	// transactions that checks the value of the previous call and increment it

// }

// func setupHandler(t testing.TB,
// 	chainID flow.ChainID,
// 	backend types.Backend,
// 	rootAddr flow.Address,
// ) *handler.ContractHandler {
// 	return handler.NewContractHandler(
// 		chainID,
// 		rootAddr,
// 		common.MustBytesToAddress(systemcontracts.SystemContractsForChain(chainID).FlowToken.Address.Bytes()),
// 		rootAddr,
// 		handler.NewBlockStore(chainID, backend, rootAddr),
// 		handler.NewAddressAllocator(),
// 		backend,
// 		emulator.NewEmulator(backend, rootAddr),
// 		debug.NopTracer,
// 	)
// }
