package execution_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/engine"
	execTestutil "github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	network "github.com/onflow/flow-go/network/mock"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Testnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, conID, exeID, verID)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
	defer exeNode.Done()

	genesis, err := exeNode.State.AtHeight(0).Head()
	require.NoError(t, err)

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

	collections := map[flow.Identifier]*flow.Collection{
		col1.ID(): &col1,
		col2.ID(): &col2,
	}

	block := unittest.BlockWithParentFixture(genesis)
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

	collectionNode := testutil.GenericNode(t, hub, colID, identities, chainID)
	defer collectionNode.Done()
	verificationNode := testutil.GenericNode(t, hub, verID, identities, chainID)
	defer verificationNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()

	providerEngine := new(network.Engine)
	provConduit, _ := collectionNode.Net.Register(engine.ProvideCollections, providerEngine)
	providerEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			originID := args.Get(0).(flow.Identifier)
			req := args.Get(1).(*messages.EntityRequest)

			var entities []flow.Entity
			for _, entityID := range req.EntityIDs {
				coll, exists := collections[entityID]
				require.True(t, exists)
				entities = append(entities, coll)
			}

			var blobs [][]byte
			for _, entity := range entities {
				blob, _ := msgpack.Marshal(entity)
				blobs = append(blobs, blob)
			}

			res := &messages.EntityResponse{
				Nonce:     req.Nonce,
				EntityIDs: req.EntityIDs,
				Blobs:     blobs,
			}

			err := provConduit.Submit(res, originID)
			assert.NoError(t, err)
		}).
		Once().
		Return(nil)

	var receipt *flow.ExecutionReceipt

	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ReceiveReceipts, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ReceiveReceipts, consensusEngine)
	consensusEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
			assert.Equal(t, len(collections), len(receipt.ExecutionResult.Chunks)-1) // don't count system chunk

			for i, chunk := range receipt.ExecutionResult.Chunks {
				assert.EqualValues(t, i, chunk.CollectionIndex)
			}
		}).
		Return(nil).
		Once()

	// submit block from consensus node
	exeNode.IngestionEngine.Submit(conID.NodeID, proposal)

	require.Eventually(t, func() bool {
		hub.DeliverAll()
		return receipt != nil
	}, time.Second*10, time.Millisecond*500)

	providerEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)
}

func TestBlockIngestionMultipleConsensusNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Testnet

	con1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := unittest.CompleteIdentitySet(con1ID, con2ID, exeID)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
	defer exeNode.Done()

	consensus1Node := testutil.GenericNode(t, hub, con1ID, identities, chainID)
	defer consensus1Node.Done()
	consensus2Node := testutil.GenericNode(t, hub, con2ID, identities, chainID)
	defer consensus2Node.Done()

	genesis, err := exeNode.State.AtHeight(0).Head()
	require.NoError(t, err)

	block1 := unittest.BlockWithParentFixture(genesis)
	block1.Header.View = 1
	block1.Header.ProposerID = con1ID.ID()
	block1.SetPayload(flow.Payload{})

	block1b := unittest.BlockWithParentFixture(genesis)
	block1b.Header.View = 1
	block1b.Header.ProposerID = con2ID.ID()
	block1b.SetPayload(flow.Payload{})

	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.Header.View = 2
	block2.Header.ProposerID = con2ID.ID()
	block2.SetPayload(flow.Payload{})

	proposal1 := unittest.ProposalFromBlock(&block1)
	proposal1b := unittest.ProposalFromBlock(&block1b)
	proposal2 := unittest.ProposalFromBlock(&block2)

	actualCalls := 0

	consensusEngine := new(network.Engine)
	_, _ = consensus1Node.Net.Register(engine.ReceiveReceipts, consensusEngine)
	_, _ = consensus2Node.Net.Register(engine.ReceiveReceipts, consensusEngine)
	consensusEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) { actualCalls++ }).
		Return(nil)

	exeNode.AssertHighestExecutedBlock(t, genesis)

	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal1b)
	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal2) // block 2 cannot be executed if parent (block1 is missing)

	hub.DeliverAllEventually(t, func() bool {
		return actualCalls == 2
	})

	exeNode.IngestionEngine.Submit(con1ID.NodeID, proposal1)
	hub.DeliverAllEventually(t, func() bool {
		return actualCalls == 6
	}) // now block 3 and 2 can be executed

	exeNode.AssertHighestExecutedBlock(t, block2.Header)

	consensusEngine.AssertExpectations(t)
}

