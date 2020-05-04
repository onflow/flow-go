package execution_test

import (
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

	genesis := unittest.GenesisFixture(identities)

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

	block := unittest.BlockWithParentFixture(genesis.Header)
	block.Header.View = 42
	block.SetPayload(flow.Payload{
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
	})

	proposal := unittest.ProposalFromBlock(&block)

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

	genesis := unittest.GenesisFixture(identities)

	block1 := unittest.BlockWithParentFixture(genesis.Header)
	block1.Header.View = 2
	block1.Header.ProposerID = con1ID.ID()
	block1.SetPayload(flow.Payload{})

	block1b := unittest.BlockWithParentFixture(genesis.Header)
	block1b.Header.View = 2
	block1b.Header.ProposerID = con2ID.ID()
	block1b.SetPayload(flow.Payload{})

	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.Header.View = 3
	block2.Header.ProposerID = con2ID.ID()
	block2.SetPayload(flow.Payload{})

	proposal1 := unittest.ProposalFromBlock(&block1)
	proposal1b := unittest.ProposalFromBlock(&block1b)
	proposal2 := unittest.ProposalFromBlock(&block2)

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

	exeNode.AssertHighestExecutedBlock(t, genesis.Header)

	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal1b)
	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal2) // block 2 cannot be executed if parent (block1 is missing)

	hub.Eventually(t, func() bool {
		return actualCalls == 2
	})

	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal1)
	hub.Eventually(t, func() bool {
		return actualCalls == 6
	}) // now block 3 and 2 can be executed

	exeNode.AssertHighestExecutedBlock(t, block2.Header)

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

	genesis := unittest.GenesisFixture(identities)

	// transaction that will change state and succeed, used to test that state commitment changes
	tx1 := execTestutil.DeployCounterContractTransaction()

	seq := uint64(0)

	err := execTestutil.SignTransactionByRoot(&tx1, seq)
	require.NoError(t, err)
	seq++

	col1 := flow.Collection{Transactions: []*flow.TransactionBody{&tx1}}
	block1 := unittest.BlockWithParentFixture(genesis.Header)
	block1.Header.View = 2
	block1.Header.ProposerID = conID.ID()
	block1.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{CollectionID: col1.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
		},
	})

	proposal1 := unittest.ProposalFromBlock(&block1)

	// transaction that will change state but then panic and revert, used to test that state commitment stays identical
	tx2 := execTestutil.CreateCounterPanicTransaction()
	err = execTestutil.SignTransactionByRoot(&tx2, seq)
	require.NoError(t, err)

	col2 := flow.Collection{Transactions: []*flow.TransactionBody{&tx2}}
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.Header.View = 3
	block2.Header.ProposerID = conID.ID()
	block2.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{CollectionID: col2.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
		},
	})
	proposal2 := unittest.ProposalFromBlock(&block2)

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
			originID := args[0].(flow.Identifier)
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
			originID := args[0].(flow.Identifier)
			receipt := args[1].(*flow.ExecutionReceipt)
			consensusNode.Log.Debug().
				Hex("origin", originID[:]).
				Hex("block", receipt.ExecutionResult.BlockID[:]).
				Hex("commit", receipt.ExecutionResult.FinalStateCommit).
				Msg("execution receipt delivered")

		}).Return(nil)

	// submit block2 from consensus node to execution node 1
	exe1Node.IngestionEngine.Submit(conID.NodeID, proposal1)

	// ensure block 1 has been executed
	hub.Eventually(t, func() bool {
		return receiptsReceived == 1
	})
	exe1Node.AssertHighestExecutedBlock(t, block1.Header)
	scExe1Genesis, err := exe1Node.ExecutionState.StateCommitmentByBlockID(genesis.ID())
	assert.NoError(t, err)
	scExe1Block1, err := exe1Node.ExecutionState.StateCommitmentByBlockID(block1.ID())
	assert.NoError(t, err)
	assert.NotEqual(t, scExe1Genesis, scExe1Block1)

	// start execution node 2 with sync threshold 0 so it starts state sync right away
	exe2Node := testutil.ExecutionNode(t, hub, exe2ID, identities, 0)
	defer exe2Node.Done()
	exe2Node.AssertHighestExecutedBlock(t, genesis.Header)

	// submit block2 from consensus node to execution node 2 (who does not have block1), but not to execution node 1
	exe2Node.IngestionEngine.Submit(conID.NodeID, proposal2)

	// ensure block 1 and 2 have been executed
	hub.Eventually(t, func() bool {
		return receiptsReceived == 2
	})

	// ensure state has been synced across both nodes
	exe1Node.AssertHighestExecutedBlock(t, block1.Header)
	exe2Node.AssertHighestExecutedBlock(t, block2.Header)

	// verify state commitment is the same across nodes
	scExe2Block1, err := exe2Node.ExecutionState.StateCommitmentByBlockID(block1.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe1Block1, scExe2Block1)

	// verify state commitment of block 2 is the same as block 1, since tx failed
	scExe2Block2, err := exe2Node.ExecutionState.StateCommitmentByBlockID(block2.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe2Block1, scExe2Block2)

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

	genesis := unittest.GenesisFixture(identities)

	block := unittest.BlockWithParentFixture(genesis.Header)
	block.Header.View = 42
	block.SetPayload(flow.Payload{})
	proposal := unittest.ProposalFromBlock(&block)

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

	hub.Eventually(t, func() bool {
		return actualCalls == 2
	})

	verificationEngine.AssertExpectations(t)

	verification1Node.Done()
	verification2Node.Done()
	exeNode.Done()
}
