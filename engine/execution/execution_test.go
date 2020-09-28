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

func sendBlock(exeNode *testmock.ExecutionNode, from flow.Identifier, proposal *messages.BlockProposal) error {
	return exeNode.FollowerEngine.Process(from, proposal)
}

// Test when the ingestion engine receives a block, it will
// request collections from collection node, and send ER to
// verification node and consensus node.
// create a block that has two collections: col1 and col2;
// col1 has tx1 and tx2, col2 has tx3 and tx4.
func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Testnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, conID, exeID, verID)

	// create execution node
	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
	exeNode.Ready()
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

	block := unittest.BlockWithParentAndProposerFixture(genesis, conID.NodeID)
	block.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{
				CollectionID:     col1.ID(),
				SignerIDs:        []flow.Identifier{colID.NodeID},
				ReferenceBlockID: genesis.ID(),
			},
			{
				CollectionID:     col2.ID(),
				SignerIDs:        []flow.Identifier{colID.NodeID},
				ReferenceBlockID: genesis.ID(),
			},
		},
	})

	collectionNode := testutil.GenericNode(t, hub, colID, identities, chainID)
	defer collectionNode.Done()
	verificationNode := testutil.GenericNode(t, hub, verID, identities, chainID)
	defer verificationNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()

	// create collection node that can respond collections to execution node
	// check collection node received the collection request from execution node
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

	// create verification engine that can create approvals and send to consensus nodes
	// check the verification engine received the ER from execution node
	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ReceiveReceipts, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	// create consensus engine that accepts the result
	// check the consensus engine has received the result from execution node
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
	err = sendBlock(&exeNode, conID.NodeID, unittest.ProposalFromBlock(&block))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		// when sendBlock returned, ingestion engine might not have processed
		// the block yet, because the process is async. we have to wait
		hub.DeliverAll()
		return receipt != nil
	}, time.Second*10, time.Millisecond*500)

	// check that the block has been executed.
	exeNode.AssertHighestExecutedBlock(t, block.Header)

	providerEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)
}

