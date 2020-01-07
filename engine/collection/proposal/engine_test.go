package proposal

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	mockprotocol "github.com/dapperlabs/flow-go/protocol/mock"
	mockstorage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// testdeps contains all dependencies of a test case, initialized by WithEngine
type testdeps struct {
	state       *mockprotocol.State
	me          *mockmodule.Local
	net         *stub.Network
	provider    *mocknetwork.Engine
	pool        *mockmodule.TransactionPool
	collections *mockstorage.Collections
	guarantees  *mockstorage.Guarantees
}

// WithEngine initializes the dependencies for a test case, then runs the test
// case with the dependencies and initialized engine.
func WithEngine(t *testing.T, run func(testdeps, *Engine)) {
	var deps testdeps

	log := zerolog.New(os.Stderr)

	deps.state = new(mockprotocol.State)
	deps.me = new(mockmodule.Local)
	deps.me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	deps.net = stub.NewNetwork(deps.state, deps.me, hub)

	conf := Config{
		ProposalPerid: time.Millisecond,
	}

	deps.provider = new(mocknetwork.Engine)
	deps.pool = new(mockmodule.TransactionPool)
	deps.collections = new(mockstorage.Collections)
	deps.guarantees = new(mockstorage.Guarantees)

	e, err := New(log, conf, deps.net, deps.me, deps.state, deps.provider, deps.pool, deps.collections, deps.guarantees)
	require.NoError(t, err)

	run(deps, e)
}

func TestStartStop(t *testing.T) {
	WithEngine(t, func(_ testdeps, e *Engine) {
		ready := e.Ready()

		select {
		case <-ready:
		case <-time.After(time.Second):
			t.Fail()
		}

		done := e.Done()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fail()
		}
	})
}

func TestProposalEngine(t *testing.T) {

	t.Run("should propose collection when txpool is non-empty", func(t *testing.T) {
		WithEngine(t, func(test testdeps, e *Engine) {

			tx := unittest.TransactionFixture()

			test.pool.On("Size").Return(uint(1)).Once()
			test.pool.On("All").Return([]*flow.Transaction{&tx}).Once()
			test.collections.On("Save", mock.Anything).Return(nil).Once()
			test.guarantees.On("Save", mock.Anything).Return(nil).Once()
			test.provider.On("ProcessLocal", mock.Anything).Return(nil).Once()

			err := e.createProposal()
			assert.NoError(t, err)

			// should submit guarantee for proposed collection
			test.provider.AssertCalled(t, "ProcessLocal", mock.AnythingOfType("*messages.SubmitCollectionGuarantee"))
		})

	})

	t.Run("should not propose collection when txpool is empty", func(t *testing.T) {
		WithEngine(t, func(test testdeps, e *Engine) {

			test.pool.On("Size").Return(uint(0)).Once()

			// should return an error and not submit a guarantee to provider engine
			err := e.createProposal()
			if assert.Error(t, err) {
				assert.True(t, errors.Is(err, ErrEmptyTxpool))
			}
			test.provider.AssertNotCalled(t, "ProcessLocal", mock.Anything)
		})
	})
}
