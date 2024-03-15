package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	access "github.com/onflow/flow-go/engine/access/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

var targetEvent string

type testCase struct {
	encoding  entities.EventEncodingVersion
	queryMode IndexQueryMode
}

type BackendEventsSuite struct {
	suite.Suite

	log        zerolog.Logger
	state      *protocol.State
	snapshot   *protocol.Snapshot
	params     *protocol.Params
	rootHeader *flow.Header

	eventsIndex       *EventsIndex
	events            *storagemock.Events
	headers           *storagemock.Headers
	receipts          *storagemock.ExecutionReceipts
	connectionFactory *connectionmock.ConnectionFactory
	chainID           flow.ChainID

	executionNodes flow.IdentityList
	execClient     *access.ExecutionAPIClient

	sealedHead  *flow.Header
	blocks      []*flow.Block
	blockIDs    []flow.Identifier
	blockEvents []flow.Event

	testCases []testCase
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(BackendEventsSuite))
}

func (s *BackendEventsSuite) SetupTest() {
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
	s.eventsIndex = NewEventsIndex(s.events)

	blockCount := 5
	s.blocks = make([]*flow.Block, blockCount)
	s.blockIDs = make([]flow.Identifier, blockCount)

	for i := 0; i < blockCount; i++ {
		var header *flow.Header
		if i == 0 {
			header = unittest.BlockHeaderFixture()
		} else {
			header = unittest.BlockHeaderWithParentFixture(s.blocks[i-1].Header)
		}

		payload := unittest.PayloadFixture()
		header.PayloadHash = payload.Hash()
		block := &flow.Block{
			Header:  header,
			Payload: &payload,
		}
		// the last block is sealed
		if i == blockCount-1 {
			s.sealedHead = header
		}

		s.blocks[i] = block
		s.blockIDs[i] = block.ID()

		s.T().Logf("block %d: %s", header.Height, block.ID())
	}

	s.blockEvents = generator.GetEventsWithEncoding(10, entities.EventEncodingVersion_CCF_V0)
	targetEvent = string(s.blockEvents[0].Type)

	s.events.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) ([]flow.Event, error) {
		for _, headerID := range s.blockIDs {
			if blockID == headerID {
				return s.blockEvents, nil
			}
		}
		return nil, storage.ErrNotFound
	}).Maybe()

	s.headers.On("BlockIDByHeight", mock.Anything).Return(func(height uint64) (flow.Identifier, error) {
		for _, block := range s.blocks {
			if height == block.Header.Height {
				return block.ID(), nil
			}
		}
		return flow.ZeroID, storage.ErrNotFound
	}).Maybe()

	s.headers.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) (*flow.Header, error) {
		for _, block := range s.blocks {
			if blockID == block.ID() {
				return block.Header, nil
			}
		}
		return nil, storage.ErrNotFound
	}).Maybe()

	s.testCases = make([]testCase, 0)

	for _, encoding := range []entities.EventEncodingVersion{
		entities.EventEncodingVersion_CCF_V0,
		entities.EventEncodingVersion_JSON_CDC_V0,
	} {
		for _, queryMode := range []IndexQueryMode{
			IndexQueryModeExecutionNodesOnly,
			IndexQueryModeLocalOnly,
			IndexQueryModeFailover,
		} {
			s.testCases = append(s.testCases, testCase{
				encoding:  encoding,
				queryMode: queryMode,
			})
		}
	}
}

func (s *BackendEventsSuite) defaultBackend() *backendEvents {
	return &backendEvents{
		log:               s.log,
		chain:             s.chainID.Chain(),
		state:             s.state,
		headers:           s.headers,
		executionReceipts: s.receipts,
		connFactory:       s.connectionFactory,
		nodeCommunicator:  NewNodeCommunicator(false),
		maxHeightRange:    DefaultMaxHeightRange,
		queryMode:         IndexQueryModeExecutionNodesOnly,
		eventsIndex:       s.eventsIndex,
	}
}

// setupExecutionNodes sets up the mocks required to test against an EN backend
func (s *BackendEventsSuite) setupExecutionNodes(block *flow.Block) {
	s.params.On("FinalizedRoot").Return(s.rootHeader, nil)
	s.state.On("Params").Return(s.params)
	s.state.On("Final").Return(s.snapshot)
	s.snapshot.On("Identities", mock.Anything).Return(s.executionNodes, nil)

	// this line causes a S1021 lint error because receipts is explicitly declared. this is required
	// to ensure the mock library handles the response type correctly
	var receipts flow.ExecutionReceiptList //nolint:gosimple
	receipts = unittest.ReceiptsForBlockFixture(block, s.executionNodes.NodeIDs())
	s.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)

	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mockCloser{}, nil)
}

