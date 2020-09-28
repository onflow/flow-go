package execution_test

import (
	"fmt"
	"sync"
	"testing"

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

func TestSyncFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, conID, exe1ID, exe2ID, verID)

	// exe1 will sync from exe2 after exe2 executes transactions
	exeNode1 := testutil.ExecutionNode(t, hub, exe1ID, identities, 1, chainID)
	defer exeNode1.Done()
	exeNode2 := testutil.ExecutionNode(t, hub, exe2ID, identities, 21, chainID)
	defer exeNode2.Done()

	collectionNode := testutil.GenericNode(t, hub, colID, identities, chainID)
	defer collectionNode.Done()
	verificationNode := testutil.GenericNode(t, hub, verID, identities, chainID)
	defer verificationNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()

	genesis, err := exeNode1.State.AtHeight(0).Head()
	require.NoError(t, err)

	seq := uint64(0)

	chain := exeNode1.ChainID.Chain()

	tx1 := execTestutil.DeployCounterContractTransaction(chain.ServiceAddress(), chain)
	err = execTestutil.SignTransactionAsServiceAccount(tx1, seq, chain)
	require.NoError(t, err)
	seq++

	tx2 := execTestutil.CreateCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = execTestutil.SignTransactionAsServiceAccount(tx2, seq, chain)
	require.NoError(t, err)
	seq++

	tx4 := execTestutil.AddToCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = execTestutil.SignTransactionAsServiceAccount(tx4, seq, chain)
	require.NoError(t, err)
	seq++

	tx5 := execTestutil.AddToCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = execTestutil.SignTransactionAsServiceAccount(tx5, seq, chain)
	require.NoError(t, err)

	col1 := &flow.Collection{Transactions: []*flow.TransactionBody{tx1}}
	col2 := &flow.Collection{Transactions: []*flow.TransactionBody{tx2}}
	col4 := &flow.Collection{Transactions: []*flow.TransactionBody{tx4}}
	col5 := &flow.Collection{Transactions: []*flow.TransactionBody{tx5}}

	blob1, _ := msgpack.Marshal(col1)
	blob2, _ := msgpack.Marshal(col2)
	blob4, _ := msgpack.Marshal(col4)
	blob5, _ := msgpack.Marshal(col5)

	// Create three blocks, with one tx each
	block1 := unittest.BlockWithParentFixture(genesis)
	block1.Header.View = 42
	block1.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{
				CollectionID: col1.ID(),
				SignerIDs:    []flow.Identifier{colID.NodeID},
			},
		},
	})

	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.Header.View = 44
	block2.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{
				CollectionID: col2.ID(),
				SignerIDs:    []flow.Identifier{colID.NodeID},
			},
		},
	})

	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.Header.View = 45
	block3.SetPayload(flow.Payload{})

	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.Header.View = 46
	block4.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{
				CollectionID: col4.ID(),
				SignerIDs:    []flow.Identifier{colID.NodeID},
			},
		},
	})

	block5 := unittest.BlockWithParentFixture(block4.Header)
	block5.Header.View = 47
	block5.SetPayload(flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{
			{
				CollectionID: col5.ID(),
				SignerIDs:    []flow.Identifier{colID.NodeID},
			},
		},
	})

	proposal1 := unittest.ProposalFromBlock(&block1)
	proposal2 := unittest.ProposalFromBlock(&block2)
	proposal3 := unittest.ProposalFromBlock(&block3)
	proposal4 := unittest.ProposalFromBlock(&block4)
	proposal5 := unittest.ProposalFromBlock(&block5)

	fmt.Printf("block0 ID %x parent %x\n", genesis.ID(), genesis.ParentID)
	fmt.Printf("block1 ID %x parent %x\n", block1.ID(), block1.Header.ParentID)
	fmt.Printf("block2 ID %x parent %x\n", block2.ID(), block2.Header.ParentID)
	fmt.Printf("block3 ID %x parent %x\n", block3.ID(), block3.Header.ParentID)
	fmt.Printf("block4 ID %x parent %x\n", block4.ID(), block4.Header.ParentID)
	fmt.Printf("block5 ID %x parent %x\n", block5.ID(), block5.Header.ParentID)

	providerEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.ProvideCollections, providerEngine)

	providerEngine.On("Submit", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			originID := args.Get(0).(flow.Identifier)
			req := args.Get(1).(*messages.EntityRequest)
			if originID == exe2ID.NodeID && req.EntityIDs[0] == col1.ID() {
				res := &messages.EntityResponse{Nonce: req.Nonce, Blobs: [][]byte{blob1}, EntityIDs: req.EntityIDs[:1]}
				_ = colConduit.Submit(res, originID)
				return
			}
			if originID == exe2ID.NodeID && req.EntityIDs[0] == col2.ID() {
				res := &messages.EntityResponse{Nonce: req.Nonce, Blobs: [][]byte{blob2}, EntityIDs: req.EntityIDs[:1]}
				_ = colConduit.Submit(res, originID)
				return
			}
			if originID == exe1ID.NodeID && req.EntityIDs[0] == col4.ID() {
				res := &messages.EntityResponse{Nonce: req.Nonce, Blobs: [][]byte{blob4}, EntityIDs: req.EntityIDs[:1]}
				_ = colConduit.Submit(res, originID)
				return
			}
			if originID == exe1ID.NodeID && req.EntityIDs[0] == col5.ID() {
				res := &messages.EntityResponse{Nonce: req.Nonce, Blobs: [][]byte{blob5}, EntityIDs: req.EntityIDs[:1]}
				_ = colConduit.Submit(res, originID)
				return
			}
			assert.FailNowf(t, "invalid collection request", "(origin: %x, entities: %s)", originID, req.EntityIDs)
		},
	).
		Times(4).
		Return(nil)
	providerEngine.On("Submit", exe2ID.NodeID, mock.MatchedBy(func(r *messages.EntityRequest) bool { return r.EntityIDs[0] == col1.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.EntityResponse{Blobs: [][]byte{blob1}}, exe2ID.NodeID)
	}).Return(nil)
	providerEngine.On("Submit", exe2ID.NodeID, mock.MatchedBy(func(r *messages.EntityRequest) bool { return r.EntityIDs[0] == col2.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.EntityResponse{Blobs: [][]byte{blob2}}, exe2ID.NodeID)
	}).Return(nil)
	providerEngine.On("Submit", exe1ID.NodeID, mock.MatchedBy(func(r *messages.EntityRequest) bool { return r.EntityIDs[0] == col4.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.EntityResponse{Blobs: [][]byte{blob4}}, exe2ID.NodeID)
	}).Return(nil)
	providerEngine.On("Submit", exe1ID.NodeID, mock.MatchedBy(func(r *messages.EntityRequest) bool { return r.EntityIDs[0] == col5.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.EntityResponse{Blobs: [][]byte{blob5}}, exe2ID.NodeID)
	}).Return(nil)

	var receipt *flow.ExecutionReceipt

	executedBlocks := make(map[flow.Identifier]flow.Identifier)
	ebMutex := sync.RWMutex{}

	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ReceiveReceipts, verificationEngine)
	verificationEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			ebMutex.Lock()
			defer ebMutex.Unlock()
			originID, _ := args[0].(flow.Identifier)
			receipt, _ = args[1].(*flow.ExecutionReceipt)
			executedBlocks[receipt.ExecutionResult.BlockID] = originID
			fmt.Printf("verified %x\n", receipt.ExecutionResult.BlockID)
		}).
		Return(nil).Times(0)

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ReceiveReceipts, consensusEngine)
	consensusEngine.On("Submit", mock.Anything, mock.Anything).Return(nil).Times(0)

	// submit block from consensus node
	exeNode2.IngestionEngine.Submit(conID.NodeID, proposal1)
	exeNode2.IngestionEngine.Submit(conID.NodeID, proposal2)

	// wait for block2 to be executed on execNode2
	hub.DeliverAllEventually(t, func() bool {
		ebMutex.RLock()
		defer ebMutex.RUnlock()

		if nodeID, ok := executedBlocks[block2.ID()]; ok {
			return nodeID == exe2ID.NodeID
		}
		return false
	})

	// make sure exe1 didn't get any blocks
	exeNode1.AssertHighestExecutedBlock(t, genesis)
	exeNode2.AssertHighestExecutedBlock(t, block2.Header)

	// submit block3 and block4 to exe1 which should trigger sync
	exeNode1.IngestionEngine.Submit(conID.NodeID, proposal3)
	exeNode1.IngestionEngine.Submit(conID.NodeID, proposal4)

	// wait for block3/4 to be executed on execNode1
	hub.DeliverAllEventually(t, func() bool {
		ebMutex.RLock()
		defer ebMutex.RUnlock()

		block3ok := false
		block4ok := false
		if nodeID, ok := executedBlocks[block3.ID()]; ok {
			block3ok = nodeID == exe1ID.NodeID
		}
		if nodeID, ok := executedBlocks[block4.ID()]; ok {
			block4ok = nodeID == exe1ID.NodeID
		}
		return block3ok && block4ok
	})

	// make sure collections are saved and synced as well
	rCol1, err := exeNode1.Collections.ByID(col1.ID())
	require.NoError(t, err)
	assert.Equal(t, col1, rCol1)

	rCol2, err := exeNode1.Collections.ByID(col2.ID())
	require.NoError(t, err)
	assert.Equal(t, col2, rCol2)

	rCol4, err := exeNode1.Collections.ByID(col4.ID())
	require.NoError(t, err)
	assert.Equal(t, col4, rCol4)

	rCol1, err = exeNode2.Collections.ByID(col1.ID())
	require.NoError(t, err)
	assert.Equal(t, col1, rCol1)

	rCol2, err = exeNode2.Collections.ByID(col2.ID())
	require.NoError(t, err)
	assert.Equal(t, col2, rCol2)

	// node two didn't get block4
	_, err = exeNode2.Collections.ByID(col4.ID())
	require.Error(t, err)

	exeNode1.AssertHighestExecutedBlock(t, block4.Header)

	// submit block5, to make sure we're still processing any incoming blocks after sync is complete
	exeNode1.IngestionEngine.Submit(conID.NodeID, proposal5)
	hub.DeliverAllEventually(t, func() bool {
		ebMutex.RLock()
		defer ebMutex.RUnlock()

		if nodeID, ok := executedBlocks[block5.ID()]; ok {
			return nodeID == exe1ID.NodeID
		}
		return false
	})

	exeNode1.AssertHighestExecutedBlock(t, block5.Header)

	providerEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
}
