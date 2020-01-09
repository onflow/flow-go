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

// testcontext contains the context for a test case.
type testcontext struct {
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
func WithEngine(t *testing.T, run func(testcontext, *Engine)) {
	var ctx testcontext

	log := zerolog.New(os.Stderr)

	ctx.state = new(mockprotocol.State)
	ctx.me = new(mockmodule.Local)
	ctx.me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	ctx.net = stub.NewNetwork(ctx.state, ctx.me, hub)

	conf := Config{
		ProposalPeriod: time.Millisecond,
	}

	ctx.provider = new(mocknetwork.Engine)
	ctx.pool = new(mockmodule.TransactionPool)
	ctx.collections = new(mockstorage.Collections)
	ctx.guarantees = new(mockstorage.Guarantees)

	e, err := New(log, conf, ctx.net, ctx.me, ctx.state, ctx.provider, ctx.pool, ctx.collections, ctx.guarantees)
	require.NoError(t, err)

	run(ctx, e)
}

func TestStartStop(t *testing.T) {
	WithEngine(t, func(_ testcontext, e *Engine) {
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
		WithEngine(t, func(ctx testcontext, e *Engine) {

			tx := unittest.TransactionFixture()

			ctx.pool.On("Size").Return(uint(1)).Once()
			ctx.pool.On("All").Return([]*flow.Transaction{&tx}).Once()
			ctx.pool.On("Rem", tx.Fingerprint()).Return(true).Once()
			ctx.collections.On("Persist", mock.Anything).Return(nil).Once()
			ctx.guarantees.On("Persist", mock.Anything).Return(nil).Once()
			ctx.provider.On("ProcessLocal", mock.AnythingOfType("*messages.SubmitCollectionGuarantee")).Return(nil).Once()

			err := e.createProposal()
			assert.NoError(t, err)

			// should submit guarantee for proposed collection to provider
			ctx.provider.AssertExpectations(t)
			// should remove tx
			ctx.pool.AssertExpectations(t)
			// should save collection and guarantee
			ctx.collections.AssertExpectations(t)
			ctx.guarantees.AssertExpectations(t)
		})
	})

	t.Run("should not propose collection when txpool is empty", func(t *testing.T) {
		WithEngine(t, func(ctx testcontext, e *Engine) {

			ctx.pool.On("Size").Return(uint(0)).Once()

			// should return an error and not submit a guarantee to provider engine
			err := e.createProposal()
			if assert.Error(t, err) {
				assert.True(t, errors.Is(err, ErrEmptyTxpool))
			}
			ctx.provider.AssertNotCalled(t, "ProcessLocal", mock.Anything)
		})
	})
}