// setupENSuccessResponse configures the execution node client to return a successful response
func (s *BackendEventsSuite) setupENSuccessResponse(eventType string, blocks []*flow.Block) {
	s.setupExecutionNodes(blocks[len(blocks)-1])

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
			BlockHeight: block.Header.Height,
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

	s.execClient.On("GetEventsForBlockIDs", mock.Anything, expectedExecRequest).
		Return(expectedResponse, nil)
}

// setupENFailingResponse configures the execution node client to return an error
func (s *BackendEventsSuite) setupENFailingResponse(eventType string, headers []*flow.Header, err error) {
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

// TestGetEvents_HappyPaths tests the happy paths for GetEventsForBlockIDs and GetEventsForHeightRange
// across all queryModes and encodings
func (s *BackendEventsSuite) TestGetEvents_HappyPaths() {
	ctx := context.Background()

	startHeight := s.blocks[0].Header.Height
	endHeight := s.sealedHead.Height

	reporter := syncmock.NewIndexReporter(s.T())
	reporter.On("LowestIndexedHeight").Return(startHeight, nil)
	reporter.On("HighestIndexedHeight").Return(endHeight+10, nil)
	err := s.eventsIndex.Initialize(reporter)
	s.Require().NoError(err)

	s.state.On("Sealed").Return(s.snapshot)
	s.snapshot.On("Head").Return(s.sealedHead, nil)

	s.Run("GetEventsForHeightRange - end height updated", func() {
		backend := s.defaultBackend()
		backend.queryMode = IndexQueryModeFailover
		endHeight := startHeight + 20 // should still return 5 responses
		encoding := entities.EventEncodingVersion_CCF_V0

		response, err := backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, encoding)
		s.Require().NoError(err)

		s.assertResponse(response, encoding)
	})

	for _, tt := range s.testCases {
		s.Run(fmt.Sprintf("all from storage - %s - %s", tt.encoding.String(), tt.queryMode), func() {
			switch tt.queryMode {
			case IndexQueryModeExecutionNodesOnly:
				// not applicable
				return
			case IndexQueryModeLocalOnly, IndexQueryModeFailover:
				// only calls to local storage
			}

			backend := s.defaultBackend()
			backend.queryMode = tt.queryMode

			response, err := backend.GetEventsForBlockIDs(ctx, targetEvent, s.blockIDs, tt.encoding)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)

			response, err = backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, tt.encoding)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)
		})

		s.Run(fmt.Sprintf("all from en - %s - %s", tt.encoding.String(), tt.queryMode), func() {
			events := storagemock.NewEvents(s.T())
			eventsIndex := NewEventsIndex(events)

			switch tt.queryMode {
			case IndexQueryModeLocalOnly:
				// not applicable
				return
			case IndexQueryModeExecutionNodesOnly:
				// only calls to EN, no calls to storage
			case IndexQueryModeFailover:
				// all calls to storage fail
				// simulated by not initializing the eventIndex so all calls return ErrIndexNotInitialized
			}

			backend := s.defaultBackend()
			backend.queryMode = tt.queryMode
			backend.eventsIndex = eventsIndex

			s.setupENSuccessResponse(targetEvent, s.blocks)

			response, err := backend.GetEventsForBlockIDs(ctx, targetEvent, s.blockIDs, tt.encoding)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)

			response, err = backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, tt.encoding)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)
		})

		s.Run(fmt.Sprintf("mixed storage & en - %s - %s", tt.encoding.String(), tt.queryMode), func() {
			events := storagemock.NewEvents(s.T())
			eventsIndex := NewEventsIndex(events)

			switch tt.queryMode {
			case IndexQueryModeLocalOnly, IndexQueryModeExecutionNodesOnly:
				// not applicable
				return
			case IndexQueryModeFailover:
				// only failing blocks queried from EN
				s.setupENSuccessResponse(targetEvent, []*flow.Block{s.blocks[0], s.blocks[4]})
			}

			// the first and last blocks are not available from storage, and should be fetched from the EN
			reporter := syncmock.NewIndexReporter(s.T())
			reporter.On("LowestIndexedHeight").Return(s.blocks[1].Header.Height, nil)
			reporter.On("HighestIndexedHeight").Return(s.blocks[3].Header.Height, nil)

			events.On("ByBlockID", s.blockIDs[1]).Return(s.blockEvents, nil)
			events.On("ByBlockID", s.blockIDs[2]).Return(s.blockEvents, nil)
			events.On("ByBlockID", s.blockIDs[3]).Return(s.blockEvents, nil)

			err := eventsIndex.Initialize(reporter)
			s.Require().NoError(err)

			backend := s.defaultBackend()
			backend.queryMode = tt.queryMode
			backend.eventsIndex = eventsIndex

			response, err := backend.GetEventsForBlockIDs(ctx, targetEvent, s.blockIDs, tt.encoding)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)

			response, err = backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, tt.encoding)
			s.Require().NoError(err)
			s.assertResponse(response, tt.encoding)
		})
	}
}

