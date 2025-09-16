package events

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	access "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"

	"github.com/onflow/flow-go/module/irrecoverable"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

var targetEvent string

type testCase struct {
	encoding  entities.EventEncodingVersion
	queryMode query_mode.IndexQueryMode
}

type EventsSuite struct {
	suite.Suite

	log        zerolog.Logger
	state      *protocol.State
	snapshot   *protocol.Snapshot
	params     *protocol.Params
	rootHeader *flow.Header

	events            *storagemock.Events
	headers           *storagemock.Headers
	receipts          *storagemock.ExecutionReceipts
	connectionFactory *connectionmock.ConnectionFactory
	chainID           flow.ChainID

	executionNodes flow.IdentityList
	execClient     *access.ExecutionAPIClient

	executionResult *flow.ExecutionResult
	sealedHead      *flow.Header
	blocks          []*flow.Block
	blockIDs        []flow.Identifier
	blockEvents     []flow.Event

	executionResultProvider *osyncmock.ExecutionResultInfoProvider
	executionStateCache     *osyncmock.ExecutionStateCache
	executionDataSnapshot   *osyncmock.Snapshot
	criteria                optimistic_sync.Criteria

	testCases []testCase
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(EventsSuite))
}

func (s *EventsSuite) SetupTest() {
	s.log = unittest.Logger()
	s.state = protocol.NewState(s.T())
	s.snapshot = protocol.NewSnapshot(s.T())
	s.rootHeader = unittest.BlockHeaderFixture()
	s.params = protocol.NewParams(s.T())
	s.events = storagemock.NewEvents(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.chainID = flow.Testnet

	s.execClient = access.NewExecutionAPIClient(s.T())
	s.executionNodes = unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

	s.executionResult = unittest.ExecutionResultFixture()
	blockCount := 5
	s.blocks = make([]*flow.Block, blockCount)
	s.blockIDs = make([]flow.Identifier, blockCount)

	for i := 0; i < blockCount; i++ {
		var header *flow.Header
		if i == 0 {
			header = unittest.BlockHeaderFixture()
		} else {
			header = unittest.BlockHeaderWithParentFixture(s.blocks[i-1].ToHeader())
		}

		payload := unittest.PayloadFixture()
		header.PayloadHash = payload.Hash()
		block, err := flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: header.HeaderBody,
				Payload:    payload,
			},
		)
		require.NoError(s.T(), err)

		// the last block is sealed
		if i == blockCount-1 {
			s.sealedHead = header
		}

		s.blocks[i] = block
		s.blockIDs[i] = block.ID()

		s.T().Logf("block %d: %s", header.Height, block.ID())
	}

	s.blockEvents = unittest.EventGenerator.GetEventsWithEncoding(10, entities.EventEncodingVersion_CCF_V0)
	targetEvent = string(s.blockEvents[0].Type)

	// events returned from the db are sorted by txID, txIndex, then eventIndex.
	// reproduce that here to ensure output order works as expected
	returnBlockEvents := make([]flow.Event, len(s.blockEvents))
	copy(returnBlockEvents, s.blockEvents)

	sort.Slice(returnBlockEvents, func(i, j int) bool {
		return bytes.Compare(returnBlockEvents[i].TransactionID[:], returnBlockEvents[j].TransactionID[:]) < 0
	})

	s.events.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) ([]flow.Event, error) {
		for _, headerID := range s.blockIDs {
			if blockID == headerID {
				return returnBlockEvents, nil
			}
		}
		return nil, storage.ErrNotFound
	}).Maybe()

	s.headers.On("BlockIDByHeight", mock.Anything).Return(func(height uint64) (flow.Identifier, error) {
		for _, block := range s.blocks {
			if height == block.Height {
				return block.ID(), nil
			}
		}
		return flow.ZeroID, storage.ErrNotFound
	}).Maybe()

	s.headers.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) (*flow.Header, error) {
		for _, block := range s.blocks {
			if blockID == block.ID() {
				return block.ToHeader(), nil
			}
		}
		return nil, storage.ErrNotFound
	}).Maybe()

	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.executionDataSnapshot.
		On("Events").
		Return(s.events, nil).
		Maybe()

	s.executionResultProvider = osyncmock.NewExecutionResultInfoProvider(s.T())
	s.executionResultProvider.
		On("ExecutionResultInfo", mock.Anything, mock.Anything).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResult: s.executionResult,
			ExecutionNodes:  s.executionNodes.ToSkeleton(),
		}, nil).
		Maybe() // it is called only for local query mode

	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(s.executionDataSnapshot, nil).
		Maybe() // it is called only for local query mode

	s.criteria = optimistic_sync.Criteria{}

	s.testCases = make([]testCase, 0)

	for _, encoding := range []entities.EventEncodingVersion{
		entities.EventEncodingVersion_CCF_V0,
		entities.EventEncodingVersion_JSON_CDC_V0,
	} {
		for _, queryMode := range []query_mode.IndexQueryMode{
			query_mode.IndexQueryModeExecutionNodesOnly,
			query_mode.IndexQueryModeLocalOnly,
			query_mode.IndexQueryModeFailover,
		} {
			s.testCases = append(s.testCases, testCase{
				encoding:  encoding,
				queryMode: queryMode,
			})
		}
	}
}