// Tests synchronisation and make sure it picks up blocks
// processing after syncing range (block3)
func TestExecutionStateSyncMultipleExecutionNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := unittest.CompleteIdentitySet(colID, conID, exe1ID, exe2ID)

	collectionNode := testutil.GenericNode(t, hub, colID, identities, chainID)
	defer collectionNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()
	exe1Node := testutil.ExecutionNode(t, hub, exe1ID, identities, 27, chainID)
	defer exe1Node.Done()

	genesis, err := exe1Node.State.AtHeight(0).Head()
	require.NoError(t, err)

	seq := uint64(0)

	// transaction that will change state and succeed, used to test that state commitment changes
	chain := exe1Node.ChainID.Chain()
	tx1 := execTestutil.DeployCounterContractTransaction(chain.ServiceAddress(), chain)
	err = execTestutil.SignTransactionAsServiceAccount(tx1, seq, chain)
	require.NoError(t, err)
	seq++

	col1 := &flow.Collection{Transactions: []*flow.TransactionBody{tx1}}
	block1 := unittest.BlockWithParentFixture(genesis)
	block1.Header.View = 1
	block1.Header.ProposerID = conID.ID()
	block1.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{CollectionID: col1.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
		},
	})

	proposal1 := unittest.ProposalFromBlock(&block1)
	blob1, _ := msgpack.Marshal(col1)

	// transaction that will change state but then panic and revert, used to test that state commitment stays identical
	tx2 := execTestutil.CreateCounterPanicTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = execTestutil.SignTransactionAsServiceAccount(tx2, seq, chain)
	require.NoError(t, err)
	seq++

	col2 := &flow.Collection{Transactions: []*flow.TransactionBody{tx2}}
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.Header.View = 2
	block2.Header.ProposerID = conID.ID()
	block2.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{CollectionID: col2.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
		},
	})

	proposal2 := unittest.ProposalFromBlock(&block2)
	blob2, _ := msgpack.Marshal(col2)

	tx3 := execTestutil.AddToCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = execTestutil.SignTransactionAsServiceAccount(tx3, seq, chain)
	require.NoError(t, err)

	col3 := &flow.Collection{Transactions: []*flow.TransactionBody{tx3}}
	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.Header.View = 3
	block3.Header.ProposerID = conID.ID()
	block3.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{CollectionID: col3.ID(), SignerIDs: []flow.Identifier{colID.NodeID}},
		},
	})

	blob3, _ := msgpack.Marshal(col3)
	proposal3 := unittest.ProposalFromBlock(&block3)

	// setup mocks and assertions
	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.RequestCollections, collectionEngine)
	collectionEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			originID := args[0].(flow.Identifier)
			req := args[1].(*messages.EntityRequest)
			if req.EntityIDs[0] == col1.ID() {
				res := &messages.EntityResponse{Blobs: [][]byte{blob1}, EntityIDs: req.EntityIDs[:1]}
				err := colConduit.Submit(res, originID)
				assert.NoError(t, err)
			} else if req.EntityIDs[0] == col2.ID() {
				res := &messages.EntityResponse{Blobs: [][]byte{blob2}, EntityIDs: req.EntityIDs[:1]}
				err := colConduit.Submit(res, originID)
				assert.NoError(t, err)
			} else if req.EntityIDs[0] == col3.ID() {
				res := &messages.EntityResponse{Blobs: [][]byte{blob3}, EntityIDs: req.EntityIDs[:1]}
				err := colConduit.Submit(res, originID)
				assert.NoError(t, err)
			} else {
				assert.FailNow(t, "requesting unexpected collection", req.EntityIDs[0])
			}
		}).
		Return(nil).
		Times(3)

	receiptsReceived := 0

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ReceiveReceipts, consensusEngine)
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
	hub.DeliverAllEventually(t, func() bool {
		return receiptsReceived == 1
	})
	exe1Node.AssertHighestExecutedBlock(t, block1.Header)

	scExe1Genesis, err := exe1Node.ExecutionState.StateCommitmentByBlockID(context.Background(), genesis.ID())
	assert.NoError(t, err)

	scExe1Block1, err := exe1Node.ExecutionState.StateCommitmentByBlockID(context.Background(), block1.ID())
	assert.NoError(t, err)
	assert.NotEqual(t, scExe1Genesis, scExe1Block1)

	// start execution node 2 with sync threshold 0 so it starts state sync right away
	exe2Node := testutil.ExecutionNode(t, hub, exe2ID, identities, 0, chainID)
	defer exe2Node.Done()
	exe2Node.AssertHighestExecutedBlock(t, genesis)

	// submit block2 and block 3 from consensus node to execution node 2 (who does not have block1), but not to execution node 1
	err = exe2Node.IngestionEngine.Process(conID.NodeID, proposal2)
	assert.NoError(t, err)
	err = exe2Node.IngestionEngine.Process(conID.NodeID, proposal3)
	assert.NoError(t, err)

	// ensure block 1, 2 and 3 have been executed
	hub.DeliverAllEventually(t, func() bool {
		return receiptsReceived == 3
	})

	// ensure state has been synced across both nodes
	exe1Node.AssertHighestExecutedBlock(t, block1.Header)
	exe2Node.AssertHighestExecutedBlock(t, block3.Header)

	scExe2Block1, err := exe2Node.ExecutionState.StateCommitmentByBlockID(context.Background(), block1.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe1Block1, scExe2Block1)

	// verify state commitment of block 2 is the same as block 1, since tx failed
	scExe2Block2, err := exe2Node.ExecutionState.StateCommitmentByBlockID(context.Background(), block2.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe2Block1, scExe2Block2)

	collectionEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)
}