func (s *BackendEventsSuite) TestGetEventsForHeightRange_HandlesErrors() {
	ctx := context.Background()

	startHeight := s.blocks[0].Header.Height
	endHeight := s.sealedHead.Height
	encoding := entities.EventEncodingVersion_CCF_V0

	s.Run("returns error for endHeight < startHeight", func() {
		backend := s.defaultBackend()
		endHeight := startHeight - 1

		response, err := backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, encoding)
		s.Assert().Equal(codes.InvalidArgument, status.Code(err))
		s.Assert().Nil(response)
	})

	s.Run("returns error for range larger than max", func() {
		backend := s.defaultBackend()
		endHeight := startHeight + DefaultMaxHeightRange

		response, err := backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, encoding)
		s.Assert().Equal(codes.InvalidArgument, status.Code(err))
		s.Assert().Nil(response)
	})

	s.Run("throws irrecoverable if sealed header not available", func() {
		s.state.On("Sealed").Return(s.snapshot)
		s.snapshot.On("Head").Return(nil, storage.ErrNotFound).Once()

		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", storage.ErrNotFound)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, signCtxErr))

		backend := s.defaultBackend()

		response, err := backend.GetEventsForHeightRange(signalerCtx, targetEvent, startHeight, endHeight, encoding)
		// these will never be returned in production
		s.Assert().Equal(codes.Unknown, status.Code(err))
		s.Assert().Nil(response)
	})

	s.state.On("Sealed").Return(s.snapshot)
	s.snapshot.On("Head").Return(s.sealedHead, nil)

	s.Run("returns error for startHeight > sealed height", func() {
		backend := s.defaultBackend()
		startHeight := s.sealedHead.Height + 1
		endHeight := startHeight + 1

		response, err := backend.GetEventsForHeightRange(ctx, targetEvent, startHeight, endHeight, encoding)
		s.Assert().Equal(codes.OutOfRange, status.Code(err))
		s.Assert().Nil(response)
	})
}

func (s *BackendEventsSuite) TestGetEventsForBlockIDs_HandlesErrors() {
	ctx := context.Background()

	encoding := entities.EventEncodingVersion_CCF_V0

	s.Run("returns error when too many blockIDs requested", func() {
		backend := s.defaultBackend()
		backend.maxHeightRange = 3

		response, err := backend.GetEventsForBlockIDs(ctx, targetEvent, s.blockIDs, encoding)
		s.Assert().Equal(codes.InvalidArgument, status.Code(err))
		s.Assert().Nil(response)
	})

	s.Run("returns error for missing header", func() {
		headers := storagemock.NewHeaders(s.T())
		backend := s.defaultBackend()
		backend.headers = headers

		for i, blockID := range s.blockIDs {
			// return error on the last header
			if i == len(s.blocks)-1 {
				headers.On("ByBlockID", blockID).Return(nil, storage.ErrNotFound)
				continue
			}

			headers.On("ByBlockID", blockID).Return(s.blocks[i].Header, nil)
		}

		response, err := backend.GetEventsForBlockIDs(ctx, targetEvent, s.blockIDs, encoding)
		s.Assert().Equal(codes.NotFound, status.Code(err))
		s.Assert().Nil(response)
	})
}

func (s *BackendEventsSuite) assertResponse(response []flow.BlockEvents, encoding entities.EventEncodingVersion) {
	s.Assert().Len(response, len(s.blocks))
	for i, block := range s.blocks {
		s.Assert().Equal(block.Header.Height, response[i].BlockHeight)
		s.Assert().Equal(block.Header.ID(), response[i].BlockID)
		s.Assert().Len(response[i].Events, 1)

		s.assertEncoding(&response[i].Events[0], encoding)
	}
}

func (s *BackendEventsSuite) assertEncoding(event *flow.Event, encoding entities.EventEncodingVersion) {
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
