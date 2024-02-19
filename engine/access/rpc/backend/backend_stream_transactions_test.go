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
	subscriptionmock "github.com/onflow/flow-go/engine/access/subscription/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
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
	chainStateTracker *subscriptionmock.ChainStateTracker

	chainID flow.ChainID

	broadcaster    *engine.Broadcaster
	rootBlock      flow.Block
	sealedBlock    *flow.Block
	finalizedBlock *flow.Block

	blocksArray []*flow.Block
	blockMap    map[uint64]*flow.Block
	txResultMap map[flow.Identifier]*flow.LightTransactionResult

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
	s.chainStateTracker = subscriptionmock.NewChainStateTracker(s.T())

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	s.sealedBlock = &s.rootBlock
	s.finalizedBlock = unittest.BlockWithParentFixture(s.sealedBlock.Header)
	s.blocksArray = []*flow.Block{
		s.sealedBlock,
		s.finalizedBlock,
	}

	s.blockMap = map[uint64]*flow.Block{
		s.sealedBlock.Header.Height:    s.sealedBlock,
		s.finalizedBlock.Header.Height: s.finalizedBlock,
	}

	s.txResultMap = map[flow.Identifier]*flow.LightTransactionResult{}

	s.transactionResults.On("ByBlockIDTransactionID",
		mock.AnythingOfType("flow.Identifier"),
		mock.AnythingOfType("flow.Identifier")).
		Return(func(blockID flow.Identifier, transactionID flow.Identifier) *flow.LightTransactionResult {
			if txResult, ok := s.txResultMap[blockID]; ok {
				return txResult
			}
			return nil
		},
			func(blockID flow.Identifier, transactionID flow.Identifier) error {
				if _, ok := s.txResultMap[blockID]; ok {
					return nil
				}
				return storage.ErrNotFound
			}).Maybe()

	s.seals.On("HighestInFork", mock.AnythingOfType("flow.Identifier")).Return(
		func(_ flow.Identifier) (*flow.Seal, error) {
			return unittest.Seal.Fixture(unittest.Seal.WithBlock(s.sealedBlock.Header)), nil
		}, nil).Maybe()

	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) flow.Identifier {
			if block, ok := s.blockMap[height]; ok {
				return block.Header.ID()
			}
			return flow.ZeroID
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Header {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return nil
				}
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Block {
			if block, ok := s.blockMap[height]; ok {
				return block
			}
			return &flow.Block{}
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.state.On("Sealed").Return(s.sealedSnapshot, nil).Maybe()
	s.state.On("Final").Return(s.finalSnapshot, nil).Maybe()
	s.state.On("AtBlockID", mock.Anything).Return(s.finalSnapshot, nil)

	s.sealedSnapshot.On("Head").Return(func() *flow.Header {
		return s.sealedBlock.Header
	}, nil).Maybe()
	s.finalSnapshot.On("Head").Return(func() *flow.Header {
		return s.finalizedBlock.Header
	}, nil).Maybe()

	s.chainStateTracker.On("GetStartHeight", mock.Anything, mock.Anything, mock.Anything).Return(func(id flow.Identifier, _ uint64, _ flow.BlockStatus) (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil).Once()
	s.chainStateTracker.On("GetHighestHeight", flow.BlockStatusFinalized).Return(func(_ flow.BlockStatus) (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil)

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
		ChainStateTracker:        s.chainStateTracker,
		SubscriptionParams: SubscriptionParams{
			SendTimeout:    subscription.DefaultSendTimeout,
			SendBufferSize: subscription.DefaultSendBufferSize,
			ResponseLimit:  subscription.DefaultResponseLimit,
			Broadcaster:    s.broadcaster,
		},
	}
}

func (s *TransactionStatusSuite) addNewFinalizedBlock(parent *flow.Header, options ...func(*flow.Block)) {
	s.finalizedBlock = unittest.BlockWithParentFixture(parent)
	for _, option := range options {
		option(s.finalizedBlock)
	}

	s.state.On("AtBlockID", mock.Anything).Unset()
	s.state.On("AtBlockID", mock.Anything).Return(s.finalSnapshot, nil)

	s.blocksArray = append(s.blocksArray, s.finalizedBlock)
	s.blockMap[s.finalizedBlock.Header.Height] = s.finalizedBlock

	s.broadcaster.Publish()
}

// TestSubscribeTransactionStatus tests the functionality of the SendAndSubscribeTransactionStatuses method in the Backend.
// It covers the emulation of transaction stages from pending to sealed, and receiving status updates.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatus() {
	// Create subscription context with closer
	ctx, cancel := context.WithCancel(context.Background())

	// Generate sent transaction with ref block of the current finalized block
	transaction := unittest.TransactionFixture()
	transaction.SetReferenceBlockID(s.finalizedBlock.ID())
	txId := transaction.ID()
	expectedMsgIndexCounter := counters.NewMonotonousCounter(0)

	// Create a special common function to read subscription messages from the channel and check converting it to transaction info
	// and check results for correctness
	checkNewSubscriptionMessage := func(sub subscription.Subscription, expectedTxStatus flow.TransactionStatus) {
		s.log.Debug().Msg(fmt.Sprintf("!!! checkNewSubscriptionMessage: %s", expectedTxStatus.String()))
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

	// 1. Subscribe to transaction status and receive the first message with pending status
	sub := s.backend.SendAndSubscribeTransactionStatuses(ctx, &transaction.TransactionBody)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusPending)

	// 2. Make transaction reference block sealed, and add a new finalized block that includes the transaction
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, func(block *flow.Block) {
		col := flow.CollectionFromTransactions([]*flow.Transaction{&transaction})
		guarantee := col.Guarantee()
		block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))

		s.txResultMap[block.ID()] = &flow.LightTransactionResult{
			TransactionID:   transaction.ID(),
			Failed:          false,
			ComputationUsed: 0,
		}
	})
	checkNewSubscriptionMessage(sub, flow.TransactionStatusFinalized)

	// 3. Add one more finalized block on top of the transaction block
	s.addNewFinalizedBlock(s.finalizedBlock.Header)
	checkNewSubscriptionMessage(sub, flow.TransactionStatusExecuted)

	// 4. Make the transaction block sealed, and add a new finalized block
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header)
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
