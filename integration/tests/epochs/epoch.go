package epochs

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// StaticEpochTransitionSuite is the suite used for epoch transition tests
// with a static identity table.
type StaticEpochTransitionSuite struct {
	Suite
}

func (s *StaticEpochTransitionSuite) SetupTest() {
	// use shorter epoch phases as no staking operations need to occur in the
	// staking phase for this test
	s.StakingAuctionLen = 10
	s.DKGPhaseLen = 50
	s.EpochLen = 200

	// run the generic setup, which starts up the network
	s.Suite.SetupTest()
}

// DynamicEpochTransitionSuite  is the suite used for epoch transitions tests
// with a dynamic identity table.
type DynamicEpochTransitionSuite struct {
	Suite
}

func (s *DynamicEpochTransitionSuite) SetupTest() {
	// use a longer staking auction length to accommodate staking operations for
	// joining/leaving nodes
	s.StakingAuctionLen = 200
	s.DKGPhaseLen = 50
	s.EpochLen = 380

	// run the generic setup, which starts up the network
	s.Suite.SetupTest()
}

// TestStaticEpochTransition asserts epoch state transitions over two full epochs
// without any nodes joining or leaving.
func (s *StaticEpochTransitionSuite) TestStaticEpochTransition() {

	// phaseCheck is a utility struct that contains information about the
	// final view of each epoch/phase.
	type phaseCheck struct {
		epoch     uint64
		phase     flow.EpochPhase
		finalView uint64 // the final view of the phase as defined by the EpochSetup
	}

	phaseChecks := []*phaseCheck{}
	// iterate through two epochs and populate a list of phase checks
	for counter := 0; counter < 2; counter++ {

		// wait until the access node reaches the desired epoch
		var epoch protocol.Epoch
		var epochCounter uint64
		for epoch == nil || epochCounter != uint64(counter) {
			snapshot, err := s.client.GetLatestProtocolSnapshot(s.ctx)
			require.NoError(s.T(), err)
			epoch = snapshot.Epochs().Current()
			epochCounter, err = epoch.Counter()
			require.NoError(s.T(), err)
		}

		epochFirstView, err := epoch.FirstView()
		require.NoError(s.T(), err)
		epochDKGPhase1Final, err := epoch.DKGPhase1FinalView()
		require.NoError(s.T(), err)
		epochDKGPhase2Final, err := epoch.DKGPhase2FinalView()
		require.NoError(s.T(), err)
		epochDKGPhase3Final, err := epoch.DKGPhase3FinalView()
		require.NoError(s.T(), err)
		epochFinal, err := epoch.FinalView()
		require.NoError(s.T(), err)

		epochViews := []*phaseCheck{
			{epoch: epochCounter, phase: flow.EpochPhaseStaking, finalView: epochFirstView},
			{epoch: epochCounter, phase: flow.EpochPhaseSetup, finalView: epochDKGPhase1Final},
			{epoch: epochCounter, phase: flow.EpochPhaseSetup, finalView: epochDKGPhase2Final},
			{epoch: epochCounter, phase: flow.EpochPhaseSetup, finalView: epochDKGPhase3Final},
			{epoch: epochCounter, phase: flow.EpochPhaseCommitted, finalView: epochFinal},
		}

		for _, v := range epochViews {
			s.BlockState.WaitForSealedView(s.T(), v.finalView)
		}

		phaseChecks = append(phaseChecks, epochViews...)
	}

	s.net.StopContainers()

	consensusContainers := make([]*testnet.Container, 0)
	for _, c := range s.net.Containers {
		if c.Config.Role == flow.RoleConsensus {
			consensusContainers = append(consensusContainers, c)
		}
	}

	for _, c := range consensusContainers {
		containerState, err := c.OpenState()
		require.NoError(s.T(), err)

		// create a map of [view] => {epoch-counter, phase}
		lookup := map[uint64]struct {
			epochCounter uint64
			phase        flow.EpochPhase
		}{}

		final, err := containerState.Final().Head()
		require.NoError(s.T(), err)

		var h uint64
		for h = 0; h <= final.Height; h++ {
			snapshot := containerState.AtHeight(h)

			head, err := snapshot.Head()
			require.NoError(s.T(), err)

			epoch := snapshot.Epochs().Current()
			currentEpochCounter, err := epoch.Counter()
			require.NoError(s.T(), err)
			currentPhase, err := snapshot.Phase()
			require.NoError(s.T(), err)

			lookup[head.View] = struct {
				epochCounter uint64
				phase        flow.EpochPhase
			}{
				currentEpochCounter,
				currentPhase,
			}
		}

		for _, v := range phaseChecks {
			item := lookup[v.finalView]
			assert.Equal(s.T(), v.epoch, item.epochCounter, "wrong epoch at view %d", v.finalView)
			assert.Equal(s.T(), v.phase, item.phase, "wrong phase at view %d", v.finalView)
		}
	}
}

