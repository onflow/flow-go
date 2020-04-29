package execution

import (
	"bytes"
	"context"
	"fmt"
	"strings"
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
	cancel        context.CancelFunc
	net           *testnet.FlowNetwork
	nodeIDs       []flow.Identifier
	ghostID       flow.Identifier
	exe1ID        flow.Identifier
	exe2ID        flow.Identifier
	ghostTracking bool
	blockState    BlockState
	receiptState  ReceiptState
}

func (es *ExecutionSuite) Start(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	es.cancel = cancel
	es.net.Start(ctx)
}

func (gs *ExecutionSuite) Ghost() *client.GhostClient {
	ghost := gs.net.ContainerByID(gs.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *ExecutionSuite) AccessClient() *testnet.Client {
	client, err := testnet.NewClient(fmt.Sprintf(":%s", gs.net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(gs.T(), err, "could not get access client")
	return client
}

func (gs *ExecutionSuite) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// need one access node
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	nodeConfigs = append(nodeConfigs, acsConfig)

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.InfoLevel))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need two execution nodes
	gs.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, exe1Config)
	gs.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe2ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, exe2Config)

	// need one verification node
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.WithID(gs.ghostID), testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("execution_tests", nodeConfigs)

	// initialize the network
	gs.net = testnet.PrepareFlowNetwork(gs.T(), netConfig)
}

