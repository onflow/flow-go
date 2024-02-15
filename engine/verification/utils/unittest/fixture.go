package verificationtest

import (
	"context"
	"math/rand"
	"testing"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	exstate "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/derived"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/cluster"

	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	moduleMock "github.com/onflow/flow-go/module/mock"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// TODO: enable parallel execution once cadence type equivalence check issue
	// is resolved.
	testMaxConcurrency = 1
)

// ExecutionReceiptData is a test helper struct that represents all data required
// to verify the result of an execution receipt.
type ExecutionReceiptData struct {
	ReferenceBlock *flow.Block // block that execution receipt refers to
	ChunkDataPacks []*flow.ChunkDataPack
	SpockSecrets   [][]byte
}

// CompleteExecutionReceipt is a test helper struct that represents a container block accompanied with all
// data required to verify its execution receipts.
// TODO update this as needed based on execution requirements
type CompleteExecutionReceipt struct {
	ContainerBlock *flow.Block // block that contains execution receipt of reference block

	// TODO: this is a temporary field to support finder engine logic
	// It should be removed once we replace finder engine.
	Receipts     []*flow.ExecutionReceipt // copy of execution receipts in container block
	ReceiptsData []*ExecutionReceiptData  // execution receipts data of the container block
}

type CompleteExecutionReceiptList []*CompleteExecutionReceipt

// ChunkDataResponseOf is a test helper method that returns a chunk data pack response message for the specified chunk ID that
// should belong to this complete execution receipt list.
//
// It fails the test if no chunk with specified chunk ID is found in this complete execution receipt list.
func (c CompleteExecutionReceiptList) ChunkDataResponseOf(t *testing.T, chunkID flow.Identifier) *messages.ChunkDataResponse {
	_, chunkIndex := c.resultOf(t, chunkID)
	receiptData := c.ReceiptDataOf(t, chunkID)

	// publishes the chunk data pack response to the network
	res := &messages.ChunkDataResponse{
		ChunkDataPack: *receiptData.ChunkDataPacks[chunkIndex],
		Nonce:         rand.Uint64(),
	}

	return res
}

// ChunkOf is a test helper method that returns the chunk of the specified index from the specified result that
// should belong to this complete execution receipt list.
//
// It fails the test if no execution result with the specified identifier is found in this complete execution receipt list.
func (c CompleteExecutionReceiptList) ChunkOf(t *testing.T, resultID flow.Identifier, chunkIndex uint64) *flow.Chunk {
	for _, completeER := range c {
		for _, result := range completeER.ContainerBlock.Payload.Results {
			if result.ID() == resultID {
				return result.Chunks[chunkIndex]
			}
		}
	}

	require.Fail(t, "could not find specified chunk in the complete execution result list")
	return nil
}

// ReceiptDataOf is a test helper method that returns the receipt data of the specified chunk ID that
// should belong to this complete execution receipt list.
//
// It fails the test if no chunk with specified chunk ID is found in this complete execution receipt list.
func (c CompleteExecutionReceiptList) ReceiptDataOf(t *testing.T, chunkID flow.Identifier) *ExecutionReceiptData {
	for _, completeER := range c {
		for _, receiptData := range completeER.ReceiptsData {
			for _, cdp := range receiptData.ChunkDataPacks {
				if cdp.ChunkID == chunkID {
					return receiptData
				}
			}
		}
	}

	require.Fail(t, "could not find receipt data of specified chunk in the complete execution result list")
	return nil
}

// resultOf is a test helper method that returns the execution result and chunk index of the specified chunk ID that
// should belong to this complete execution receipt list.
//
// It fails the test if no chunk with specified chunk ID is found in this complete execution receipt list.
func (c CompleteExecutionReceiptList) resultOf(t *testing.T, chunkID flow.Identifier) (*flow.ExecutionResult, uint64) {
	for _, completeER := range c {
		for _, result := range completeER.ContainerBlock.Payload.Results {
			for _, chunk := range result.Chunks {
				if chunk.ID() == chunkID {
					return result, chunk.Index
				}
			}
		}
	}

	require.Fail(t, "could not find specified chunk in the complete execution result list")
	return nil, uint64(0)
}

