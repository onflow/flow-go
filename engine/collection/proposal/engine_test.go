package proposal_test

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/model"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
)

func TestStartStop(t *testing.T) {
	log := zerolog.New(os.Stderr)

	state := new(protocol.State)
	me := new(module.Local)
	me.On("NodeID").Return(model.Identifier{})

	hub := stub.NewNetworkHub()
	stub := stub.NewNetwork(state, me, hub)

	e, err := proposal.New(log, stub, me, state)
	require.NoError(t, err)

	t.Run("should start and stop promptly", func(t *testing.T) {
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