// TestGetEvents_HappyPaths tests the happy paths for GetEventsForBlockIDs and GetEventsForHeightRange
// across all queryModes and encodings
func (s *EventsSuite) TestGetEvents_HappyPaths() {
	ctx := context.Background()

	startHeight := s.blocks[0].Height
	endHeight := s.sealedHead.Height

	s.state.On("Sealed").Return(s.snapshot)
	s.snapshot.On("Head").Return(s.sealedHead, nil)

	s.Run("GetEventsForHeightRange - end height updated", func() {
		backend := s.defaultBackend(query_mode.IndexQueryModeFailover)
		endHeight := startHeight + 20 // should still return 5 responses
		encoding := entities.EventEncodingVersion_CCF_V0

		response, _, err := backend.GetEventsForHeightRange(
			ctx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		s.Require().NoError(err)

		s.assertResponse(response, encoding)
	})

	for _, tt := range s.testCases {
		s.Run(fmt.Sprintf("with local query mode. encdoing: %s, query mode: %s", tt.encoding.String(), tt.queryMode), func() {
			if tt.queryMode != query_mode.IndexQueryModeLocalOnly {
				return
			}

			backend := s.defaultBackend(tt.queryMode)
			response, _, err := backend.GetEventsForBlockIDs(
				ctx,
				targetEvent,
				s.blockIDs,
				tt.encoding,
				s.criteria,
			)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)

			response, _, err = backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, tt.encoding, s.criteria)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)
		})

		s.Run(fmt.Sprintf("with execution node query mode. encdoing: %s, query mode: %s", tt.encoding.String(), tt.queryMode), func() {
			if tt.queryMode != query_mode.IndexQueryModeExecutionNodesOnly {
				return
			}

			backend := s.defaultBackend(tt.queryMode)
			s.setupENSuccessResponse(targetEvent, s.blocks)

			response, _, err := backend.GetEventsForBlockIDs(
				ctx,
				targetEvent,
				s.blockIDs,
				tt.encoding,
				s.criteria,
			)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)

			response, _, err = backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, tt.encoding, s.criteria)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)
		})

		s.Run(fmt.Sprintf("with failover query mode. encdoing: %s, query mode: %s", tt.encoding.String(), tt.queryMode), func() {
			if tt.queryMode != query_mode.IndexQueryModeFailover {
				return
			}

			//TODO: this tests only local query mode tbh. we need to make it fail at some point and
			// make sure that execution nodes are called

			backend := s.defaultBackend(tt.queryMode)
			response, _, err := backend.GetEventsForBlockIDs(
				ctx,
				targetEvent,
				s.blockIDs,
				tt.encoding,
				s.criteria,
			)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)

			response, _, err = backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, tt.encoding, s.criteria)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)
		})
	}
}

