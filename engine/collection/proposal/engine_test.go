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

	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// testcontext contains the context for a test case.
type testcontext struct {
	state       *protocol.State
	snapshot    *protocol.Snapshot
	me          *module.Local
	net         *stub.Network
	provider    *network.Engine
	pool        *mempool.Transactions
	collections *storage.Collections
	guarantees  *storage.Guarantees
	headers     *storage.Headers
	builder     *module.Builder
	finalizer   *module.Finalizer
}

// WithEngine initializes the dependencies for a test case, then runs the test
// case with the dependencies and initialized engine.
func WithEngine(t *testing.T, run func(testcontext, *Engine)) {
	var ctx testcontext

	log := zerolog.New(os.Stderr)
	tracer, err := trace.NewTracer(log)
	require.NoError(t, err)

	ctx.state = new(protocol.State)
	ctx.snapshot = new(protocol.Snapshot)
	ctx.state.On("Final").Return(ctx.snapshot)
	ctx.snapshot.On("Head").Return(&flow.Header{}, nil)
	ctx.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil)
	ctx.me = new(module.Local)
	ctx.me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	ctx.net = stub.NewNetwork(ctx.state, ctx.me, hub)

	ctx.provider = new(network.Engine)
	ctx.pool = new(mempool.Transactions)
	ctx.collections = new(storage.Collections)
	ctx.guarantees = new(storage.Guarantees)
	ctx.headers = new(storage.Headers)
	ctx.builder = new(module.Builder)
	ctx.finalizer = new(module.Finalizer)

	eng, err := New(log, ctx.net, ctx.me, ctx.state, tracer, ctx.provider, ctx.pool, ctx.collections, ctx.guarantees, ctx.headers)
	require.NoError(t, err)

	cold, err := coldstuff.New(log, ctx.state, ctx.me, eng, ctx.builder, ctx.finalizer, time.Second, time.Second)
	require.NoError(t, err)

	run(ctx, eng.WithConsensus(cold))
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
			ctx.pool.On("Rem", tx.ID()).Return(true).Once()
			ctx.collections.On("Store", mock.Anything).Return(nil).Once()
			ctx.guarantees.On("Store", mock.Anything).Return(nil).Once()
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

			// should return an error
			err := e.createProposal()
			if assert.Error(t, err) {
				assert.True(t, errors.Is(err, ErrEmptyTxpool))
			}

			// should not submit proposal to provider
			ctx.provider.AssertNotCalled(t, "ProcessLocal", mock.Anything)
			// should not remove anything from txpool
			ctx.pool.AssertNotCalled(t, "Rem", mock.Anything)
		})
	})
}