// Test the following behaviors:
// (1) ENs sync statecommitment with each other
// (2) a failed transaction will not change statecommitment
//
// We prepare 3 transactions in 3 blocks:
// tx1 will deploy a contract
// tx2 will always panic
// tx3 will be succeed and change statecommitment
// and then create 2 EN nodes, both have tx1 executed. To test the synchronisation,
// we send tx2 and tx3 in 2 blocks to only EN1, and check that tx2 will not change statecommitment for
// verifying behavior (1);
// and check EN2 should have the same statecommitment as EN1 since they sync
// with each other for verifying behavior (2).
// TODO: state sync is disabled, we are only verifying 2) for now.
func TestExecutionStateSyncMultipleExecutionNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	// exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := unittest.CompleteIdentitySet(colID, conID, exe1ID)

	collectionNode := testutil.GenericNode(t, hub, colID, identities, chainID)
	defer collectionNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()
	exe1Node := testutil.ExecutionNode(t, hub, exe1ID, identities, 27, chainID)
	exe1Node.Ready()
	defer exe1Node.Done()

	deployContractBlock := func(t *testing.T, chain flow.Chain, seq uint64, parent *flow.Header, ref *flow.Header) (
		*flow.TransactionBody, *flow.Collection, flow.Block, *messages.BlockProposal, uint64) {
		// make tx
		tx := execTestutil.DeployCounterContractTransaction(chain.ServiceAddress(), chain)
		err := execTestutil.SignTransactionAsServiceAccount(tx, seq, chain)
		require.NoError(t, err)

		// make collection
		col := &flow.Collection{Transactions: []*flow.TransactionBody{tx}}

		// make block
		block := unittest.BlockWithParentAndProposerFixture(parent, conID.NodeID)
		block.SetPayload(flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					CollectionID:     col.ID(),
					SignerIDs:        []flow.Identifier{colID.NodeID},
					ReferenceBlockID: ref.ID(),
				},
			},
		})

		// make proposal
		proposal := unittest.ProposalFromBlock(&block)

		return tx, col, block, proposal, seq + 1
	}

	makePanicBlock := func(t *testing.T, chain flow.Chain, seq uint64, parent *flow.Header, ref *flow.Header) (
		*flow.TransactionBody, *flow.Collection, flow.Block, *messages.BlockProposal, uint64) {
		// make tx
		tx := execTestutil.CreateCounterPanicTransaction(chain.ServiceAddress(), chain.ServiceAddress())
		err := execTestutil.SignTransactionAsServiceAccount(tx, seq, chain)
		require.NoError(t, err)

		// make collection
		col := &flow.Collection{Transactions: []*flow.TransactionBody{tx}}

		// make block
		block := unittest.BlockWithParentAndProposerFixture(parent, conID.NodeID)
		block.SetPayload(flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{CollectionID: col.ID(), SignerIDs: []flow.Identifier{colID.NodeID}, ReferenceBlockID: ref.ID()},
			},
		})

		proposal := unittest.ProposalFromBlock(&block)

		return tx, col, block, proposal, seq + 1
	}

	makeSuccessBlock := func(t *testing.T, chain flow.Chain, seq uint64, parent *flow.Header, ref *flow.Header) (
		*flow.TransactionBody, *flow.Collection, flow.Block, *messages.BlockProposal, uint64) {
		tx := execTestutil.AddToCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
		err := execTestutil.SignTransactionAsServiceAccount(tx, seq, chain)
		require.NoError(t, err)

		col := &flow.Collection{Transactions: []*flow.TransactionBody{tx}}
		block := unittest.BlockWithParentAndProposerFixture(parent, conID.NodeID)
		block.SetPayload(flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{CollectionID: col.ID(), SignerIDs: []flow.Identifier{colID.NodeID}, ReferenceBlockID: ref.ID()},
			},
		})

		proposal := unittest.ProposalFromBlock(&block)

		return tx, col, block, proposal, seq + 1
	}

	genesis, err := exe1Node.State.AtHeight(0).Head()
	require.NoError(t, err)

	seq := uint64(0)

	chain := exe1Node.ChainID.Chain()

	// transaction that will change state and succeed, used to test that state commitment changes
	// genesis <- block1 [tx1] <- block2 [tx2] <- block3 [tx3]
	_, col1, block1, proposal1, seq := deployContractBlock(t, chain, seq, genesis, genesis)

	_, col2, block2, proposal2, seq := makePanicBlock(t, chain, seq, block1.Header, genesis)

	_, col3, block3, proposal3, _ := makeSuccessBlock(t, chain, seq, block2.Header, genesis)
	// seq++

	// setup mocks and assertions
	collectionEngine := mockCollectionEngineToReturnCollections(
		t,
		&collectionNode,
		[]*flow.Collection{col1, col2, col3},
	)

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
	err = sendBlock(&exe1Node, conID.NodeID, proposal1)
	require.NoError(t, err)

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
	// exe2Node := testutil.ExecutionNode(t, hub, exe2ID, identities, 0, chainID)
	// exe2Node.Ready()
	// defer exe2Node.Done()
	// exe2Node.AssertHighestExecutedBlock(t, genesis)

	// submit block2 and block 3 from consensus node to execution node 1 (who have block1),
	err = sendBlock(&exe1Node, conID.NodeID, proposal2)
	assert.NoError(t, err)

	err = sendBlock(&exe1Node, conID.NodeID, proposal3)
	assert.NoError(t, err)

	// ensure block 1, 2 and 3 have been executed
	hub.DeliverAllEventually(t, func() bool {
		return receiptsReceived == 3
	})

	// ensure state has been synced across both nodes
	exe1Node.AssertHighestExecutedBlock(t, block3.Header)
	// exe2Node.AssertHighestExecutedBlock(t, block3.Header)

	// verify state commitment of block 2 is the same as block 1, since tx failed
	scExe1Block2, err := exe1Node.ExecutionState.StateCommitmentByBlockID(context.Background(), block2.ID())
	assert.NoError(t, err)
	assert.Equal(t, scExe1Block1, scExe1Block2)

	collectionEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)
}