func (gs *ExecutionSuite) TestStateSyncAfterNetworkPartition() {
	// start the network and defer cleanup
	gs.Start(time.Minute)
	defer gs.net.Remove()

	// start tracking blocks
	go gs.trackBlocksAndReceipts()
	defer gs.stopBlocksAndReceiptsTracking()

	// pause execution node 2
	err := gs.net.ContainerByID(gs.exe2ID).Pause()
	require.NoError(gs.T(), err, "could not pause execution node 2")

	// wait for first finalized block, called blockA
	blockA := gs.blockState.WaitForFirstFinalized(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.receiptState.WaitForAtFrom(gs.T(), blockA.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockA with SC %x", erExe1BlockA.ExecutionResult.FinalStateCommit)

	// send transaction
	err = common.DeployCounter(context.Background(), gs.AccessClient())

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := WaitUntilFinalizedStateCommitmentChanged(gs.T(), &gs.blockState, &gs.receiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := gs.receiptState.WaitForAtFrom(gs.T(), blockB.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockB with SC %x", erExe1BlockB.ExecutionResult.FinalStateCommit)

	// require that state between blockA and blockB has changed
	require.NotEqual(gs.T(), erExe1BlockA.ExecutionResult.FinalStateCommit,
		erExe1BlockB.ExecutionResult.FinalStateCommit)

	// unpause execution node 2
	err = gs.net.ContainerByID(gs.exe2ID).Start()
	require.NoError(gs.T(), err)

	// wait until the next proposed block is finalized, called blockC
	blockC := gs.blockState.WaitUntilNextHeightFinalized(gs.T())
	gs.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erExe1BlockC := gs.receiptState.WaitForAtFrom(gs.T(), blockC.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockC with SC %x", erExe1BlockC.ExecutionResult.FinalStateCommit)

	// require that state between blockB and blockC has not changed
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe1BlockC.ExecutionResult.FinalStateCommit)

	// // TODO listen for messages for execution state sync

	// wait for execution receipt for blockA from execution node 2 (this one must have been synced)
	erExe2BlockA := gs.receiptState.WaitForAtFrom(gs.T(), blockA.Header.ID(), gs.exe2ID)
	gs.T().Logf("got erExe2BlockA with SC %x", erExe2BlockA.ExecutionResult.FinalStateCommit)

	// require that state for blockA is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockA.ExecutionResult.FinalStateCommit, erExe2BlockA.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockB from execution node 2 (this one must have been synced)
	erExe2BlockB := gs.receiptState.WaitForAtFrom(gs.T(), blockB.Header.ID(), gs.exe2ID)
	gs.T().Logf("got erExe2BlockB with SC %x", erExe2BlockB.ExecutionResult.FinalStateCommit)

	// require that state for blockB is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe2BlockB.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockC from execution node 2 (this one must have been synced)
	erExe2BlockC := gs.receiptState.WaitForAtFrom(gs.T(), blockC.Header.ID(), gs.exe2ID)
	gs.T().Logf("got erExe2BlockC with SC %x", erExe2BlockC.ExecutionResult.FinalStateCommit)

	// require that state for blockC is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockC.ExecutionResult.FinalStateCommit, erExe2BlockC.ExecutionResult.FinalStateCommit)
}

func (gs *ExecutionSuite) trackBlocksAndReceipts() {
	var reader *client.FlowMessageStreamReader

	gs.ghostTracking = true

	// subscribe to the ghost
	timeout := time.After(5 * time.Second)
	ticker := time.Tick(100 * time.Millisecond)
	for stay := true; stay && gs.ghostTracking; {
		select {
		case <-timeout:
			require.FailNowf(gs.T(), "could not subscribe to ghost", "%v", timeout)
		case <-ticker:
			if !gs.ghostTracking {
				return
			}

			var err error
			reader, err = gs.Ghost().Subscribe(context.Background())
			if err == nil {
				stay = false
			}
		}
	}

	for {
		if !gs.ghostTracking {
			return
		}

		// we read the next message until we reach deadline
		_, msg, err := reader.Next()

		// don't error if container shuts down
		if err != nil && strings.Contains(err.Error(), "transport is closing") && !gs.ghostTracking {
			return
		}

		// don't allow other errers
		require.NoError(gs.T(), err, "could not read next message")

		switch m := msg.(type) {
		case *messages.BlockProposal:
			gs.blockState.Add(m)
			gs.T().Logf("block proposal received at height %v: %x", m.Header.Height, m.Header.ID())
		case *flow.ExecutionReceipt:
			gs.receiptState.Add(m)
			gs.T().Logf("execution receipts received for block ID %x by executor ID %x with SC %x", m.ExecutionResult.BlockID,
				m.ExecutorID, m.ExecutionResult.FinalStateCommit)
		default:
			continue
		}
	}
}

func (gs *ExecutionSuite) stopBlocksAndReceiptsTracking() {
	gs.ghostTracking = false
}

type BlockState struct {
	blocksByID        map[flow.Identifier]*messages.BlockProposal
	finalizedByHeight map[uint64]*messages.BlockProposal
	highestFinalized  uint64
	highestProposed   uint64
}

func (bs *BlockState) Add(b *messages.BlockProposal) {
	if bs.blocksByID == nil {
		bs.blocksByID = make(map[flow.Identifier]*messages.BlockProposal)
	}
	bs.blocksByID[b.Header.ID()] = b
	bs.highestProposed = b.Header.Height

	// add confirmations
	confirmsHeight := b.Header.Height - 3
	if b.Header.Height >= 3 && confirmsHeight > bs.highestFinalized {
		if bs.finalizedByHeight == nil {
			bs.finalizedByHeight = make(map[uint64]*messages.BlockProposal)
		}
		// put all ancestors into `finalizedByHeight`
		for ancestor, ok := b, true; ancestor.Header.Height > bs.highestFinalized; {
			h := ancestor.Header.Height

			// if ancestor is confirmed put it into the finalized map
			if h <= confirmsHeight {
				bs.finalizedByHeight[h] = ancestor

				// if ancestor confirms a heigher height than highestFinalized, increase highestFinalized
				if h > bs.highestFinalized {
					bs.highestFinalized = h
				}
			}

			// find parent
			ancestor, ok = bs.blocksByID[ancestor.Header.ParentID]

			// stop if parent not found
			if !ok {
				return
			}
		}
	}
}

// WaitForFirstFinalized waits until the first block is finalized
func (bs *BlockState) WaitForFirstFinalized(t *testing.T) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		return bs.highestFinalized > 0
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive first finalized block within %v seconds", nextBlockTimeout))

	return bs.finalizedByHeight[bs.highestFinalized]
}

// WaitForNextFinalized waits until the next block is finalized: If the latest proposed block has height 13, and the
// latest finalized block is 10, this will wait until block height 11 is finalized
func (bs *BlockState) WaitForNextFinalized(t *testing.T) *messages.BlockProposal {
	currentFinalized := bs.highestFinalized
	require.Eventually(t, func() bool {
		return bs.highestFinalized > currentFinalized
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive next finalizedblock (height %v) within %v seconds", currentFinalized+1,
			nextBlockTimeout))

	return bs.finalizedByHeight[currentFinalized+1]
}

// WaitUntilNextHeightFinalized waits until the next block height that will be proposed is finalized: If the latest
// proposed block has height 13, and the latest finalized block is 10, this will wait until block height 14 is finalized
func (bs *BlockState) WaitUntilNextHeightFinalized(t *testing.T) *messages.BlockProposal {
	currentProposed := bs.highestProposed
	require.Eventually(t, func() bool {
		return bs.highestFinalized > currentProposed
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized block for next block height (%v) within %v seconds", currentProposed+1,
			nextBlockTimeout))

	return bs.finalizedByHeight[currentProposed+1]
}

// WaitForFinalizedChild waits until any child of the passed parent will be finalized: If the latest proposed block has
// height 13, and the latest finalized block is 10 and we are waiting for the first finalized child of a parent at
// height 11, this will wait until block height 12 is finalized
func (bs *BlockState) WaitForFinalizedChild(t *testing.T, parent *messages.BlockProposal) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		_, ok := bs.finalizedByHeight[parent.Header.Height+1]
		return ok
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized child block for parent block height %v within %v seconds",
			parent.Header.Height, nextBlockTimeout))

	return bs.finalizedByHeight[parent.Header.Height+1]
}

