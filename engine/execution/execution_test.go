package execution

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleCollection
	})
	conID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleConsensus
	})
	exeID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleExecution
	})
	// verID := unittest.IdentityFixture(func(id *flow.Identity) {
	// 	id.Role = flow.RoleVerification
	// })

	identities := flow.IdentityList{colID, conID, exeID}

	genesis := mock.Genesis(identities)

	_ = testutil.CollectionNode(t, hub, colID, genesis)
	_ = testutil.ConsensusNode(t, hub, conID, genesis)
	exeNode := testutil.ExecutionNode(t, hub, exeID, genesis)
	// verNode := testutil.VerificationNode(t, hub, verID, genesis)

	defer func() {
		exeNode.BadgerDB.Close()
		exeNode.LevelDB.SafeClose()
	}()

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	col := flow.Collection{Transactions: []flow.TransactionBody{tx1, tx2}}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signatures:   nil,
	}

	content := flow.Content{
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}

	block := &flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		Content: content,
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	executableBlock := &executor.ExecutableBlock{
		Block: block,
		Collections: []*executor.ExecutableCollection{
			{
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
		PreviousResultID: flow.ZeroID,
	}

	err := exeNode.BlocksEngine.Process(flow.Identifier{42}, block)
	assert.NoError(t, err)

	err = exeNode.BlocksEngine.Process(flow.Identifier{84}, &messages.CollectionResponse{
		Collection: col,
	})
	assert.NoError(t, err)

	net, ok := hub.GetNetwork(exeNode.Me.NodeID())
	require.True(t, ok)

	// wait for receipt engine to finish processing
	exeNode.ReceiptsEngine.Wait()

	var receipt *flow.ExecutionReceipt

	net.FlushAllExcept(func(message *stub.PendingMessage) bool {
		event, ok := message.Event.(*flow.ExecutionReceipt)
		if ok {
			receipt = event
			return false
		}

		return true
	})

	require.NotNil(t, receipt)

	view := exeNode.State.NewView(receipt.ExecutionResult.FinalStateCommitment)

	_, err = exeNode.VM.NewBlockContext(executableBlock.Block).ExecuteScript(view, []byte(`
		pub fun main(): Int {
		  return 5
		}
	`))
}

func makeRealBlock(n int) (flow.Block, []flow.Collection) {
	colls := make([]flow.Collection, n)
	collsGuarantees := make([]*flow.CollectionGuarantee, n)

	for i, _ := range colls {
		tx := unittest.TransactionBodyFixture()
		colls[i].Transactions = []flow.TransactionBody{tx}

		collsGuarantees[i] = &flow.CollectionGuarantee{
			CollectionID: colls[i].ID(),
		}
	}

	block := unittest.BlockFixture()
	block.Guarantees = collsGuarantees
	return block, colls
}

const counterScript = `

  pub contract Counting {

      pub resource Counter {
          pub var count: Int

          init() {
              self.count = 0
          }

          pub fun add(_ count: Int) {
              self.count = self.count + count
          }
      }

      pub fun createCounter(): @Counter {
          return <-create Counter()
      }
  }
`

// generateAddTwoToCounterScript generates a script that increments a counter.
// If no counter exists, it is created.
func generateAddTwoToCounterScript(counterAddress flow.Address) string {
	return fmt.Sprintf(
		`
            import 0x%s

            transaction {

                prepare(signer: Account) {
                    if signer.storage[Counting.Counter] == nil {
                        let existing <- signer.storage[Counting.Counter] <- Counting.createCounter()
                        destroy existing

                        signer.published[&Counting.Counter] = &signer.storage[Counting.Counter] as Counting.Counter
                    }

                    signer.published[&Counting.Counter]?.add(2)
                }
            }
        `,
		counterAddress,
	)
}

func generateGetCounterCountScript(counterAddress flow.Address, accountAddress flow.Address) string {
	return fmt.Sprintf(
		`
            import 0x%s

            pub fun main(): Int {
                return getAccount(0x%s).published[&Counting.Counter]?.count ?? 0
            }
        `,
		counterAddress,
		accountAddress,
	)
}
