package proposal

import (
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

func TestStartStop(t *testing.T) {
	log := zerolog.New(os.Stderr)

	state := new(mockprotocol.State)
	me := new(mockmodule.Local)
	me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	stub := stub.NewNetwork(state, me, hub)

	conf := Config{
		ProposalPerid: time.Millisecond,
	}

	provider := new(mocknetwork.Engine)
	pool := new(mockmodule.TransactionPool)
	collections := new(mockstorage.Collections)
	guarantees := new(mockstorage.Guarantees)

	e, err := New(log, conf, stub, me, state, provider, pool, collections, guarantees)
	require.NoError(t, err)

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

}

func TestProposalEngine(t *testing.T) {

	log := zerolog.New(os.Stderr)

	state := new(mockprotocol.State)
	me := new(mockmodule.Local)
	me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	stub := stub.NewNetwork(state, me, hub)

	conf := Config{
		ProposalPerid: time.Millisecond,
	}

	provider := new(mocknetwork.Engine)
	pool := new(mockmodule.TransactionPool)
	collections := new(mockstorage.Collections)
	guarantees := new(mockstorage.Guarantees)

	t.Run("should propose collection when txpool is non-empty", func(t *testing.T) {
		e, err := New(log, conf, stub, me, state, provider, pool, collections, guarantees)
		require.NoError(t, err)

		tx := unittest.TransactionFixture()

		pool.On("Size").Return(uint(1)).Once()
		pool.On("All").Return([]*flow.Transaction{&tx}).Once()
		collections.On("Save", mock.Anything).Return(nil).Once()
		guarantees.On("Save", mock.Anything).Return(nil).Once()
		provider.On("ProcessLocal", mock.Anything).Return(nil).Once()

		err = e.createProposal()
		assert.NoError(t, err)

		provider.AssertCalled(t, "ProcessLocal", mock.Anything)
	})
}

func WithProposalEngine(t *testing.T, run func(e *Engine)) {
	log := zerolog.New(os.Stderr)

	state := new(mockprotocol.State)
	me := new(mockmodule.Local)
	me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	stub := stub.NewNetwork(state, me, hub)

	conf := Config{
		ProposalPerid: time.Millisecond,
	}

	provider := new(mocknetwork.Engine)
	pool := new(mockmodule.TransactionPool)
	collections := new(mockstorage.Collections)
	guarantees := new(mockstorage.Guarantees)

	e, err := New(log, conf, stub, me, state, provider, pool, collections, guarantees)
	require.NoError(t, err)

	run(e)
}
