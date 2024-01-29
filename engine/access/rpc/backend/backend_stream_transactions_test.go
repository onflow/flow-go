package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
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
	log            zerolog.Logger

	blocks             *storagemock.Blocks
	headers            *storagemock.Headers
	collections        *storagemock.Collections
	transactions       *storagemock.Transactions
	receipts           *storagemock.ExecutionReceipts
	results            *storagemock.ExecutionResults
	transactionResults *storagemock.LightTransactionResults
	seals              *storagemock.Seals

	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	archiveClient          *access.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory
	communicator      *backendmock.Communicator

	chainID flow.ChainID

	broadcaster    *engine.Broadcaster
	rootBlock      flow.Block
	sealedBlock    *flow.Block
	finalizedBlock *flow.Block

	backend *Backend
}

func TestTransactionStatusSuite(t *testing.T) {
	suite.Run(t, new(TransactionStatusSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *TransactionStatusSuite) SetupTest() {
	s.log = zerolog.New(zerolog.NewConsoleWriter())
	s.state = new(protocol.State)
	s.sealedSnapshot = new(protocol.Snapshot)
	s.finalSnapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()

	params := new(protocol.Params)
	params.On("FinalizedRoot").Return(header, nil)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	s.state.On("Params").Return(params)

	s.blocks = new(storagemock.Blocks)
	s.headers = new(storagemock.Headers)
	s.transactions = new(storagemock.Transactions)
	s.collections = new(storagemock.Collections)
	s.receipts = new(storagemock.ExecutionReceipts)
	s.results = new(storagemock.ExecutionResults)
	s.seals = new(storagemock.Seals)
	s.colClient = new(access.AccessAPIClient)
	s.archiveClient = new(access.AccessAPIClient)
	s.execClient = new(access.ExecutionAPIClient)
	s.transactionResults = storagemock.NewLightTransactionResults(s.T())
	s.chainID = flow.Testnet
	s.historicalAccessClient = new(access.AccessAPIClient)
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())

	s.communicator = new(backendmock.Communicator)

	s.broadcaster = engine.NewBroadcaster()

	// generate blockCount consecutive blocks with associated seal, result and execution data
	rootBlockHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0))
	s.rootBlock = unittest.BlockFixture()
	rootBlockHeader.PayloadHash = s.rootBlock.Header.PayloadHash
	s.rootBlock.Header = rootBlockHeader

	s.sealedBlock = &s.rootBlock

	s.finalizedBlock = unittest.BlockWithParentFixture(s.sealedBlock.Header)

	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) flow.Identifier {
			return s.finalizedBlock.Header.ID()
		}, nil)
	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Header {
			return s.finalizedBlock.Header
		}, nil)
	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Block {
			return s.finalizedBlock
		}, nil)

	s.state.On("Sealed").Return(s.sealedSnapshot, nil).Maybe()
	s.state.On("Final").Return(s.finalSnapshot, nil).Maybe()
	s.sealedSnapshot.On("Head").Return(
		func() *flow.Header {
			return s.sealedBlock.Header
		},
		nil,
	).Maybe()
	s.finalSnapshot.On("Head").Return(
		func() *flow.Header {
			return s.finalizedBlock.Header
		},
		nil,
	).Maybe()

	var err error
	s.backend, err = New(s.backendParams())
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
		LightTransactionResults:  s.transactionResults,
		ChainID:                  s.chainID,
		CollectionRPC:            s.colClient,
		MaxHeightRange:           DefaultMaxHeightRange,
		SnapshotHistoryLimit:     DefaultSnapshotHistoryLimit,
		Communicator:             NewNodeCommunicator(false),
		AccessMetrics:            metrics.NewNoopCollector(),
		Log:                      s.log,
		TxErrorMessagesCacheSize: 1000,
		SubscriptionParams: SubscriptionParams{
			SendTimeout:            subscription.DefaultSendTimeout,
			SendBufferSize:         subscription.DefaultSendBufferSize,
			ResponseLimit:          subscription.DefaultResponseLimit,
			Broadcaster:            s.broadcaster,
			RootHeight:             s.rootBlock.Header.Height,
			HighestAvailableHeight: s.rootBlock.Header.Height,
			Seals:                  s.seals,
		},
	}
}

func (s *TransactionStatusSuite) TestSubscribeBlocks() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.backend.SetFinalizedHighestHeight(s.finalizedBlock.Header.Height)
	referenceBlock := unittest.BlockHeaderFixture()
	transaction := unittest.TransactionFixture()
	transaction.SetReferenceBlockID(referenceBlock.ID())

	subCtx, subCancel := context.WithCancel(ctx)
	sub := s.backend.SendAndSubscribeTransactionStatuses(subCtx, &transaction.TransactionBody)

	s.sealedBlock = s.finalizedBlock
	s.finalizedBlock = unittest.BlockWithParentFixture(s.sealedBlock.Header)

	col := flow.CollectionFromTransactions([]*flow.Transaction{&transaction})
	guarantee := col.Guarantee()
	s.finalizedBlock.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))

	s.transactionResults.On("ByBlockIDTransactionID", s.finalizedBlock.ID(), transaction.ID()).
		Return(&flow.LightTransactionResult{
			TransactionID:   transaction.ID(),
			Failed:          false,
			ComputationUsed: 0,
		}, nil)

	s.backend.SetFinalizedHighestHeight(s.finalizedBlock.Header.Height)

	s.broadcaster.Publish()

	// consume block from subscription
	unittest.RequireReturnsBefore(s.T(), func() {
		v, ok := <-sub.Channel()
		require.True(
			s.T(),
			ok,
			"channel closed while waiting for transaction info txID %v, blockID: %v : err: %v",
			transaction.ID(),
			s.finalizedBlock.ID(),
			sub.Err(),
		)
		txInfo, ok := v.(*convert.TransactionSubscribeInfo)
		require.True(s.T(), ok, "unexpected response type: %T", v)
		assert.Equal(s.T(), transaction.ID(), txInfo.ID)
		assert.Equal(s.T(), flow.TransactionStatusExecuted, txInfo.Status)
		assert.Equal(s.T(), uint64(0), txInfo.MessageIndex)
	}, time.Second, fmt.Sprintf(
		"timed out waiting for result txID: %v, blockID %v",
		transaction.ID(),
		s.finalizedBlock.ID(),
	))

	subCancel()

	// ensure subscription shuts down gracefully
	unittest.RequireReturnsBefore(s.T(), func() {
		v, ok := <-sub.Channel()
		assert.Nil(s.T(), v)
		assert.False(s.T(), ok)
		assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
	}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
}