func mockCollectionEngineToReturnCollections(t *testing.T, collectionNode *testmock.GenericNode, cols []*flow.Collection) *network.Engine {
	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.RequestCollections, collectionEngine)

	// make lookup
	colMap := make(map[flow.Identifier][]byte)
	for _, col := range cols {
		blob, _ := msgpack.Marshal(col)
		colMap[col.ID()] = blob
	}
	collectionEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			originID := args[0].(flow.Identifier)
			req := args[1].(*messages.EntityRequest)
			blob, ok := colMap[req.EntityIDs[0]]
			if !ok {
				assert.FailNow(t, "requesting unexpected collection", req.EntityIDs[0])
			}
			res := &messages.EntityResponse{Blobs: [][]byte{blob}, EntityIDs: req.EntityIDs[:1]}
			err := colConduit.Submit(res, originID)
			assert.NoError(t, err)
		}).
		Return(nil).
		Times(len(cols))
	return collectionEngine
}

// Test that when ingestion engine receives a block whose parent is missing,
// it will request the missing parent from consensus node.
// We prepare 3 blocks: genesis <- block1 <- block2
// We mock a consensus node that stores the 3 blocks and mock the consensus node's
// sync engine, which will respond to missing block requests. And then send
// block2 to the ingestion engine, and verify it should fetch the missing block1
// from the consensus node and eventually have both block1 and block2 executed.
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

	block1 := unittest.BlockWithParentAndProposerFixture(genesis, conID.NodeID)
	block1.SetPayload(flow.Payload{})

	block2 := unittest.BlockWithParentAndProposerFixture(block1.Header, conID.NodeID)
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
	err = sendBlock(&exeNode, conID.NodeID, proposal2)
	require.NoError(t, err)

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

// Test the receipt will be sent to multiple verification nodes
func TestBroadcastToMultipleVerificationNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	ver2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, conID, exeID, ver1ID, ver2ID)

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
	exeNode.Ready()
	defer exeNode.Done()

	verification1Node := testutil.GenericNode(t, hub, ver1ID, identities, chainID)
	defer verification1Node.Done()
	verification2Node := testutil.GenericNode(t, hub, ver2ID, identities, chainID)
	defer verification2Node.Done()

	genesis, err := exeNode.State.AtHeight(0).Head()
	require.NoError(t, err)

	block := unittest.BlockWithParentAndProposerFixture(genesis, conID.NodeID)
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

	err = sendBlock(&exeNode, exeID.NodeID, proposal)
	require.NoError(t, err)

	hub.DeliverAllEventually(t, func() bool {
		return actualCalls == 2
	})

	verificationEngine.AssertExpectations(t)
}

// Test that when received the same state delta for the second time,
// the delta will be saved again without causing any error.
// func TestReceiveTheSameDeltaMultipleTimes(t *testing.T) {
// 	hub := stub.NewNetworkHub()
//
// 	chainID := flow.Mainnet
//
// 	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
// 	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
// 	ver1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
// 	ver2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
//
// 	identities := unittest.CompleteIdentitySet(colID, exeID, ver1ID, ver2ID)
//
// 	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21, chainID)
// 	defer exeNode.Done()
//
// 	genesis, err := exeNode.State.AtHeight(0).Head()
// 	require.NoError(t, err)
//
// 	delta := unittest.StateDeltaWithParentFixture(genesis)
// 	delta.ExecutableBlock.StartState = unittest.GenesisStateCommitment
// 	delta.EndState = unittest.GenesisStateCommitment
//
// 	fmt.Printf("block id: %v, delta for block (%v)'s parent id: %v\n", genesis.ID(), delta.Block.ID(), delta.ParentID())
// 	exeNode.IngestionEngine.SubmitLocal(delta)
// 	time.Sleep(time.Second)
//
// 	exeNode.IngestionEngine.SubmitLocal(delta)
// 	// handling the same delta again to verify the DB calls in saveExecutionResults
// 	// are idempotent, if they weren't, it will hit log.Fatal and crash before
// 	// sleep is done
// 	time.Sleep(time.Second)
//
// }
