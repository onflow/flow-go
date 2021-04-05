package requester

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"

	protocol "github.com/onflow/flow-go/state/protocol/mock"
)

// RequesterEngineTestSuite encapsulates data structures for running unittests on requester engine.
type RequesterEngineTestSuite struct {
	// modules
	log             zerolog.Logger
	handler         fetcher.ChunkDataPackHandler // contains callbacks for handling received chunk data packs.
	retryInterval   time.Duration                // determines time in milliseconds for retrying chunk data requests.
	pendingRequests mempool.ChunkRequests        // used to store all the pending chunks that assigned to this node
	state           *protocol.State              // used to check the last sealed height
	con             *mocknetwork.Conduit         // used to send chunk data request, and receive the response

	// identities
	verIdentity *flow.Identity // verification node
}

// setupTest initiates a test suite prior to each test.
func setupTest() *RequesterEngineTestSuite {
	r := &RequesterEngineTestSuite{
		log:             unittest.Logger(),
		handler:         &mockfetcher.ChunkDataPackHandler{},
		retryInterval:   100 * time.Millisecond,
		pendingRequests: mempool.ChunkRequests{},
		state:           &protocol.State{},
		verIdentity:     unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
	}

	return r
}

// newRequesterEngine returns a requester engine for testing.
func newRequesterEngine(t *testing.T, s *RequesterEngineTestSuite) *Engine {
	net := &mock.Network{}
	// mocking the network registration of the engine
	net.On("Register", engine.RequestChunks, testifymock.Anything).
		Return(s.con, nil).
		Once()

	e, err := New(s.log, s.state, net, s.retryInterval, s.handler)
	require.NoError(t, err)

	testifymock.AssertExpectationsForObjects(t, net)

	return e
}
