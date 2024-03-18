package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	protocolint "github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/engine/access/index"

	"github.com/onflow/flow-go/utils/unittest/mocks"

	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	subscriptionmock "github.com/onflow/flow-go/engine/access/subscription/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type TransactionStatusSuite struct {
	suite.Suite

	state          *protocol.State
	sealedSnapshot *protocol.Snapshot
	finalSnapshot  *protocol.Snapshot
	tempSnapshot   *protocol.Snapshot
	log            zerolog.Logger

	blocks             *storagemock.Blocks
	headers            *storagemock.Headers
	collections        *storagemock.Collections
	transactions       *storagemock.Transactions
	receipts           *storagemock.ExecutionReceipts
	results            *storagemock.ExecutionResults
	transactionResults *storagemock.LightTransactionResults
	events             *storagemock.Events
	seals              *storagemock.Seals

	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	archiveClient          *access.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory
	communicator      *backendmock.Communicator
	blockTracker      *subscriptionmock.BlockTracker
	reporter          *syncmock.IndexReporter

	chainID flow.ChainID

	broadcaster    *engine.Broadcaster
	rootBlock      flow.Block
	sealedBlock    *flow.Block
	finalizedBlock *flow.Block

	blockMap   map[uint64]*flow.Block
	resultsMap map[flow.Identifier]*flow.ExecutionResult

	backend *Backend
}