// The following Epoch join and leave tests will stake a node by submitting all the transactions
// that a node operator would submit, start a new container for that node, and remove
// a container from the network of the same node type. After this orchestration assertions
// specific to that node type are made to ensure the network is healthy.

type EpochJoinAndLeaveANSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveAN should update access nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveANSuite) TestEpochJoinAndLeaveAN() {
	s.runTestEpochJoinAndLeave(flow.RoleAccess, s.assertNetworkHealthyAfterANChange)
}

type EpochJoinAndLeaveVNSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveVN should update verification nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveVNSuite) TestEpochJoinAndLeaveVN() {
	s.runTestEpochJoinAndLeave(flow.RoleVerification, s.assertNetworkHealthyAfterVNChange)
}

type EpochJoinAndLeaveLNSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveLN should update collection nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveLNSuite) TestEpochJoinAndLeaveLN() {
	s.runTestEpochJoinAndLeave(flow.RoleCollection, s.assertNetworkHealthyAfterLNChange)
}

type EpochJoinAndLeaveSNSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveSN should update consensus nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveSNSuite) TestEpochJoinAndLeaveSN() {
	s.runTestEpochJoinAndLeave(flow.RoleConsensus, s.assertNetworkHealthyAfterSNChange)
}

// runTestEpochJoinAndLeave coordinates adding and removing one node with the given
// role during the first epoch, then running the network health validation function
// once the network has successfully transitioned into the second epoch.
//
// This tests:
// * that nodes can stake and join the network at an epoch boundary
// * that nodes can unstake and leave the network at an epoch boundary
// * role-specific network health validation after the swap has completed
func (s *Suite) runTestEpochJoinAndLeave(role flow.Role, checkNetworkHealth nodeUpdateValidation) {

	env := utils.LocalnetEnv()

	var containerToReplace *testnet.Container

	// replace access_2, avoid replacing access_1 the container used for client connections
	if role == flow.RoleAccess {
		containerToReplace = s.net.ContainerByName("access_2")
		require.NotNil(s.T(), containerToReplace)
	} else {
		// grab the first container of this node role type, this is the container we will replace
		containerToReplace = s.getContainerToReplace(role)
		require.NotNil(s.T(), containerToReplace)
	}

	// staking our new node and add get the corresponding container for that node
	info, testContainer := s.StakeNewNode(s.ctx, env, role)

	// use admin transaction to remove node, this simulates a node leaving the network
	s.removeNodeFromProtocol(s.ctx, env, containerToReplace.Config.NodeID)

	// wait for epoch setup phase before we start our container and pause the old container
	s.WaitForPhase(s.ctx, flow.EpochPhaseSetup)

	// get latest snapshot and start new container
	snapshot, err := s.client.GetLatestProtocolSnapshot(s.ctx)
	require.NoError(s.T(), err)
	testContainer.WriteRootSnapshot(snapshot)
	testContainer.Container.Start(s.ctx)

	currentEpochFinalView, err := snapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// wait for 5 views after the start of the next epoch before we pause our container to replace
	s.BlockState.WaitForSealedView(s.T(), currentEpochFinalView+5)

	//make sure container to replace removed from smart contract state
	s.assertNodeNotApprovedOrProposed(s.ctx, env, containerToReplace.Config.NodeID)

	// assert transition to second epoch happened as expected
	// if counter is still 0, epoch emergency fallback was triggered and we can fail early
	s.assertEpochCounter(s.ctx, 1)

	err = containerToReplace.Pause()
	require.NoError(s.T(), err)

	// wait for 5 views after pausing our container to replace before we assert healthy network
	s.BlockState.WaitForSealedView(s.T(), currentEpochFinalView+10)

	// make sure the network is healthy after adding new node
	checkNetworkHealth(s.ctx, env, snapshot, info)
}
