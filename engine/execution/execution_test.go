package execution_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	execTestutil "github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exeID, verID}

	genesis := flow.Genesis(identities)

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(1) } }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(2) } }"),
	}

	tx3 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(3) } }"),
	}

	tx4 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(4) } }"),
	}

	col1 := flow.Collection{Transactions: []*flow.TransactionBody{&tx1, &tx2}}
	col2 := flow.Collection{Transactions: []*flow.TransactionBody{&tx3, &tx4}}

	collections := map[flow.Identifier]flow.Collection{
		col1.ID(): col1,
		col2.ID(): col2,
	}

	block := &flow.Block{
		Header: flow.Header{
			ParentID: genesis.ID(),
			View:     42,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					CollectionID: col1.ID(),
					SignerIDs:    []flow.Identifier{colID.NodeID},
				},
				{
					CollectionID: col2.ID(),
					SignerIDs:    []flow.Identifier{colID.NodeID},
				},
			},
		},
	}

	proposal := unittest.ProposalFromBlock(block)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21)
	defer exeNode.Done()

	collectionNode := testutil.GenericNode(t, hub, colID, identities)
	verificationNode := testutil.GenericNode(t, hub, verID, identities)
	consensusNode := testutil.GenericNode(t, hub, conID, identities)

	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.CollectionProvider, collectionEngine)
	collectionEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			originID, _ := args[0].(flow.Identifier)
			req, _ := args[1].(*messages.CollectionRequest)

			col, exists := collections[req.ID]
			assert.True(t, exists)

			res := &messages.CollectionResponse{
				Collection: col,
			}

			err := colConduit.Submit(res, originID)
			assert.NoError(t, err)
		}).
		Return(nil).
		Times(len(collections))

	var receipt *flow.ExecutionReceipt

	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	consensusEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
			assert.Equal(t, len(collections), len(receipt.ExecutionResult.Chunks))

			for i, chunk := range receipt.ExecutionResult.Chunks {
				assert.EqualValues(t, i, chunk.CollectionIndex)
			}
		}).
		Return(nil).
		Once()

	// submit block from consensus node
	exeNode.IngestionEngine.Submit(conID.NodeID, proposal)

	assert.Eventually(t, func() bool {
		hub.DeliverAll()
		return receipt != nil
	}, time.Second*10, time.Millisecond*500)

	collectionEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)

	collectionNode.Done()
	verificationNode.Done()
	consensusNode.Done()
	exeNode.Done()
}

func TestBlockIngestionMultipleConsensusNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	con1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := flow.IdentityList{con1ID, con2ID, exeID}

	genesis := flow.Genesis(identities)

	block2 := &flow.Block{
		Header: flow.Header{
			ParentID:   genesis.ID(),
			View:       2,
			Height:     2,
			ProposerID: con1ID.ID(),
		},
	}
	fork := &flow.Block{
		Header: flow.Header{
			ParentID:   genesis.ID(),
			View:       2,
			Height:     2,
			ProposerID: con2ID.ID(),
		},
	}
	block3 := &flow.Block{
		Header: flow.Header{
			ParentID:   block2.ID(),
			View:       3,
			Height:     3,
			ProposerID: con2ID.ID(),
		},
	}

	proposal2 := unittest.ProposalFromBlock(block2)
	proposal2alt := unittest.ProposalFromBlock(fork)
	proposal3 := unittest.ProposalFromBlock(block3)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21)

	consensus1Node := testutil.GenericNode(t, hub, con1ID, identities)
	consensus2Node := testutil.GenericNode(t, hub, con2ID, identities)

	actualCalls := 0

	consensusEngine := new(network.Engine)
	_, _ = consensus1Node.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	_, _ = consensus2Node.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	consensusEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) { actualCalls++ }).
		Return(nil)

	exeNode.AssertHighestExecutedBlock(t, &genesis.Header)

	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal2alt)
	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal3) // block 3 cannot be executed if parent (block2 is missing)

	hub.Eventually(t, equal(2, &actualCalls))

	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal2)
	hub.Eventually(t, equal(6, &actualCalls)) // now block 3 and 2 can be executed

	exeNode.AssertHighestExecutedBlock(t, &block3.Header)

	consensusEngine.AssertExpectations(t)

	consensus1Node.Done()
	consensus2Node.Done()
	exeNode.Done()
}