// CompleteExecutionReceiptBuilder is a test helper struct that specifies the parameters to build a CompleteExecutionReceipt.
type CompleteExecutionReceiptBuilder struct {
	resultsCount     int // number of execution results in the container block.
	executorCount    int // number of times each execution result is copied in a block (by different receipts).
	chunksCount      int // number of chunks in each execution result.
	chain            flow.Chain
	executorIDs      flow.IdentifierList // identifier of execution nodes in the test.
	clusterCommittee flow.IdentityList
}

type CompleteExecutionReceiptBuilderOpt func(builder *CompleteExecutionReceiptBuilder)

func WithResults(count int) CompleteExecutionReceiptBuilderOpt {
	return func(builder *CompleteExecutionReceiptBuilder) {
		builder.resultsCount = count
	}
}

func WithChunksCount(count int) CompleteExecutionReceiptBuilderOpt {
	return func(builder *CompleteExecutionReceiptBuilder) {
		builder.chunksCount = count
	}
}

func WithCopies(count int) CompleteExecutionReceiptBuilderOpt {
	return func(builder *CompleteExecutionReceiptBuilder) {
		builder.executorCount = count
	}
}

func WithChain(chain flow.Chain) CompleteExecutionReceiptBuilderOpt {
	return func(builder *CompleteExecutionReceiptBuilder) {
		builder.chain = chain
	}
}

func WithExecutorIDs(executorIDs flow.IdentifierList) CompleteExecutionReceiptBuilderOpt {
	return func(builder *CompleteExecutionReceiptBuilder) {
		builder.executorIDs = executorIDs
	}
}

func WithClusterCommittee(clusterCommittee flow.IdentityList) CompleteExecutionReceiptBuilderOpt {
	return func(builder *CompleteExecutionReceiptBuilder) {
		builder.clusterCommittee = clusterCommittee
	}
}

// ExecutionResultFixture is a test helper that returns an execution result for the reference block header as well as the execution receipt data
// for that result.
func ExecutionResultFixture(t *testing.T,
	chunkCount int,
	chain flow.Chain,
	refBlkHeader *flow.Header,
	protocolStateID flow.Identifier,
	clusterCommittee flow.IdentityList,
	source []byte,
) (*flow.ExecutionResult, *ExecutionReceiptData) {

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
	clusterChainID := cluster.CanonicalClusterID(1, clusterCommittee.NodeIDs())

	guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollection(&collection), unittest.WithCollRef(refBlkHeader.ParentID))
	guarantee.ChainID = clusterChainID
	indices, err := signature.EncodeSignersToIndices(clusterCommittee.NodeIDs(), clusterCommittee.NodeIDs())
	require.NoError(t, err)
	guarantee.SignerIndices = indices
	guarantees := []*flow.CollectionGuarantee{guarantee}

	metricsCollector := &metrics.NoopCollector{}
	log := zerolog.Nop()

	// setups execution outputs:
	var referenceBlock flow.Block
	var spockSecrets [][]byte
	var chunkDataPacks []*flow.ChunkDataPack
	var result *flow.ExecutionResult

	unittest.RunWithTempDir(t, func(dir string) {

		w := &fixtures.NoopWAL{}

		led, err := completeLedger.NewLedger(w, 100, metricsCollector, zerolog.Nop(), completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()

		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		// set 0 clusters to pass n_collectors >= n_clusters check
		epochConfig := epochs.DefaultEpochConfig()
		epochConfig.NumCollectorClusters = 0
		startStateCommitment, err := bootstrap.NewBootstrapper(log).BootstrapLedger(
			led,
			unittest.ServiceAccountPublicKey,
			chain,
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
			fvm.WithEpochConfig(epochConfig),
		)
		require.NoError(t, err)

		vm := fvm.NewVirtualMachine()

		blocks := new(envMock.Blocks)

		execCtx := fvm.NewContext(
			fvm.WithLogger(log),
			fvm.WithChain(chain),
			fvm.WithBlocks(blocks),
		)

		// create state.View
		snapshot := exstate.NewLedgerStorageSnapshot(
			led,
			startStateCommitment)
		committer := committer.NewLedgerViewCommitter(led, trace.NewNoopTracer())

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		me := new(moduleMock.Local)
		me.On("NodeID").Return(unittest.IdentifierFixture())
		me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
		me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil)

		// create BlockComputer
		bc, err := computer.NewBlockComputer(
			vm,
			execCtx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			log,
			committer,
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(source),
			testMaxConcurrency)
		require.NoError(t, err)

		completeColls := make(map[flow.Identifier]*entity.CompleteCollection)
		completeColls[guarantee.ID()] = &entity.CompleteCollection{
			Guarantee:    guarantee,
			Transactions: collection.Transactions,
		}

		for i := 1; i < chunkCount; i++ {
			tx := testutil.CreateCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
			err = testutil.SignTransactionAsServiceAccount(tx, 3+uint64(i), chain)
			require.NoError(t, err)

			collection := flow.Collection{Transactions: []*flow.TransactionBody{tx}}
			guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollection(&collection), unittest.WithCollRef(refBlkHeader.ParentID))
			guarantee.SignerIndices = indices
			guarantee.ChainID = clusterChainID

			collections = append(collections, &collection)
			guarantees = append(guarantees, guarantee)

			completeColls[guarantee.ID()] = &entity.CompleteCollection{
				Guarantee:    guarantee,
				Transactions: collection.Transactions,
			}
		}

		payload := flow.Payload{
			Guarantees:      guarantees,
			ProtocolStateID: protocolStateID,
		}
		referenceBlock = flow.Block{
			Header: refBlkHeader,
		}
		referenceBlock.SetPayload(payload)

		executableBlock := &entity.ExecutableBlock{
			Block:               &referenceBlock,
			CompleteCollections: completeColls,
			StartState:          &startStateCommitment,
		}
		computationResult, err := bc.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			executableBlock,
			snapshot,
			derived.NewEmptyDerivedBlockData(0))
		require.NoError(t, err)

		for _, snapshot := range computationResult.AllExecutionSnapshots() {
			spockSecrets = append(spockSecrets, snapshot.SpockSecret)
		}

		chunkDataPacks = computationResult.AllChunkDataPacks()
		result = &computationResult.ExecutionResult
	})

	return result, &ExecutionReceiptData{
		ReferenceBlock: &referenceBlock,
		ChunkDataPacks: chunkDataPacks,
		SpockSecrets:   spockSecrets,
	}
}