func (bs *BlockState) HighestFinalized() (*messages.BlockProposal, bool) {
	if bs.highestFinalized == 0 {
		return nil, false
	}
	block, ok := bs.finalizedByHeight[bs.highestFinalized]
	return block, ok
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

// WaitForAtFromAny waits for an execution receipt for the given blockID from any execution node and returns it
func (rs *ReceiptState) WaitForAtFromAny(t *testing.T, blockID flow.Identifier) *flow.ExecutionReceipt {
	require.Eventually(t, func() bool {
		return len(rs.receipts[blockID]) > 0
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x from any node within %v seconds", blockID,
			nextBlockTimeout))
	for _, r := range rs.receipts[blockID] {
		return r
	}
	panic("there needs to be an entry in rs.receipts[blockID]")
}

// WaitForAtFrom waits for an execution receipt for the given blockID and the given executorID and returns it
func (rs *ReceiptState) WaitForAtFrom(t *testing.T, blockID, executorID flow.Identifier) *flow.ExecutionReceipt {
	var r *flow.ExecutionReceipt
	require.Eventually(t, func() bool {
		var ok bool
		r, ok = rs.receipts[blockID][executorID]
		return ok
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x from %x within %v seconds", blockID, executorID,
			nextBlockTimeout))
	return r
}

// WaitUntilFinalizedStateCommitmentChanged waits until a different state commitment for a finalized block is received
// compared to the latest one from any execution node and returns the corresponding block and execution receipt
func WaitUntilFinalizedStateCommitmentChanged(t *testing.T, bs *BlockState, rs *ReceiptState) (*messages.BlockProposal, *flow.ExecutionReceipt) {
	// get the state commitment for the highest finalized block
	initialFinalizedSC := flow.GenesisStateCommitment
	b1, ok := bs.HighestFinalized()
	if ok {
		r1 := rs.WaitForAtFromAny(t, b1.Header.ID())
		initialFinalizedSC = r1.ExecutionResult.FinalStateCommit
	}

	currentHeight := b1.Header.Height + 1
	currentID := b1.Header.ID()
	var b2 *messages.BlockProposal
	var r2 *flow.ExecutionReceipt
	require.Eventually(t, func() bool {
		var ok bool
		b2, ok = bs.finalizedByHeight[currentHeight]
		if !ok {
			return false
		}
		currentID = b2.Header.ID()
		r2 = rs.WaitForAtFromAny(t, b2.Header.ID())
		if bytes.Compare(initialFinalizedSC, r2.ExecutionResult.FinalStateCommit) == 0 {
			// received a new execution result for the next finalized block, but it has the same final state commitment
			// check the next finalized block
			currentHeight++
			return false
		}
		return true
	}, nextBlockTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive an execution receipt with a different state commitment from %x within %v seconds,"+
			" last block checked height %v, last block checked ID %x", initialFinalizedSC, nextBlockTimeout,
			currentHeight, currentID))

	return b2, r2
}