func (s *EventsSuite) TestGetEventsForHeightRange_HandlesErrors() {
	ctx := context.Background()

	startHeight := s.blocks[0].Height
	endHeight := s.sealedHead.Height
	encoding := entities.EventEncodingVersion_CCF_V0

	s.Run("returns error for endHeight < startHeight", func() {
		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		endHeight := startHeight - 1

		response, _, err := backend.GetEventsForHeightRange(
			ctx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.InvalidArgument, status.Code(err))
		s.Assert().Nil(response)
	})

	s.Run("returns error for range larger than max", func() {
		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		endHeight := startHeight + DefaultMaxHeightRange

		response, _, err := backend.GetEventsForHeightRange(
			ctx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.InvalidArgument, status.Code(err))
		s.Assert().Nil(response)
	})

	s.Run("throws irrecoverable if sealed header not available", func() {
		s.state.On("Sealed").Return(s.snapshot)
		s.snapshot.On("Head").Return(nil, storage.ErrNotFound).Once()

		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", storage.ErrNotFound)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, signCtxErr))

		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		response, _, err := backend.GetEventsForHeightRange(
			signalerCtx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		// these will never be returned in production
		s.Assert().Equal(codes.Unknown, status.Code(err))
		s.Assert().Nil(response)
	})

	s.state.On("Sealed").Return(s.snapshot)
	s.snapshot.On("Head").Return(s.sealedHead, nil)

	s.Run("returns error for startHeight > sealed height", func() {
		startHeight := s.sealedHead.Height + 1
		endHeight := startHeight + 1

		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		response, _, err := backend.GetEventsForHeightRange(
			ctx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.OutOfRange, status.Code(err))
		s.Assert().Nil(response)
	})

	s.state.On("Params").Return(s.params)

	s.Run("returns error for startHeight < spork root height", func() {
		sporkRootHeight := s.blocks[0].Height - 10
		startHeight := sporkRootHeight - 1

		s.params.On("SporkRootBlockHeight").Return(sporkRootHeight).Once()
		s.params.On("SealedRoot").Return(s.rootHeader, nil).Once()

		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		response, _, err := backend.GetEventsForHeightRange(
			ctx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.NotFound, status.Code(err))
		s.Assert().ErrorContains(err, "Try to use a historic node")
		s.Assert().Nil(response)
	})

	s.Run("returns error for startHeight < node root height", func() {
		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)

		sporkRootHeight := s.blocks[0].Height - 10
		nodeRootHeader := unittest.BlockHeaderWithHeight(s.blocks[0].Height)
		startHeight := nodeRootHeader.Height - 5

		s.params.On("SporkRootBlockHeight").Return(sporkRootHeight).Once()
		s.params.On("SealedRoot").Return(nodeRootHeader, nil).Once()

		response, _, err := backend.GetEventsForHeightRange(
			ctx,
			targetEvent,
			startHeight,
			endHeight,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.NotFound, status.Code(err))
		s.Assert().ErrorContains(err, "Try to use a different Access node")
		s.Assert().Nil(response)
	})
}

