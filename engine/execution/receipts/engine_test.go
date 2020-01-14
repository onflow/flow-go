package receipts

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionReceiptProviderEngine_ProcessExecutionResult(t *testing.T) {
	targetIDs := flow.IdentityList{
		unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleConsensus
		}),
		unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleVerification
		}),
	}

	result := unittest.ExecutionResultFixture()

	t.Run("failed to load identities", func(t *testing.T) {
		state := &protocol.State{}
		ss := &protocol.Snapshot{}
		con := &network.Conduit{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{
			state: state,
			con:   con,
			me:    me,
		}

		state.On("Final").Return(ss)
		ss.On("Identities", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("identity error"))

		err := e.onExecutionResult(e.me.NodeID(), &result)
		assert.Error(t, err)

		state.AssertExpectations(t)
		ss.AssertExpectations(t)
	})

	t.Run("failed to broadcast", func(t *testing.T) {
		state := &protocol.State{}
		ss := &protocol.Snapshot{}
		con := &network.Conduit{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{
			state: state,
			con:   con,
			me:    me,
		}

		state.On("Final").Return(ss)
		ss.On("Identities", mock.Anything, mock.Anything).Return(targetIDs, nil)
		con.On(
			"Submit",
			mock.Anything,
			targetIDs[0].NodeID,
			targetIDs[1].NodeID,
		).
			Return(fmt.Errorf("network error"))

		err := e.onExecutionResult(e.me.NodeID(), &result)
		assert.Error(t, err)

		state.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
	})

	t.Run("non-local engine", func(t *testing.T) {
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{me: me}

		// submit using origin ID that does not match node ID
		err := e.onExecutionResult(flow.Identifier{42}, &result)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		state := &protocol.State{}
		ss := &protocol.Snapshot{}
		con := &network.Conduit{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{
			state: state,
			con:   con,
			me:    me,
		}

		state.On("Final").Return(ss)
		ss.On("Identities", mock.Anything, mock.Anything).Return(targetIDs, nil)
		con.On(
			"Submit",
			mock.Anything,
			targetIDs[0].NodeID,
			targetIDs[1].NodeID,
		).
			Run(func(args mock.Arguments) {
				// check the receipt is properly formed
				receipt := args[0].(*flow.ExecutionReceipt)
				assert.Equal(t, result, receipt.ExecutionResult)
			}).
			Return(nil)

		err := e.onExecutionResult(e.me.NodeID(), &result)
		assert.NoError(t, err)

		state.AssertExpectations(t)
		ss.AssertExpectations(t)
		con.AssertExpectations(t)
	})
}
