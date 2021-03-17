package utils

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	completeLedger "github.com/onflow/flow-go/ledger/complete"

	fvmMock "github.com/onflow/flow-go/fvm/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// CompleteExecutionResult represents an execution result that is ready to
// be verified. It contains all execution result and all resources required to
// verify it.
// TODO update this as needed based on execution requirements
type CompleteExecutionResult struct {
	ContainerBlock *flow.Block // block that contains execution receipt of reference block
	ReferenceBlock *flow.Block // block that execution receipt refers to
	Receipt        *flow.ExecutionReceipt
	Collections    []*flow.Collection
	ChunkDataPacks []*flow.ChunkDataPack
	SpockSecrets   [][]byte
}

// CompleteExecutionResultFixture returns complete execution result with an
// execution receipt referencing the block/collections.
// chunkCount determines the number of chunks inside each receipt.
// The output is an execution result with `chunkCount`+1 chunks, where the last chunk accounts
// for the system chunk.
func CompleteExecutionResultFixture(t *testing.T, chunkCount int, chain flow.Chain, root *flow.Header) CompleteExecutionResult {
	// setups up the first collection of block consists of three transactions
	tx1 := testutil.DeployCounterContractTransaction(chain.ServiceAddress(), chain)
	err := testutil.SignTransactionAsServiceAccount(tx1, 0, chain)
	require.NoError(t, err)
	tx2 := testutil.CreateCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = testutil.SignTransactionAsServiceAccount(tx2, 1, chain)
	require.NoError(t, err)
	tx3 := testutil.CreateCounterPanicTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = testutil.SignTransactionAsServiceAccount(tx3, 2, chain)
	require.NoError(t, err)
	transactions := []*flow.TransactionBody{tx1, tx2, tx3}
	collection := flow.Collection{Transactions: transactions}
	collections := []*flow.Collection{&collection}
	guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollection(&collection), unittest.WithCollRef(root.ID()))
	guarantees := []*flow.CollectionGuarantee{guarantee}

	metricsCollector := &metrics.NoopCollector{}
	log := zerolog.Nop()

	// setups execution outputs:
	spockSecrets := make([][]byte, 0)
	chunks := make([]*flow.Chunk, 0)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0)

	var payload flow.Payload
	var referenceBlock flow.Block
	header := unittest.BlockHeaderWithParentFixture(root)

	unittest.RunWithTempDir(t, func(dir string) {
		led, err := completeLedger.NewLedger(dir, 100, metricsCollector, zerolog.Nop(), nil, completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)
		defer led.Done()

		startStateCommitment, err := bootstrap.NewBootstrapper(log).BootstrapLedger(
			led,
			unittest.ServiceAccountPublicKey,
			unittest.GenesisTokenSupply,
			chain,
		)
		require.NoError(t, err)

		rt := runtime.NewInterpreterRuntime()

		vm := fvm.New(rt)

		blocks := new(fvmMock.Blocks)

		execCtx := fvm.NewContext(
			log,
			fvm.WithChain(chain),
			fvm.WithBlocks(blocks),
		)

		// create state.View
		view := delta.NewView(state.LedgerGetRegister(led, startStateCommitment))
		programs := fvm.NewEmptyPrograms()

		// create BlockComputer
		bc, err := computer.NewBlockComputer(vm, execCtx, nil, nil, log)
		require.NoError(t, err)

		for i := 1; i < chunkCount; i++ {
			tx := testutil.CreateCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
			err = testutil.SignTransactionAsServiceAccount(tx, 3+uint64(i), chain)
			require.NoError(t, err)

			collection := flow.Collection{Transactions: []*flow.TransactionBody{tx}}
			guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollection(&collection), unittest.WithCollRef(root.ID()))
			collections = append(collections, &collection)
			guarantees = append(guarantees, guarantee)
		}

		// generates system chunk collection and guarantee as the last collection of the block
		sysCollection, sysGuarantee := SystemChunkCollectionFixture(chain.ServiceAddress())
		collections = append(collections, sysCollection)
		sysGuarantee.ReferenceBlockID = root.ID()
		guarantees = append(guarantees, sysGuarantee)

		// shapes the block out of collections
		payload = flow.Payload{
			Guarantees: guarantees,
		}
		header.PayloadHash = payload.Hash()
		referenceBlock = flow.Block{
			Header:  &header,
			Payload: &payload,
		}
		blockID := referenceBlock.ID()

		// executes collections
		for i := 0; i < len(collections); i++ {
			collection := collections[i]
			guarantee := guarantees[i]
			chunk, chunkDataPack, endStateCommitment, spock := executeCollection(t,
				blockID,
				collection,
				guarantee,
				uint(i),
				startStateCommitment,
				view,
				programs,
				bc,
				led)

			// *execution.ComputationResult, error
			chunks = append(chunks, chunk)
			chunkDataPacks = append(chunkDataPacks, chunkDataPack)
			spockSecrets = append(spockSecrets, spock)
			startStateCommitment = endStateCommitment
		}

	})

	// makes sure all chunks are referencing the correct block id.
	blockID := referenceBlock.ID()
	for _, chunk := range chunks {
		require.Equal(t, blockID, chunk.BlockID, "inconsistent block id in chunk fixture")
	}

	result := flow.ExecutionResult{
		BlockID: blockID,
		Chunks:  chunks,
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	// container block is the block that contains the execution receipt of reference block
	containerBlock := unittest.BlockWithParentFixture(referenceBlock.Header, unittest.WithoutGuarantee)
	containerBlock.Payload.Receipts = []*flow.ExecutionReceipt{receipt}
	containerBlock.Header.PayloadHash = containerBlock.Payload.Hash()

	return CompleteExecutionResult{
		Receipt:        receipt,
		ReferenceBlock: &referenceBlock,
		ContainerBlock: &containerBlock,
		Collections:    collections,
		ChunkDataPacks: chunkDataPacks,
		SpockSecrets:   spockSecrets,
	}

}