// TODO merge this test with TestSyncFlow in engine/execution/sync_test.go
func TestExecutionStateSyncMultipleExecutionNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := flow.IdentityList{colID, conID, exe1ID, exe2ID}

	genesis := flow.Genesis(identities)

	// transaction that will change state and succeed, used to test that state commitment changes
	tx1 := execTestutil.DeployCounterContractTransaction()

	seq := uint64(0)

	err := execTestutil.SignTransactionbyRoot(&tx1, seq)
	require.NoError(t, err)
	seq++

	col1 := flow.Collection{Transactions: []*flow.TransactionBody{&tx1}}
	block2 := &flow.Block{
		Header: flow.Header{
			ParentID:   genesis.ID(),
			View:       2,
			Height:     2,
			ProposerID: conID.ID(),
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{CollectionID: col1.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
			},
		},
	}
	block2.PayloadHash = block2.Payload.Hash()
	proposal2 := unittest.ProposalFromBlock(block2)

	// transaction that will change state but then panic and revert, used to test that state commitment stays identical
	tx2 := execTestutil.CreateCounterPanicTransaction()
	err = execTestutil.SignTransactionbyRoot(&tx2, seq)
	require.NoError(t, err)

	col2 := flow.Collection{Transactions: []*flow.TransactionBody{&tx2}}
	block3 := &flow.Block{
		Header: flow.Header{
			ParentID:   block2.ID(),
			View:       3,
			Height:     3,
			ProposerID: conID.ID(),
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{CollectionID: col2.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
			},
		},
	}
	block3.PayloadHash = block3.Payload.Hash()
	proposal3 := unittest.ProposalFromBlock(block3)

	// setup mocks and assertions
	collectionNode := testutil.GenericNode(t, hub, colID, identities)
	defer collectionNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities)
	defer consensusNode.Done()
	exe1Node := testutil.ExecutionNode(t, hub, exe1ID, identities, 27)
	defer exe1Node.Done()
	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.CollectionProvider, collectionEngine)
	collectionEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			originID, _ := args[0].(flow.Identifier)
			req := args[1].(*messages.CollectionRequest)
			if req.ID == col1.ID() {
				err := colConduit.Submit(&messages.CollectionResponse{Collection: col1}, originID)
				assert.NoError(t, err)
			} else if req.ID == col2.ID() {
				err := colConduit.Submit(&messages.CollectionResponse{Collection: col2}, originID)
				assert.NoError(t, err)
			} else {
				assert.Fail(t, "requesting unexpected collection", req.ID)
			}
		}).
		Return(nil).
		Twice()

	receiptsReceived := 0

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	consensusEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receiptsReceived++
			sender := args[0].(flow.Identifier)
			receipt := args[1].(*flow.ExecutionReceipt)
			fmt.Printf("Got execution receipt %v from %v for block %v with state commitment %#x\n", receiptsReceived, sender, receipt.ExecutionResult.BlockID.String(), receipt.ExecutionResult.FinalStateCommit)

		}).Return(nil)

	// submit block2 from consensus node to execution node 1
	exe1Node.IngestionEngine.Submit(conID.NodeID, proposal2)

	// esure block has been executed
	hub.Eventually(t, equal(1, &receiptsReceived))
	exe1Node.AssertHighestExecutedBlock(t, &block2.Header)
	scExe1Genesis, err := exe1Node.ExecutionState.StateCommitmentByBlockID(genesis.ID())
	assert.NoError(t, err)
	scExe1Block2, err := exe1Node.ExecutionState.StateCommitmentByBlockID(block2.ID())
	assert.NoError(t, err)
	assert.NotEqual(t, scExe1Genesis, scExe1Block2)

	// start execution node 2 with sync threshold 0 so it starts state sync right away
	exe2Node := testutil.ExecutionNode(t, hub, exe2ID, identities, 0)
	defer exe2Node.Done()
	exe2Node.AssertHighestExecutedBlock(t, &genesis.Header)

	// submit block3 from consensus node to execution node 2 (who does not have block2), but not to execution node 1
	exe2Node.IngestionEngine.Submit(conID.NodeID, proposal3)

	// esure block 2 and 3 have been executed
	hub.Eventually(t, equal(3, &receiptsReceived))

	// ensure state has been synced across both nodes
	exe1Node.AssertHighestExecutedBlock(t, &block2.Header)
	exe2Node.AssertHighestExecutedBlock(t, &block3.Header)

	// verify state commitment is the same across nodes
	scExe2Block2, err := exe2Node.ExecutionState.StateCommitmentByBlockID(block2.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe1Block2, scExe2Block2)

	// verify state commitment of block 3 is the same as block 2, since tx failed
	scExe2Block3, err := exe2Node.ExecutionState.StateCommitmentByBlockID(block3.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe2Block2, scExe2Block3)

	collectionEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)
}

func TestBroadcastToMultipleVerificationNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	ver2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, exeID, ver1ID, ver2ID}

	genesis := flow.Genesis(identities)

	block := &flow.Block{
		Header: flow.Header{
			ParentID: genesis.ID(),
			View:     42,
		},
	}
	proposal := unittest.ProposalFromBlock(block)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21)
	defer exeNode.Done()

	verification1Node := testutil.GenericNode(t, hub, ver1ID, identities)
	verification2Node := testutil.GenericNode(t, hub, ver2ID, identities)

	actualCalls := 0

	var receipt *flow.ExecutionReceipt

	verificationEngine := new(network.Engine)
	_, _ = verification1Node.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	_, _ = verification2Node.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			actualCalls++

			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil)

	exeNode.IngestionEngine.SubmitLocal(proposal)

	hub.Eventually(t, equal(2, &actualCalls))

	verificationEngine.AssertExpectations(t)

	verification1Node.Done()
	verification2Node.Done()
	exeNode.Done()
}

func equal(expected int, actual *int) func() bool {
	return func() bool {
		fmt.Printf("expect %d got %d\n", expected, *actual)
		return expected == *actual
	}
}
