package verifier_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type TestSuite struct {
	suite.Suite
	net   *module.Network
	state *protocol.State
	ss    *protocol.Snapshot
	me    *module.Local
	// mock conduit for submitting result approvals
	conduit *network.Conduit
}

func TestVerifierEgine(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) SetupTest() {
	suite.state = &protocol.State{}
	suite.net = &module.Network{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.conduit = &network.Conduit{}

	suite.net.On("Register", uint8(engine.ApprovalProvider), testifymock.Anything).
		Return(suite.conduit, nil).
		Once()

	suite.state.On("Final").Return(suite.ss)
}

func (suite *TestSuite) TestNewEngine() *verifier.Engine {
	e, err := verifier.New(zerolog.Logger{}, suite.net, suite.state, suite.me)
	require.Nil(suite.T(), err)

	suite.net.AssertExpectations(suite.T())
	return e
}

func (suite *TestSuite) TestInvalidSender() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentifierFixture()
	invalidID := unittest.IdentifierFixture()

	suite.me.On("NodeID").Return(myID)

	completeRA := unittest.CompleteExecutionResultFixture()

	err := eng.Process(invalidID, &completeRA)
	assert.Error(suite.T(), err)
}

func (suite *TestSuite) TestIncorrectResult() {
	// TODO when ERs are verified
	suite.T().Skip()
}

func (suite *TestSuite) TestVerify() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	completeER := unittest.CompleteExecutionResultFixture()

	suite.me.On("NodeID").Return(myID).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil).Once()
	suite.conduit.
		On("Submit", testifymock.Anything, consensusNodes.Get(0).NodeID).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			// check that the approval matches the input execution result
			ra, ok := args[0].(*flow.ResultApproval)
			suite.Assert().True(ok)
			suite.Assert().Equal(completeER.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Once()

	err := eng.Process(myID, &completeER)
	suite.Assert().Nil(err)

	suite.me.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.conduit.AssertExpectations(suite.T())
}

// checks that an execution result received by the verification node results in:
// - request of the appropriate collection
// - formation of a complete execution result by the ingest engine
// - broadcast of a matching result approval to consensus nodes
func TestHappyPath(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIDList := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conID := conIDList.Get(0)

	identities := flow.IdentityList{colID, conID, exeID, verID}
	genesis := flow.Genesis(identities)

	verNode := testutil.VerificationNode(t, hub, verID, genesis)
	colNode := testutil.CollectionNode(t, hub, colID, genesis)

	completeER := unittest.CompleteExecutionResultFixture()

	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeID, genesis)
	exeEngine := new(network.Engine)
	exeConduit, err := exeNode.Net.Register(engine.ExecutionStateProvider, exeEngine)
	assert.Nil(t, err)
	exeEngine.On("Process", verID.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			req, ok := args[1].(*messages.ExecutionStateRequest)
			require.True(t, ok)
			assert.Equal(t, completeER.Receipt.ExecutionResult.Chunks.ByIndex(0).ID(), req.ChunkID)

			res := &messages.ExecutionStateResponse{
				State: *completeER.ChunkStates[0],
			}
			err := exeConduit.Submit(res, verID.NodeID)
			assert.Nil(t, err)
		}).
		Return(nil).
		Once()

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conID, genesis)
	conEngine := new(network.Engine)
	conEngine.On("Process", verID.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			ra, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
			assert.Equal(t, completeER.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Return(nil).
		Once()
	_, err = conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// assume the verification node has received the block
	err = verNode.Blocks.Add(completeER.Block)
	assert.Nil(t, err)

	// inject the collection into the collection node mempool
	err = colNode.Collections.Store(completeER.Collections[0])
	assert.Nil(t, err)

	// send the ER from execution to verification node
	err = verNode.ReceiptsEngine.Process(exeID.NodeID, completeER.Receipt)
	assert.Nil(t, err)

	// the receipt should be added to the mempool
	assert.True(t, verNode.Receipts.Has(completeER.Receipt.ID()))

	// flush the chunk state request
	verNet, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)
	verNet.DeliverSome(func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.ExecutionStateProvider
	})

	// flush the chunk state response
	exeNet, ok := hub.GetNetwork(exeID.NodeID)
	assert.True(t, ok)
	exeNet.DeliverSome(func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.ExecutionStateProvider
	})

	// the chunk state should be added to the mempool
	assert.True(t, verNode.ChunkStates.Has(completeER.ChunkStates[0].ID()))

	// flush the collection request
	verNet.DeliverSome(func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.CollectionProvider
	})

	// flush the collection response
	colNet, ok := hub.GetNetwork(colID.NodeID)
	assert.True(t, ok)
	colNet.DeliverSome(func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.CollectionProvider
	})

	// flush the result approval broadcast
	verNet.FlushAll()

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// the receipt should be removed from the mempool
	assert.False(t, verNode.Receipts.Has(completeER.Receipt.ID()))
	// associated resources should be removed from the mempool
	assert.False(t, verNode.Collections.Has(completeER.Collections[0].ID()))
	assert.False(t, verNode.ChunkStates.Has(completeER.ChunkStates[0].ID()))
	assert.False(t, verNode.Blocks.Has(completeER.Block.ID()))
}

