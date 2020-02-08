package execution

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor/mocks"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionEngine_OnExecutableBlock(t *testing.T) {
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

	startState := unittest.StateCommitmentFixture()

	t.Run("non-local engine", func(t *testing.T) {
		me := new(module.Local)
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{me: me}

		view := state.NewView(func(key string) (bytes []byte, e error) {
			return nil, nil
		})

		// submit using origin ID that does not match node ID
		err := e.onCompleteBlock(flow.Identifier{42}, completeBlock, view, startState)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		receipts := new(network.Engine)
		me := new(module.Local)
		me.On("NodeID").Return(flow.ZeroID)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		exec := mocks.NewMockBlockExecutor(ctrl)

		computationResult := unittest.ComputationResultFixture()

		e := &Engine{
			provider: receipts,
			executor: exec,
			me:       me,
		}

		receipts.On(
			"SubmitLocal",
			mock.Anything,
			mock.Anything,
		).
			Run(func(args mock.Arguments) {
				receipt := args[0].(*execution.ComputationResult)

				assert.Equal(t, computationResult, receipt)
			}).
			Return(nil)

		exec.EXPECT().ExecuteBlock(gomock.Eq(completeBlock), gomock.AssignableToTypeOf(&state.View{}), gomock.AssignableToTypeOf(flow.StateCommitment{})).Return(computationResult, nil)

		view := state.NewView(func(key string) (bytes []byte, e error) {
			return nil, nil
		})

		err := e.onCompleteBlock(e.me.NodeID(), completeBlock, view, startState)
		require.NoError(t, err)

		receipts.AssertExpectations(t)
	})
}
