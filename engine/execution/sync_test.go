package execution_test

import (
	"fmt"
	"sync"
	"testing"

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

func TestSyncFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := unittest.CompleteIdentitySet(colID, conID, exe1ID, exe2ID, verID)

	// exe1 will sync from exe2 after exe2 executes transactions
	exeNode1 := testutil.ExecutionNode(t, hub, exe1ID, identities, 1)
	defer exeNode1.Done()
	exeNode2 := testutil.ExecutionNode(t, hub, exe2ID, identities, 21)
	defer exeNode2.Done()

	collectionNode := testutil.GenericNode(t, hub, colID, identities)
	defer collectionNode.Done()
	verificationNode := testutil.GenericNode(t, hub, verID, identities)
	defer verificationNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities)
	defer consensusNode.Done()

	genesis, err := exeNode1.State.AtHeight(0).Head()
	require.NoError(t, err)

	seq := uint64(0)

	tx1 := execTestutil.DeployCounterContractTransaction()
	err = execTestutil.SignTransactionByRoot(&tx1, seq)
	require.NoError(t, err)
	seq++

	tx2 := execTestutil.CreateCounterTransaction()
	err = execTestutil.SignTransactionByRoot(&tx2, seq)
	require.NoError(t, err)
	seq++

	tx4 := execTestutil.AddToCounterTransaction()
	err = execTestutil.SignTransactionByRoot(&tx4, seq)
	require.NoError(t, err)

	col1 := flow.Collection{Transactions: []*flow.TransactionBody{&tx1}}
	col2 := flow.Collection{Transactions: []*flow.TransactionBody{&tx2}}
	col4 := flow.Collection{Transactions: []*flow.TransactionBody{&tx4}}

	//Create three blocks, with one tx each
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

	proposal1 := unittest.ProposalFromBlock(&block1)
	proposal2 := unittest.ProposalFromBlock(&block2)
	proposal3 := unittest.ProposalFromBlock(&block3)
	proposal4 := unittest.ProposalFromBlock(&block4)

	fmt.Printf("block0 ID %x parent %x\n", genesis.ID(), genesis.ParentID)
	fmt.Printf("block1 ID %x parent %x\n", block1.ID(), block1.Header.ParentID)
	fmt.Printf("block2 ID %x parent %x\n", block2.ID(), block2.Header.ParentID)
	fmt.Printf("block3 ID %x parent %x\n", block3.ID(), block3.Header.ParentID)
	fmt.Printf("block4 ID %x parent %x\n", block4.ID(), block4.Header.ParentID)

	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.CollectionProvider, collectionEngine)

	collectionEngine.On("Submit", exe2ID.NodeID, mock.MatchedBy(func(r *messages.CollectionRequest) bool { return r.ID == col1.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.CollectionResponse{Collection: col1}, exe2ID.NodeID)
	}).Return(nil)
	collectionEngine.On("Submit", exe2ID.NodeID, mock.MatchedBy(func(r *messages.CollectionRequest) bool { return r.ID == col2.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.CollectionResponse{Collection: col2}, exe2ID.NodeID)
	}).Return(nil)
	collectionEngine.On("Submit", exe1ID.NodeID, mock.MatchedBy(func(r *messages.CollectionRequest) bool { return r.ID == col4.ID() })).Run(func(args mock.Arguments) {
		_ = colConduit.Submit(&messages.CollectionResponse{Collection: col4}, exe1ID.NodeID)
	}).Return(nil)

	var receipt *flow.ExecutionReceipt

	executedBlocks := map[flow.Identifier]*flow.Identifier{}
	ebMutex := sync.RWMutex{}

	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	verificationEngine.On("Submit", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			ebMutex.Lock()
			defer ebMutex.Unlock()
			identifier, _ := args[0].(flow.Identifier)
			receipt, _ = args[1].(*flow.ExecutionReceipt)
			executedBlocks[receipt.ExecutionResult.BlockID] = &identifier
			fmt.Printf("verified %x\n", receipt.ExecutionResult.BlockID)
		}).
		Return(nil).Times(0)

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	consensusEngine.On("Submit", mock.Anything, mock.Anything).Return(nil).Times(0)

	// submit block from consensus node
	exeNode2.IngestionEngine.Submit(conID.NodeID, proposal1)
	exeNode2.IngestionEngine.Submit(conID.NodeID, proposal2)

	// wait for block2 to be executed on execNode2
	hub.Eventually(t, func() bool {
		ebMutex.RLock()
		defer ebMutex.RUnlock()

		if nodeId, ok := executedBlocks[block2.ID()]; ok {
			return *nodeId == exe2ID.NodeID
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
	hub.Eventually(t, func() bool {
		ebMutex.RLock()
		defer ebMutex.RUnlock()

		block3ok := false
		block4ok := false
		if nodeId, ok := executedBlocks[block3.ID()]; ok {
			block3ok = *nodeId == exe1ID.NodeID
		}
		if nodeId, ok := executedBlocks[block4.ID()]; ok {
			block4ok = *nodeId == exe1ID.NodeID
		}
		return block3ok && block4ok
	})

	collectionEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
}
