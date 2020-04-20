package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const nextBlockTimeout = 60 * time.Second

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionSuite))
}

type ExecutionSuite struct {
	suite.Suite
	cancel       context.CancelFunc
	net          *testnet.FlowNetwork
	nodeIDs      []flow.Identifier
	ghostID      flow.Identifier
	exe1ID       flow.Identifier
	exe2ID       flow.Identifier
	blockState   BlockState
	receiptState ReceiptState
}

func (es *ExecutionSuite) Start(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	es.cancel = cancel
	es.net.Start(ctx)
}

func (es *ExecutionSuite) Stop() {
	err := es.net.Stop()
	require.NoError(es.T(), err, "should stop without error")
	es.cancel()
	err = es.net.Cleanup()
	require.NoError(es.T(), err, "should cleanup without error")
}

func (gs *ExecutionSuite) Node(id flow.Identifier) *testnet.Container {
	node, found := gs.net.ContainerByID(id)
	require.True(gs.T(), found, "could not find node")
	return node
}

func (gs *ExecutionSuite) Ghost() *client.GhostClient {
	ghost, found := gs.net.ContainerByID(gs.ghostID)
	require.True(gs.T(), found, "could not find ghost containter")
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *ExecutionSuite) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need two execution nodes
	gs.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe1ID),
		testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, exe1Config)
	gs.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe2ID),
		testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, exe2Config)

	// need one verification node
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.WithID(gs.ghostID), testnet.AsGhost(true),
		testnet.WithLogLevel(zerolog.DebugLevel))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("execution_tests", nodeConfigs)

	// initialize the network
	gs.net = testnet.PrepareFlowNetwork(gs.T(), netConfig)
}

func (gs *ExecutionSuite) TestStateSyncAfterNetworkPartition() {
	// start the network and defer cleanup
	gs.Start(time.Minute)
	defer gs.Stop()

	// start tracking blocks
	go gs.trackBlocksAndReceipts()

	// pause execution node 2
	err := gs.Node(gs.exe2ID).Pause()
	require.NoError(gs.T(), err)

	// wait for first finalized block, called blockA
	blockA := gs.blockState.WaitForFirstFinalized(gs.T())
	fmt.Println("got blockA", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.receiptState.WaitFor(gs.T(), blockA.Header.ID(), gs.exe1ID)
	fmt.Println("got erExe1BlockA", erExe1BlockA)

	// TODO send transaction

	// wait for finalized child of blockA, called blockB
	blockB := gs.blockState.WaitForFinalizedChild(gs.T(), blockA)
	fmt.Println("got blockB", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockB := gs.receiptState.WaitFor(gs.T(), blockB.Header.ID(), gs.exe1ID)
	fmt.Println("got erExe1BlockB", erExe1BlockB)

	// assert that state between blockA and blockB has changed
	require.NotEqual(gs.T(), erExe1BlockA.ExecutionResult.FinalStateCommit,
		erExe1BlockB.ExecutionResult.FinalStateCommit)

	// unpause execution node 2
	err = gs.Node(gs.exe2ID).Start()
	require.NoError(gs.T(), err)

	// wait for finalized child of blockB, called blockC
	blockC := gs.blockState.WaitForFinalizedChild(gs.T(), blockB)
	fmt.Println("got blockC", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erExe1BlockC := gs.receiptState.WaitFor(gs.T(), blockC.Header.ID(), gs.exe1ID)
	fmt.Println("got erExe1BlockC", erExe1BlockC)

	// assert that state between blockB and blockC has not changed
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe1BlockC.ExecutionResult.FinalStateCommit)

	// TODO listen for messages for execution state sync

	// wait for execution receipt for blockA from execution node 2 (this one must have been synced)
	erExe2BlockA := gs.receiptState.WaitFor(gs.T(), blockA.Header.ID(), gs.exe2ID)
	fmt.Println("got erExe2BlockA", erExe2BlockA)

	// assert that state for blockA is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockA.ExecutionResult.FinalStateCommit, erExe2BlockA.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockB from execution node 2 (this one must have been synced)
	erExe2BlockB := gs.receiptState.WaitFor(gs.T(), blockB.Header.ID(), gs.exe2ID)
	fmt.Println("got erExe2BlockB", erExe2BlockB)

	// assert that state for blockB is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe2BlockB.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockC from execution node 2 (this one must have been synced)
	erExe2BlockC := gs.receiptState.WaitFor(gs.T(), blockC.Header.ID(), gs.exe2ID)
	fmt.Println("got erExe2BlockC", erExe2BlockC)

	// assert that state for blockC is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockC.ExecutionResult.FinalStateCommit, erExe2BlockC.ExecutionResult.FinalStateCommit)
}

func (gs *ExecutionSuite) trackBlocksAndReceipts() {
	var reader *client.FlowMessageStreamReader

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		var err error
		reader, err = gs.Ghost().Subscribe(context.Background())
		if err == nil {
			break
		}
		if attempts >= 10 {
			require.FailNowf(gs.T(), "could not subscribe to ghost", "%d attempts", attempts)
		}
	}

	for {
		// we read the next message until we reach deadline
		_, msg, err := reader.Next()
		require.NoError(gs.T(), err, "could not read next message")

		fmt.Printf("ghost got message: %T %#v\n", msg, msg)

		switch m := msg.(type) {
		case *messages.BlockProposal:
			gs.blockState.Add(m)
			gs.T().Logf("block proposal received at height %v: %x", m.Header.Height, m.Header.ID())
		case *flow.ExecutionReceipt:
			gs.receiptState.Add(m)
			gs.T().Logf("execution receipts received for block ID %x by executor ID %x", m.ExecutionResult.BlockID,
				m.ExecutorID)
		default:
			continue
		}
	}
}

type BlockState struct {
	blocksByID        map[flow.Identifier]*messages.BlockProposal
	finalizedByHeight map[uint64]*messages.BlockProposal
	highestFinalized  uint64
}

func (bs *BlockState) Add(b *messages.BlockProposal) {
	if bs.blocksByID == nil {
		bs.blocksByID = make(map[flow.Identifier]*messages.BlockProposal)
	}
	bs.blocksByID[b.Header.ID()] = b

	// add confirmations
	confirmsHeight := b.Header.Height - 3
	if confirmsHeight > bs.highestFinalized {
		if bs.finalizedByHeight == nil {
			bs.finalizedByHeight = make(map[uint64]*messages.BlockProposal)
		}
		for i := confirmsHeight; i > bs.highestFinalized; i-- {
			bs.finalizedByHeight[b.Header.Height] = b
		}
		bs.highestFinalized = confirmsHeight
	}
}

func (bs *BlockState) WaitForFirstFinalized(t *testing.T) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		return bs.highestFinalized > 0
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive first finalized block within %v seconds", nextBlockTimeout))

	return bs.finalizedByHeight[1]
}