func TestExecutionQueryMissingBlocks(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Testnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := unittest.CompleteIdentitySet(colID, conID, exeID)

	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()
	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 0, chainID)
	exeNode.Ready()
	defer exeNode.Done()

	genesis, err := exeNode.State.AtHeight(0).Head()
	require.NoError(t, err)

	fmt.Println("genesis block ID", genesis.ID())
	exeNode.AssertHighestExecutedBlock(t, genesis)

	block1 := unittest.BlockWithParentFixture(genesis)
	block1.Header.View = 1
	block1.Header.ProposerID = conID.ID()
	block1.SetPayload(flow.Payload{})
	// proposal1 := unittest.ProposalFromBlock(&block1)

	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.Header.View = 2
	block2.Header.ProposerID = conID.ID()
	block2.SetPayload(flow.Payload{})
	proposal2 := unittest.ProposalFromBlock(&block2)

	// register sync engine
	syncEngine := new(network.Engine)
	syncConduit, _ := consensusNode.Net.Register(engine.SyncCommittee, syncEngine)
	syncEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			originID := args[0].(flow.Identifier)
			switch msg := args[1].(type) {
			case *messages.SyncRequest:
				consensusNode.Log.Debug().Hex("origin", originID[:]).Uint64("height", msg.Height).
					Uint64("nonce", msg.Nonce).Msg("protocol sync request received")

				res := &messages.SyncResponse{
					Height: block2.Header.Height,
					Nonce:  msg.Nonce,
				}

				err := syncConduit.Submit(res, originID)
				assert.NoError(t, err)
			case *messages.BatchRequest:
				ids := zerolog.Arr()
				for _, b := range msg.BlockIDs {
					ids.Hex(b[:])
				}

				consensusNode.Log.Debug().Hex("origin", originID[:]).Array("blockIDs", ids).
					Uint64("nonce", msg.Nonce).Msg("protocol batch request received")

				blocks := make([]*flow.Block, 0)
				for _, id := range msg.BlockIDs {
					if id == block1.ID() {
						blocks = append(blocks, &block1)
					} else if id == block2.ID() {
						blocks = append(blocks, &block2)
					} else {
						require.FailNow(t, "unknown block requested: %v", id)
					}
				}

				// send the response
				res := &messages.BlockResponse{
					Nonce:  msg.Nonce,
					Blocks: blocks,
				}

				err := syncConduit.Submit(res, originID)
				assert.NoError(t, err)
			case *messages.RangeRequest:

				consensusNode.Log.Debug().
					Hex("origin", originID[:]).
					Uint64("from_height", msg.FromHeight).
					Uint64("to_height", msg.ToHeight).
					Uint64("nonce", msg.Nonce).
					Msg("protocol range request received")

				blocks := make([]*flow.Block, 0)
				for height := msg.FromHeight; height <= msg.ToHeight; height++ {
					if height == block1.Header.Height {
						blocks = append(blocks, &block1)
					} else if height == block2.Header.Height {
						blocks = append(blocks, &block2)
					} else {
						require.FailNow(t, "unknown block requested: %v", height)
					}
				}

				// send the response
				res := &messages.BlockResponse{
					Nonce:  msg.Nonce,
					Blocks: blocks,
				}

				err := syncConduit.Submit(res, originID)
				assert.NoError(t, err)
			default:
				t.Errorf("unexpected msg to sync engine: %T, %v", args[1], args[1])
			}
		}).Return(nil)

	receiptsReceived := 0

	// register consensus engine to track receipts
	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ReceiveReceipts, consensusEngine)
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

	// submit block2 from consensus node to execution node
	exeNode.IngestionEngine.Submit(conID.NodeID, proposal2)

	// ensure block 1 has been executed
	hub.DeliverAllEventuallyUntil(t, func() bool {
		return receiptsReceived == 2
	}, 30*time.Second, 500*time.Millisecond)

	// ensure blocks have been executed
	exeNode.AssertHighestExecutedBlock(t, block2.Header)

	scExeGenesis, err := exeNode.ExecutionState.StateCommitmentByBlockID(context.Background(), genesis.ID())
	assert.NoError(t, err)
	scExeBlock2, err := exeNode.ExecutionState.StateCommitmentByBlockID(context.Background(), block2.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExeGenesis, scExeBlock2)

	syncEngine.AssertExpectations(t)
}

func TestBroadcastToMultipleVerificationNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	ver2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, exeID, ver1ID, ver2ID)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
	defer exeNode.Done()

	verification1Node := testutil.GenericNode(t, hub, ver1ID, identities, chainID)
	defer verification1Node.Done()
	verification2Node := testutil.GenericNode(t, hub, ver2ID, identities, chainID)
	defer verification2Node.Done()

	genesis, err := exeNode.State.AtHeight(0).Head()
	require.NoError(t, err)

	block := unittest.BlockWithParentFixture(genesis)
	block.Header.View = 42
	block.SetPayload(flow.Payload{})
	proposal := unittest.ProposalFromBlock(&block)

	actualCalls := 0

	var receipt *flow.ExecutionReceipt

	verificationEngine := new(network.Engine)
	_, _ = verification1Node.Net.Register(engine.ReceiveReceipts, verificationEngine)
	_, _ = verification2Node.Net.Register(engine.ReceiveReceipts, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			actualCalls++
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil)

	exeNode.IngestionEngine.SubmitLocal(proposal)

	hub.DeliverAllEventually(t, func() bool {
		return actualCalls == 2
	})

	verificationEngine.AssertExpectations(t)
}

func TestReceiveTheSameDeltaMultipleTimes(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	ver2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, exeID, ver1ID, ver2ID)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
	defer exeNode.Done()

	genesis, err := exeNode.State.AtHeight(0).Head()
	require.NoError(t, err)

	delta := unittest.StateDeltaWithParentFixture(genesis)
	delta.ExecutableBlock.StartState = unittest.GenesisStateCommitment
	delta.EndState = unittest.GenesisStateCommitment

	fmt.Printf("block id: %v, delta for block (%v)'s parent id: %v\n", genesis.ID(), delta.Block.ID(), delta.ParentID())
	exeNode.IngestionEngine.SubmitLocal(delta)
	time.Sleep(time.Second)

	exeNode.IngestionEngine.SubmitLocal(delta)
	// handling the same delta again to verify the DB calls in saveExecutionResults
	// are idempotent, if they weren't, it will hit log.Fatal and crash before
	// sleep is done
	time.Sleep(time.Second)

}
