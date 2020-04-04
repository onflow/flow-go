package computation

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/dapperlabs/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/unittest"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	module "github.com/dapperlabs/flow-go/module/mock"
)

func TestComputeBlockWithStorage(t *testing.T) {
	encoded := hex.EncodeToString([]byte(`
			access(all) contract Container {
				access(all) resource Counter {
					pub var count: Int
		
					init(_ v: Int) {
						self.count = v
					}
					pub fun add(_ count: Int) {
						self.count = self.count + count
					}
				}
				pub fun createCounter(_ v: Int): @Counter {
					return <-create Counter(v)
				}
			}`))

	tx1 := flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }`, encoded)),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}

	tx2 := flow.TransactionBody{
		Script: []byte(`

			import 0x01

			transaction {
				prepare(acc: AuthAccount) {
					if acc.storage[Container.Counter] == nil {
                		let existing <- acc.storage[Container.Counter] <- Container.createCounter(3)
                		destroy existing
					}
              	}
            }`),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := flow.Block{
		Header: flow.Header{
			View: 42,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
	}

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	rt := runtime.NewInterpreterRuntime()

	vm := virtualmachine.New(rt)

	blockComputer := computer.NewBlockComputer(vm)

	engine := &Manager{
		blockComputer: blockComputer,
		me:            me,
	}

	view := unittest.EmptyView()

	require.Empty(t, view.Delta())

	returnedComputationResult, err := engine.ComputeBlock(executableBlock, view)
	require.NoError(t, err)

	require.NotEmpty(t, view.Delta())
	require.Len(t, returnedComputationResult.StateViews, 1)
	assert.NotEmpty(t, returnedComputationResult.StateViews[0].Delta())
}