func (bs *BlockState) WaitForNextFinalized(t *testing.T) *messages.BlockProposal {
	currentFinalized := bs.highestFinalized
	require.Eventually(t, func() bool {
		return bs.highestFinalized > currentFinalized
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive next finalizedblock within %v seconds", nextBlockTimeout))

	return bs.finalizedByHeight[bs.highestFinalized]
}

func (bs *BlockState) WaitForFinalizedChild(t *testing.T, parentHeight *messages.BlockProposal) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		_, ok := bs.finalizedByHeight[parentHeight.Header.Height+1]
		return ok
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized child block at block height %v within %v seconds",
			parentHeight.Header.Height, nextBlockTimeout))

	return bs.finalizedByHeight[parentHeight.Header.Height+1]
}

type ReceiptState struct {
	// receipts contains execution receipts are indexed by blockID, then by executorID
	receipts map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt
}

func (rs *ReceiptState) Add(er *flow.ExecutionReceipt) {
	if rs.receipts == nil {
		rs.receipts = make(map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt)
	}

	if rs.receipts[er.ExecutionResult.BlockID] == nil {
		rs.receipts[er.ExecutionResult.BlockID] = make(map[flow.Identifier]*flow.ExecutionReceipt)
	}

	rs.receipts[er.ExecutionResult.BlockID][er.ExecutorID] = er
}

func (rs *ReceiptState) WaitFor(t *testing.T, blockID, executorID flow.Identifier) *flow.ExecutionReceipt {
	r, ok := rs.receipts[blockID][executorID]

	if ok {
		return r
	}

	require.Eventually(t, func() bool {
		r, ok = rs.receipts[blockID][executorID]
		return ok
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x and %x within %v seconds", blockID, executorID,
			nextBlockTimeout))

	return r
}