// CompleteExecutionReceiptChainFixture is a test fixture that creates a chain of blocks of size `count`.
// The chain is in the form of root <- R1,1 <- R1,2 <- ... <- C1 <- R2,1 <- R2,2 <- ... <- C2 <- ...
// In this chain R refers to reference blocks that contain guarantees.
// C refers to a container block that contains an execution receipt for its preceding reference blocks.
// e.g., C1 contains an execution receipt for R1,1, R1,2, etc., and C2 contains a receipt for R2,1, R2,2, etc.
// For sake of simplicity and test, container blocks (i.e., C) do not contain any guarantee.
//
// It returns a slice of complete execution receipt fixtures that contains a container block as well as all data to verify its contained receipts.
func CompleteExecutionReceiptChainFixture(t *testing.T,
	root *flow.Header,
	rootProtocolStateID flow.Identifier,
	count int,
	sources [][]byte,
	opts ...CompleteExecutionReceiptBuilderOpt,
) []*CompleteExecutionReceipt {
	completeERs := make([]*CompleteExecutionReceipt, 0, count)
	parent := root

	builder := &CompleteExecutionReceiptBuilder{
		resultsCount:  1,
		executorCount: 1,
		chunksCount:   1,
		chain:         root.ChainID.Chain(),
	}

	for _, apply := range opts {
		apply(builder)
	}

	if len(builder.executorIDs) == 0 {
		builder.executorIDs = unittest.IdentifierListFixture(builder.executorCount)
	}

	require.GreaterOrEqual(t, len(builder.executorIDs), builder.executorCount,
		"number of executors in the tests should be greater than or equal to the number of receipts per block")

	var sourcesIndex = 0
	for i := 0; i < count; i++ {
		// Generates two blocks as parent <- R <- C where R is a reference block containing guarantees,
		// and C is a container block containing execution receipt for R.
		receipts, allData, head := ExecutionReceiptsFromParentBlockFixture(t, parent, rootProtocolStateID, builder, sources[sourcesIndex:])
		sourcesIndex += builder.resultsCount
		containerBlock := ContainerBlockFixture(head, rootProtocolStateID, receipts, sources[sourcesIndex])
		sourcesIndex++
		completeERs = append(completeERs, &CompleteExecutionReceipt{
			ContainerBlock: containerBlock,
			Receipts:       receipts,
			ReceiptsData:   allData,
		})

		parent = containerBlock.Header
	}
	return completeERs
}