// LightExecutionResultFixture returns a light mocked version of execution result with an
// execution receipt referencing the block/collections. In the light version of execution result,
// everything is wired properly, but with the minimum viable content provided. This version is basically used
// for profiling.
func LightExecutionResultFixture(chunkCount int) CompleteExecutionResult {
	collections := make([]*flow.Collection, 0, chunkCount)
	guarantees := make([]*flow.CollectionGuarantee, 0, chunkCount)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0, chunkCount)

	// creates collections and guarantees
	for i := 0; i < chunkCount; i++ {
		coll := unittest.CollectionFixture(1)
		guarantee := coll.Guarantee()
		collections = append(collections, &coll)
		guarantees = append(guarantees, &guarantee)
	}

	payload := flow.Payload{
		Guarantees: guarantees,
	}

	header := unittest.BlockHeaderFixture()
	header.Height = 0
	header.PayloadHash = payload.Hash()

	referenceBlock := flow.Block{
		Header:  &header,
		Payload: &payload,
	}
	blockID := referenceBlock.ID()

	// creates chunks
	chunks := make([]*flow.Chunk, 0)
	for i := 0; i < chunkCount; i++ {
		chunk := &flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				BlockID:         blockID,
				EventCollection: unittest.IdentifierFixture(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, chunk)

		// creates a light (quite empty) chunk data pack for the chunk at bare minimum
		chunkDataPack := flow.ChunkDataPack{
			ChunkID: chunk.ID(),
		}
		chunkDataPacks = append(chunkDataPacks, &chunkDataPack)
	}

	result := flow.ExecutionResult{
		BlockID: blockID,
		Chunks:  chunks,
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	// container block contains the execution receipt and points back to reference block
	// as its parent.
	containerBlock := unittest.BlockWithParentFixture(referenceBlock.Header, unittest.WithoutGuarantee)
	containerBlock.Payload.Receipts = []*flow.ExecutionReceipt{receipt}

	return CompleteExecutionResult{
		Receipt:        receipt,
		ReferenceBlock: &referenceBlock,
		ContainerBlock: &containerBlock,
		Collections:    collections,
		ChunkDataPacks: chunkDataPacks,
	}
}