func (s *EventsSuite) TestGetEventsForBlockIDs_HandlesErrors() {
	ctx := context.Background()
	encoding := entities.EventEncodingVersion_CCF_V0

	s.Run("returns error when too many blockIDs requested", func() {
		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		backend.maxHeightRange = 3

		response, _, err := backend.GetEventsForBlockIDs(
			ctx,
			targetEvent,
			s.blockIDs,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.InvalidArgument, status.Code(err))
		s.Assert().Nil(response)
	})

	s.Run("returns error for missing header", func() {
		headers := storagemock.NewHeaders(s.T())
		backend := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)
		backend.headers = headers

		for i, blockID := range s.blockIDs {
			// return error on the last header
			if i == len(s.blocks)-1 {
				headers.On("ByBlockID", blockID).Return(nil, storage.ErrNotFound)
				continue
			}

			headers.On("ByBlockID", blockID).Return(s.blocks[i].ToHeader(), nil)
		}

		response, _, err := backend.GetEventsForBlockIDs(
			ctx,
			targetEvent,
			s.blockIDs,
			encoding,
			s.criteria,
		)
		s.Assert().Equal(codes.NotFound, status.Code(err))
		s.Assert().Nil(response)
	})
}

func (s *EventsSuite) assertResponse(response []flow.BlockEvents, encoding entities.EventEncodingVersion) {
	s.Assert().Len(response, len(s.blocks))
	for i, block := range s.blocks {
		s.Assert().Equal(block.Height, response[i].BlockHeight)
		s.Assert().Equal(block.ID(), response[i].BlockID)
		s.Assert().Len(response[i].Events, 1)

		s.assertEncoding(&response[i].Events[0], encoding)
	}
}

func (s *EventsSuite) assertEncoding(event *flow.Event, encoding entities.EventEncodingVersion) {
	var err error
	switch encoding {
	case entities.EventEncodingVersion_CCF_V0:
		_, err = ccf.Decode(nil, event.Payload)
	case entities.EventEncodingVersion_JSON_CDC_V0:
		_, err = jsoncdc.Decode(nil, event.Payload)
	default:
		s.T().Errorf("unknown encoding: %s", encoding.String())
	}
	s.Require().NoError(err)
}

func (s *EventsSuite) defaultBackend(mode query_mode.IndexQueryMode) *Events {
	e, err := NewEventsBackend(
		s.log,
		s.state,
		s.chainID.Chain(),
		DefaultMaxHeightRange,
		s.headers,
		s.connectionFactory,
		node_communicator.NewNodeCommunicator(false),
		mode,
		commonrpc.NewExecutionNodeIdentitiesProvider(
			s.log,
			s.state,
			s.receipts,
			flow.IdentifierList{},
			flow.IdentifierList{},
		),
		s.executionResultProvider,
		s.executionStateCache,
		optimistic_sync.DefaultCriteria,
	)
	require.NoError(s.T(), err)

	return e
}

// setupENSuccessResponse configures the execution node client to return a successful response
func (s *EventsSuite) setupENSuccessResponse(eventType string, blocks []*flow.Block) {
	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil)

	ids := make([][]byte, len(blocks))
	results := make([]*execproto.GetEventsForBlockIDsResponse_Result, len(blocks))

	events := make([]*entities.Event, 0)
	for _, event := range s.blockEvents {
		if string(event.Type) == eventType {
			events = append(events, convert.EventToMessage(event))
		}
	}

	for i, block := range blocks {
		id := block.ID()
		ids[i] = id[:]
		results[i] = &execproto.GetEventsForBlockIDsResponse_Result{
			BlockId:     id[:],
			BlockHeight: block.Height,
			Events:      events,
		}
	}
	expectedExecRequest := &execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: ids,
	}
	expectedResponse := &execproto.GetEventsForBlockIDsResponse{
		Results:              results,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}

	s.execClient.
		On("GetEventsForBlockIDs", mock.Anything, expectedExecRequest).
		Return(expectedResponse, nil)
}

// setupENFailingResponse configures the execution node client to return an error
func (s *EventsSuite) setupENFailingResponse(eventType string, headers []*flow.Header, err error) {
	ids := make([][]byte, len(headers))
	for i, header := range headers {
		id := header.ID()
		ids[i] = id[:]
	}
	failingRequest := &execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: ids,
	}

	s.execClient.On("GetEventsForBlockIDs", mock.Anything, failingRequest).
		Return(nil, err)
}
