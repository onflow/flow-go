package execution

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution"
	executor "github.com/dapperlabs/flow-go/engine/execution/execution/executor/mock"
	state "github.com/dapperlabs/flow-go/engine/execution/execution/state/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionEngine_OnCompleteBlock(t *testing.T) {
	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signatures:   nil,
	}

	block := flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	completeBlock := &execution.CompleteBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*execution.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
	}

	t.Run("non-local engine", func(t *testing.T) {
		me := new(module.Local)
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{me: me}

		// submit using origin ID that does not match node ID
		err := e.onCompleteBlock(flow.Identifier{42}, completeBlock)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		exec := new(executor.BlockExecutor)
		receipts := new(network.Engine)
		me := new(module.Local)
		me.On("NodeID").Return(flow.ZeroID)

		e := &Engine{
			receipts: receipts,
			executor: exec,
			me:       me,
		}

		receipts.On("SubmitLocal", mock.Anything)
		exec.On("ExecuteBlock", completeBlock).Return(&flow.ExecutionResult{}, nil)

		err := e.onCompleteBlock(e.me.NodeID(), completeBlock)
		require.NoError(t, err)

		receipts.AssertExpectations(t)
		exec.AssertExpectations(t)
	})
}

func TestExecutionEngine_OnExecutionStateRequest(t *testing.T) {
	t.Run("non-verification engine", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)

		e := Engine{protoState: ps}

		originID := unittest.IdentifierFixture()
		chunkID := unittest.IdentifierFixture()

		ps.On("Final").Return(ss)
		ss.On("Identity", originID).
			Return(unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)), nil)

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		// submit using origin ID with invalid role
		err := e.onExecutionStateRequest(originID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
	})

	t.Run("non-existent chunk", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)
		es := new(state.ExecutionState)

		e := Engine{protoState: ps, execState: es}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkID := unittest.IdentifierFixture()

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		es.On("GetChunkRegisters", chunkID).Return(nil, fmt.Errorf("state error"))

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		err := e.onExecutionStateRequest(originIdentity.NodeID, req)
		assert.Error(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		es.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		ps := new(protocol.State)
		ss := new(protocol.Snapshot)
		es := new(state.ExecutionState)
		con := new(network.Conduit)

		e := Engine{protoState: ps, execState: es, execStateConduit: con}

		originIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

		chunkHeader := unittest.ChunkHeaderFixture()
		chunkID := chunkHeader.ChunkID

		registerIDs := chunkHeader.RegisterIDs
		registerValues := []flow.RegisterValue{{1}, {2}, {3}}

		expectedRegisters := flow.Ledger{
			string(registerIDs[0]): registerValues[0],
			string(registerIDs[1]): registerValues[1],
			string(registerIDs[2]): registerValues[2],
		}

		ps.On("Final").Return(ss)
		ss.On("Identity", originIdentity.NodeID).Return(originIdentity, nil)
		es.On("GetChunkRegisters", chunkID).Return(expectedRegisters, nil)
		con.On("Submit", mock.Anything, originIdentity.NodeID).
			Run(func(args mock.Arguments) {
				res, ok := args[0].(*messages.ExecutionStateResponse)
				require.True(t, ok)

				actualChunkID := res.State.ChunkID
				actualRegisters := res.State.Registers

				assert.Equal(t, chunkID, actualChunkID)
				assert.EqualValues(t, expectedRegisters, actualRegisters)
			}).
			Return(nil)

		req := &messages.ExecutionStateRequest{ChunkID: chunkID}

		err := e.onExecutionStateRequest(originIdentity.NodeID, req)
		assert.NoError(t, err)

		ps.AssertExpectations(t)
		ss.AssertExpectations(t)
		es.AssertExpectations(t)
		con.AssertExpectations(t)
	})
}