func SystemChunkCollectionFixture(serviceAddress flow.Address) (*flow.Collection, *flow.CollectionGuarantee) {
	tx := fvm.SystemChunkTransaction(serviceAddress)
	collection := &flow.Collection{
		Transactions: []*flow.TransactionBody{tx},
	}

	guarantee := collection.Guarantee()

	return collection, &guarantee
}

// executeCollection receives a collection, its guarantee, and its starting state commitment.
// It executes the collection and returns its corresponding chunk, chunk data pack, end state, and spock.
func executeCollection(
	t *testing.T,
	blockID flow.Identifier,
	collection *flow.Collection,
	guarantee *flow.CollectionGuarantee,
	chunkIndex uint,
	startStateCommitment flow.StateCommitment,
	view *delta.View,
	programs *fvm.Programs,
	bc computer.BlockComputer,
	led *completeLedger.Ledger) (*flow.Chunk, *flow.ChunkDataPack, flow.StateCommitment, []byte) {

	completeColls := make(map[flow.Identifier]*entity.CompleteCollection)
	completeColls[guarantee.ID()] = &entity.CompleteCollection{
		Guarantee:    guarantee,
		Transactions: collection.Transactions,
	}

	// creates a temporary block to compute intermediate state
	header := unittest.BlockHeaderFixture()
	block := &flow.Block{
		Header: &header,
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeColls,
		StartState:          startStateCommitment,
	}

	// *execution.ComputationResult, error
	computationResult, err := bc.ExecuteBlock(context.Background(), executableBlock, view, programs)
	require.NoError(t, err, "error executing block")
	spock := computationResult.StateSnapshots[0].SpockSecret

	ids, values := view.Delta().RegisterUpdates()
	keys := state.RegisterIDSToKeys(ids)
	flowValues := state.RegisterValuesToValues(values)

	update, err := ledger.NewUpdate(startStateCommitment, keys, flowValues)
	require.NoError(t, err)

	// TODO: update CommitDelta to also return proofs
	endStateCommitment, err := led.Set(update)
	require.NoError(t, err, "error updating registers")

	chunk := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: chunkIndex,
			StartState:      startStateCommitment,
			// TODO: include event collection hash
			EventCollection: flow.ZeroID,
			BlockID:         blockID,
			// TODO: record gas used
			TotalComputationUsed: 0,
			// TODO: record number of txs
			NumberOfTransactions: 0,
		},
		Index:    uint64(chunkIndex),
		EndState: endStateCommitment,
	}

	// chunkDataPack
	allRegisters := view.Interactions().AllRegisters()
	allKeys := state.RegisterIDSToKeys(allRegisters)

	query, err := ledger.NewQuery(chunk.StartState, allKeys)
	require.NoError(t, err)

	//values, proofs, err := led.GetRegistersWithProof(allRegisters, chunk.StartState)
	proof, err := led.Prove(query)
	require.NoError(t, err, "error reading registers with proofs from ledger")

	chunkDataPack := &flow.ChunkDataPack{
		ChunkID:      chunk.ID(),
		StartState:   chunk.StartState,
		Proof:        proof,
		CollectionID: collection.ID(),
	}

	return chunk, chunkDataPack, endStateCommitment, spock
}

func CompleteExecutionResultChainFixture(t *testing.T, root *flow.Header, count int) []*CompleteExecutionResult {
	results := make([]*CompleteExecutionResult, 0, count)
	parent := root
	for i := 0; i < count; i++ {
		result := CompleteExecutionResultFixture(t, 1, flow.Testnet.Chain(), parent)
		results = append(results, &result)
	}

	return results
}