// Test that ERs received multiple times, or concurrently with other ERs
// are verified exactly once.
func TestConcurrency(t *testing.T) {
	testcases := []struct{ erCount, senderCount int }{
		{
			erCount:     1,
			senderCount: 1,
		}, {
			erCount:     1,
			senderCount: 10,
		}, {
			erCount:     10,
			senderCount: 1,
		}, {
			erCount:     10,
			senderCount: 10,
		},
	}

	for _, testcase := range testcases {
		name := fmt.Sprintf("%d-ERs/%d-senders", testcase.erCount, testcase.senderCount)
		t.Run(name, func(t *testing.T) {
			testConcurrency(t, testcase.erCount, testcase.senderCount)
		})
	}
}

func testConcurrency(t *testing.T, erCount, senderCount int) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIDList := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conID := conIDList.Get(0)

	identities := flow.IdentityList{colID, conID, exeID, verID}
	genesis := flow.Genesis(identities)

	verNode := testutil.VerificationNode(t, hub, verID, genesis)
	colNode := testutil.CollectionNode(t, hub, colID, genesis)

	// create `erCount` ER fixtures that will be concurrently delivered
	ers := make([]verification.CompleteExecutionResult, 0, erCount)
	for i := 0; i < erCount; i++ {
		ers = append(ers, unittest.CompleteExecutionResultFixture())
	}

	// mock the execution node with a generic node and mocked engine
	// to handle requests for chunk state
	exeNode := testutil.GenericNode(t, hub, exeID, genesis)
	setupMockExeNode(t, exeNode, verID.NodeID, ers)

	// mock the consensus node with a generic node and mocked engine to assert
	// that each result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conID, genesis)
	// the wait group represents consensus node waiting on result approvals
	receiverWG := setupMockConNode(t, conNode, verID.NodeID, ers)

	verNet, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)

	// the wait group tracks goroutines for each ER sending it to VER
	var senderWG sync.WaitGroup
	senderWG.Add(erCount * senderCount)

	for _, completeER := range ers {
		for _, coll := range completeER.Collections {
			err := colNode.Collections.Store(coll)
			assert.Nil(t, err)
		}

		// spin up `senderCount` sender goroutines to mimic receiving
		// the same resource multiple times
		for i := 0; i < senderCount; i++ {
			go func(j int, id flow.Identifier, block *flow.Block, receipt *flow.ExecutionReceipt) {

				sendBlock := func() {
					_ = verNode.ReceiptsEngine.Process(conID.NodeID, block)
				}

				sendReceipt := func() {
					_ = verNode.ReceiptsEngine.Process(exeID.NodeID, receipt)
				}

				switch rand.Intn(2) {
				case 0:
					// block then receipt
					sendBlock()
					verNet.FlushAll()
					time.Sleep(1)
					sendReceipt()
				case 1:
					// receipt then block
					sendReceipt()
					verNet.FlushAll()
					time.Sleep(1)
					sendBlock()
				}

				verNet.FlushAll()
				go senderWG.Done()
			}(i, completeER.Receipt.ExecutionResult.ID(), completeER.Block, completeER.Receipt)
		}
	}

	// wait for all ERs to be sent to VER
	assert.True(t, unittest.ReturnsBefore(senderWG.Wait, time.Second))
	verNet.FlushAll()
	// wait for all RAs to be received by CON
	assert.True(t, unittest.ReturnsBefore(receiverWG.Wait, time.Second))
}

// setupMockExeNode sets up a mocked execution node that responds to requests for
// chunk states. Any requests that don't correspond to an execution receipt in
// the input ers list result in the test failing.
func setupMockExeNode(t *testing.T, node mock.GenericNode, verID flow.Identifier, ers []verification.CompleteExecutionResult) {
	eng := new(network.Engine)
	conduit, err := node.Net.Register(engine.ExecutionStateProvider, eng)
	assert.Nil(t, err)

	eng.On("Process", verID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			req, ok := args[1].(*messages.ExecutionStateRequest)
			require.True(t, ok)

			for _, er := range ers {
				if er.Receipt.ExecutionResult.Chunks.Chunks[0].ID() == req.ChunkID {
					res := &messages.ExecutionStateResponse{
						State: *er.ChunkStates[0],
					}
					err := conduit.Submit(res, verID)
					assert.Nil(t, err)
					return
				}
			}
			t.Log("invalid chunk state request", req.ChunkID.String())
			t.Fail()
		}).
		Return(nil)
}

// setupMockConNode sets up a mocked consensus node to assert that a set of
// result approvals are delivered correctly.
func setupMockConNode(t *testing.T, node mock.GenericNode, verID flow.Identifier, ers []verification.CompleteExecutionResult) *sync.WaitGroup {
	eng := new(network.Engine)
	_, err := node.Net.Register(engine.ApprovalProvider, eng)
	assert.Nil(t, err)

	// keep track of which result approvals we have received
	receivedRAs := make(map[flow.Identifier]struct{})
	// we decrement the wait group when each RA is received
	var wg sync.WaitGroup
	wg.Add(len(ers))

	eng.On("Process", verID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			ra, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)

			erID := ra.ResultApprovalBody.ExecutionResultID
			_, alreadyReceived := receivedRAs[erID]
			if alreadyReceived {
				t.Logf("received RA (with ER ID %s) twice", erID)
				t.Fail()
				return
			}

			receivedRAs[erID] = struct{}{}
			wg.Done()
		}).
		Return(nil)

	return &wg
}