func TestTransactionStatusSuite(t *testing.T) {
	suite.Run(t, new(TransactionStatusSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *TransactionStatusSuite) SetupTest() {
	s.log = zerolog.New(zerolog.NewConsoleWriter())
	s.state = protocol.NewState(s.T())
	s.sealedSnapshot = protocol.NewSnapshot(s.T())
	s.finalSnapshot = protocol.NewSnapshot(s.T())
	s.tempSnapshot = &protocol.Snapshot{}

	header := unittest.BlockHeaderFixture()

	params := protocol.NewParams(s.T())
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	s.state.On("Params").Return(params)

	s.blocks = storagemock.NewBlocks(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.transactions = storagemock.NewTransactions(s.T())
	s.collections = storagemock.NewCollections(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.results = storagemock.NewExecutionResults(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.colClient = access.NewAccessAPIClient(s.T())
	s.archiveClient = access.NewAccessAPIClient(s.T())
	s.execClient = access.NewExecutionAPIClient(s.T())
	s.transactionResults = storagemock.NewLightTransactionResults(s.T())
	s.events = storagemock.NewEvents(s.T())
	s.chainID = flow.Testnet
	s.historicalAccessClient = access.NewAccessAPIClient(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.communicator = backendmock.NewCommunicator(s.T())
	s.broadcaster = engine.NewBroadcaster()
	s.blockTracker = subscriptionmock.NewBlockTracker(s.T())
	s.resultsMap = map[flow.Identifier]*flow.ExecutionResult{}

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	rootResult := unittest.ExecutionResultFixture(unittest.WithBlock(&s.rootBlock))
	s.resultsMap[s.rootBlock.ID()] = rootResult

	s.sealedBlock = &s.rootBlock
	s.finalizedBlock = unittest.BlockWithParentFixture(s.sealedBlock.Header)
	finalizedResult := unittest.ExecutionResultFixture(unittest.WithBlock(s.finalizedBlock))
	s.resultsMap[s.finalizedBlock.ID()] = finalizedResult
	s.blockMap = map[uint64]*flow.Block{
		s.sealedBlock.Header.Height:    s.sealedBlock,
		s.finalizedBlock.Header.Height: s.finalizedBlock,
	}

	s.reporter = syncmock.NewIndexReporter(s.T())

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(mocks.StorageMapGetter(s.blockMap))

	s.state.On("Final").Return(s.finalSnapshot, nil)
	s.state.On("AtBlockID", mock.AnythingOfType("flow.Identifier")).Return(func(blockID flow.Identifier) protocolint.Snapshot {
		s.tempSnapshot.On("Head").Unset()
		s.tempSnapshot.On("Head").Return(func() *flow.Header {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header
				}
			}

			return nil
		}, nil)

		return s.tempSnapshot
	}, nil)

	s.finalSnapshot.On("Head").Return(func() *flow.Header {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader
	}, nil)

	s.blockTracker.On("GetStartHeightFromBlockID", mock.Anything).Return(func(_ flow.Identifier) (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil)
	s.blockTracker.On("GetHighestHeight", flow.BlockStatusFinalized).Return(func(_ flow.BlockStatus) (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil)

	backendParams := s.backendParams()
	err := backendParams.TxResultsIndex.Initialize(s.reporter)
	require.NoError(s.T(), err)

	s.backend, err = New(backendParams)
	require.NoError(s.T(), err)

}

// backendParams returns the Params configuration for the backend.
func (s *TransactionStatusSuite) backendParams() Params {
	return Params{
		State:                    s.state,
		Blocks:                   s.blocks,
		Headers:                  s.headers,
		Collections:              s.collections,
		Transactions:             s.transactions,
		ExecutionReceipts:        s.receipts,
		ExecutionResults:         s.results,
		ChainID:                  s.chainID,
		CollectionRPC:            s.colClient,
		MaxHeightRange:           DefaultMaxHeightRange,
		SnapshotHistoryLimit:     DefaultSnapshotHistoryLimit,
		Communicator:             NewNodeCommunicator(false),
		AccessMetrics:            metrics.NewNoopCollector(),
		Log:                      s.log,
		TxErrorMessagesCacheSize: 1000,
		BlockTracker:             s.blockTracker,
		SubscriptionParams: SubscriptionParams{
			SendTimeout:    subscription.DefaultSendTimeout,
			SendBufferSize: subscription.DefaultSendBufferSize,
			ResponseLimit:  subscription.DefaultResponseLimit,
			Broadcaster:    s.broadcaster,
		},
		TxResultsIndex: index.NewTransactionResultsIndex(s.transactionResults),
		EventsIndex:    index.NewEventsIndex(s.events),
	}
}

func (s *TransactionStatusSuite) addNewFinalizedBlock(parent *flow.Header, notify bool, options ...func(*flow.Block)) {
	s.finalizedBlock = unittest.BlockWithParentFixture(parent)
	for _, option := range options {
		option(s.finalizedBlock)
	}

	s.blockMap[s.finalizedBlock.Header.Height] = s.finalizedBlock

	if notify {
		s.broadcaster.Publish()
	}
}

// TestSubscribeTransactionStatusHappyCase tests the functionality of the SubscribeTransactionStatuses method in the Backend.
// It covers the emulation of transaction stages from pending to sealed, and receiving status updates.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusHappyCase() {
	ctx, cancel := context.WithCancel(context.Background())

	s.sealedSnapshot.On("Head").Return(func() *flow.Header {
		return s.sealedBlock.Header
	}, nil)
	s.state.On("Sealed").Return(s.sealedSnapshot, nil)
	s.results.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(mocks.StorageMapGetter(s.resultsMap))

	// Generate sent transaction with ref block of the current finalized block
	transaction := unittest.TransactionFixture()
	transaction.SetReferenceBlockID(s.finalizedBlock.ID())
	col := flow.CollectionFromTransactions([]*flow.Transaction{&transaction})
	guarantee := col.Guarantee()
	light := col.Light()
	txId := transaction.ID()

	expectedMsgIndexCounter := counters.NewMonotonousCounter(0)

	// Create a special common function to read subscription messages from the channel and check converting it to transaction info
	// and check results for correctness
	checkNewSubscriptionMessage := func(sub subscription.Subscription, expectedTxStatus flow.TransactionStatus) {
		unittest.RequireReturnsBefore(s.T(), func() {
			v, ok := <-sub.Channel()
			require.True(s.T(), ok,
				"channel closed while waiting for transaction info:\n\t- txID %x\n\t- blockID: %x \n\t- err: %v",
				txId, s.finalizedBlock.ID(), sub.Err())

			txInfo, ok := v.(*convert.TransactionSubscribeInfo)
			require.True(s.T(), ok, "unexpected response type: %T", v)

			assert.Equal(s.T(), txId, txInfo.ID)
			assert.Equal(s.T(), expectedTxStatus, txInfo.Status)

			expectedMsgIndex := expectedMsgIndexCounter.Value()
			assert.Equal(s.T(), expectedMsgIndex, txInfo.MessageIndex)
			wasSet := expectedMsgIndexCounter.Set(expectedMsgIndex + 1)
			require.True(s.T(), wasSet)
		}, 60*time.Second, fmt.Sprintf("timed out waiting for transaction info:\n\t- txID: %x\n\t- blockID: %x", txId, s.finalizedBlock.ID()))
	}

	// 1. Subscribe to transaction status and receive the first message with pending status
	sub := s.backend.SubscribeTransactionStatuses(ctx, &transaction.TransactionBody)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusPending)

	// 2. Make transaction reference block sealed, and add a new finalized block that includes the transaction
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true, func(block *flow.Block) {
		block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))
		s.collections.On("LightByID", mock.AnythingOfType("flow.Identifier")).Return(&light, nil).Maybe()
	})
	checkNewSubscriptionMessage(sub, flow.TransactionStatusFinalized)

	// 3. Add one more finalized block on top of the transaction block and add execution results to storage
	finalizedResult := unittest.ExecutionResultFixture(unittest.WithBlock(s.finalizedBlock))
	s.resultsMap[s.finalizedBlock.ID()] = finalizedResult

	s.addNewFinalizedBlock(s.finalizedBlock.Header, true)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusExecuted)

	// 4. Make the transaction block sealed, and add a new finalized block
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusSealed)

	// 5. Stop subscription
	cancel()

	// Ensure subscription shuts down gracefully
	unittest.RequireReturnsBefore(s.T(), func() {
		v, ok := <-sub.Channel()
		assert.Nil(s.T(), v)
		assert.False(s.T(), ok)
		assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
}

// TestSubscribeTransactionStatusExpired tests the functionality of the SubscribeTransactionStatuses method in the Backend
// when transaction become expired
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusExpired() {
	ctx, cancel := context.WithCancel(context.Background())

	s.blocks.On("GetLastFullBlockHeight").Return(func() (uint64, error) {
		return s.sealedBlock.Header.Height, nil
	}, nil)

	// Generate sent transaction with ref block of the current finalized block
	transaction := unittest.TransactionFixture()
	transaction.SetReferenceBlockID(s.finalizedBlock.ID())
	txId := transaction.ID()

	expectedMsgIndexCounter := counters.NewMonotonousCounter(0)

	// Create a special common function to read subscription messages from the channel and check converting it to transaction info
	// and check results for correctness
	checkNewSubscriptionMessage := func(sub subscription.Subscription, expectedTxStatus flow.TransactionStatus) {
		unittest.RequireReturnsBefore(s.T(), func() {
			v, ok := <-sub.Channel()
			require.True(s.T(), ok,
				"channel closed while waiting for transaction info:\n\t- txID %x\n\t- blockID: %x \n\t- err: %v",
				txId, s.finalizedBlock.ID(), sub.Err())

			txInfo, ok := v.(*convert.TransactionSubscribeInfo)
			require.True(s.T(), ok, "unexpected response type: %T", v)

			assert.Equal(s.T(), txId, txInfo.ID)
			assert.Equal(s.T(), expectedTxStatus, txInfo.Status)

			expectedMsgIndex := expectedMsgIndexCounter.Value()
			assert.Equal(s.T(), expectedMsgIndex, txInfo.MessageIndex)
			wasSet := expectedMsgIndexCounter.Set(expectedMsgIndex + 1)
			require.True(s.T(), wasSet)
		}, time.Second, fmt.Sprintf("timed out waiting for transaction info:\n\t- txID: %x\n\t- blockID: %x", txId, s.finalizedBlock.ID()))
	}

	// Subscribe to transaction status and receive the first message with pending status
	sub := s.backend.SubscribeTransactionStatuses(ctx, &transaction.TransactionBody)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusPending)

	// Generate 600 blocks without transaction included and check, that transaction still pending
	startHeight := s.finalizedBlock.Header.Height + 1
	lastHeight := startHeight + flow.DefaultTransactionExpiry

	for i := startHeight; i <= lastHeight; i++ {
		s.sealedBlock = s.finalizedBlock
		s.addNewFinalizedBlock(s.sealedBlock.Header, false)
	}

	// Generate final blocks and check transaction expired
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusExpired)

	// Stop subscription
	cancel()

	// Ensure subscription shuts down gracefully
	unittest.RequireReturnsBefore(s.T(), func() {
		v, ok := <-sub.Channel()
		assert.Nil(s.T(), v)
		assert.False(s.T(), ok)
		assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
}