// ExecutionReceiptsFromParentBlockFixture creates a chain of receipts from a parent block.
//
// By default each result refers to a distinct reference block, and it extends the block chain after generating each
// result (i.e., for the next result).
//
// Each result may appear in more than one receipt depending on the builder parameters.
func ExecutionReceiptsFromParentBlockFixture(t *testing.T,
	parent *flow.Header,
	protocolStateID flow.Identifier,
	builder *CompleteExecutionReceiptBuilder,
	sources [][]byte) (
	[]*flow.ExecutionReceipt,
	[]*ExecutionReceiptData, *flow.Header) {

	allData := make([]*ExecutionReceiptData, 0, builder.resultsCount*builder.executorCount)
	allReceipts := make([]*flow.ExecutionReceipt, 0, builder.resultsCount*builder.executorCount)

	for i := 0; i < builder.resultsCount; i++ {
		result, data := ExecutionResultFromParentBlockFixture(t, parent, protocolStateID, builder, sources[i:])

		// makes several copies of the same result
		for cp := 0; cp < builder.executorCount; cp++ {
			allReceipts = append(allReceipts, &flow.ExecutionReceipt{
				ExecutorID:      builder.executorIDs[cp],
				ExecutionResult: *result,
			})

			allData = append(allData, data)
		}
		parent = data.ReferenceBlock.Header
	}

	return allReceipts, allData, parent
}

// ExecutionResultFromParentBlockFixture is a test helper that creates a child (reference) block from the parent, as well as an execution for it.
func ExecutionResultFromParentBlockFixture(t *testing.T,
	parent *flow.Header,
	protocolStateID flow.Identifier,
	builder *CompleteExecutionReceiptBuilder,
	sources [][]byte,
) (*flow.ExecutionResult, *ExecutionReceiptData) {
	// create the block header including a QC with source a index `i`
	refBlkHeader := unittest.BlockHeaderWithParentWithSoRFixture(parent, sources[0])
	// execute the block with the source a index `i+1` (which will be included later in the child block)
	return ExecutionResultFixture(t, builder.chunksCount, builder.chain, refBlkHeader, protocolStateID, builder.clusterCommittee, sources[1])
}

// ContainerBlockFixture builds and returns a block that contains input execution receipts.
func ContainerBlockFixture(parent *flow.Header, protocolStateID flow.Identifier, receipts []*flow.ExecutionReceipt, source []byte) *flow.Block {
	// container block is the block that contains the execution receipt of reference block
	containerBlock := unittest.BlockWithParentFixture(parent)
	containerBlock.Header.ParentVoterSigData = unittest.QCSigDataWithSoRFixture(source)
	containerBlock.SetPayload(unittest.PayloadFixture(
		unittest.WithReceipts(receipts...),
		unittest.WithProtocolStateID(protocolStateID),
	))

	return containerBlock
}

// ExecutionResultForkFixture creates two conflicting execution results out of the same block ID.
// Each execution result has two chunks.
// First chunks of both results are the same, i.e., have same ID.
// It returns both results, their shared block, and collection corresponding to their first chunk.
func ExecutionResultForkFixture(t *testing.T) (*flow.ExecutionResult, *flow.ExecutionResult, *flow.Collection, *flow.Block) {
	// collection and block
	collections := unittest.CollectionListFixture(1)
	block := unittest.BlockWithGuaranteesFixture(
		unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
	)

	// execution fork at block with resultA and resultB that share first chunk
	resultA := unittest.ExecutionResultFixture(
		unittest.WithBlock(block),
		unittest.WithChunks(2))
	resultB := &flow.ExecutionResult{
		PreviousResultID: resultA.PreviousResultID,
		BlockID:          resultA.BlockID,
		Chunks:           append(flow.ChunkList{resultA.Chunks[0]}, unittest.ChunkListFixture(1, resultA.BlockID)...),
		ServiceEvents:    nil,
	}

	// to be a valid fixture, results A and B must share first chunk.
	require.Equal(t, resultA.Chunks[0].ID(), resultB.Chunks[0].ID())
	// and they must represent a fork
	require.NotEqual(t, resultA.ID(), resultB.ID())

	return resultA, resultB, collections[0], block
}
