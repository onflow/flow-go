package fetcher_test

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// FetcherEngineTestSuite encapsulates data structures for running unittests on fetcher engine.
type FetcherEngineTestSuite struct {
	// modules
	log                   zerolog.Logger
	metrics               module.VerificationMetrics
	tracer                module.Tracer
	verifier              network.Engine                   // the verifier engine
	state                 protocol.State                   // used to verify the request origin
	pendingChunks         mempool.ChunkStatuses            // used to store all the pending chunks that assigned to this node
	headers               storage.Headers                  // used to fetch the block header when chunk data is ready to be verified
	chunkConsumerNotifier module.ProcessingNotifier        // to report a chunk has been processed
	results               storage.ExecutionResults         // to retrieve execution result of an assigned chunk
	receipts              storage.ExecutionReceipts        // used to find executor of the chunk
	requester             mockfetcher.ChunkDataPackHandler // used to request chunk data packs from network

	// identities
	verIdentity *flow.Identity // verification node
}

// setupTest initiates a test suite prior to each test.
func setupTest() *FetcherEngineTestSuite {
	r := &FetcherEngineTestSuite{
		log:             unittest.Logger(),
		handler:         &mockfetcher.ChunkDataPackHandler{},
		retryInterval:   100 * time.Millisecond,
		pendingRequests: &mempool.ChunkRequests{},
		state:           &protocol.State{},
		verIdentity:     unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
		con:             &mocknetwork.Conduit{},
	}

	return r
}

// newRequesterEngine returns a requester engine for testing.
func newRequesterEngine(t *testing.T, s *RequesterEngineTestSuite) *requester.Engine {
	net := &mock.Network{}
	// mocking the network registration of the engine
	net.On("Register", engine.RequestChunks, testifymock.Anything).
		Return(s.con, nil).
		Once()

	e, err := requester.New(s.log, s.state, net, s.retryInterval, s.pendingRequests, s.handler)
	require.NoError(t, err)
	testifymock.AssertExpectationsForObjects(t, net)

	return e
}
