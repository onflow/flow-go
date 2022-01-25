package epochs

import (
	"context"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestEpochs(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestViewsProgress asserts epoch state transitions over two full epochs
// without any nodes joining or leaving.
func (s *Suite) TestViewsProgress() {
	unittest.SkipUnless(s.T(), unittest.TEST_FLAKY, "flaky test")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
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

//the following Epoch join and Leave test will stake a node by submitting all the transactions
//that an node operator would submit, start a new container for that node, and remove
//a container from the network of the same node type. After this orchestration assertions
//specific to that node type are made to ensure the network is healthy.
//
//TestEpochJoinAndLeaveAN should update access nodes and assert healthy network conditions related to the node change
func (s *Suite) TestEpochJoinAndLeaveAN() {
	unittest.SkipUnless(s.T(), unittest.TEST_RESOURCE_INTENSIVE, "epochs AN tests should be run on an machine with adequate resources")
	s.runTestEpochJoinAndLeave(flow.RoleAccess, s.assertNetworkHealthyAfterANChange)
}

// TestEpochJoinAndLeaveVN should update verification nodes and assert healthy network conditions related to the node change
func (s *Suite) TestEpochJoinAndLeaveVN() {
	unittest.SkipUnless(s.T(), unittest.TEST_RESOURCE_INTENSIVE, "epochs VN tests should be run on an machine with adequate resources")
	s.runTestEpochJoinAndLeave(flow.RoleVerification, s.assertNetworkHealthyAfterVNChange)
}

// TestEpochJoinAndLeaveLN should update collection nodes and assert healthy network conditions related to the node change
func (s *Suite) TestEpochJoinAndLeaveLN() {
	unittest.SkipUnless(s.T(), unittest.TEST_RESOURCE_INTENSIVE, "epochs LN tests should be run on an machine with adequate resources")
	s.runTestEpochJoinAndLeave(flow.RoleCollection, s.assertNetworkHealthyAfterLNChange)
}

// TestEpochJoinAndLeaveSN should update consensus nodes and assert healthy network conditions related to the node change
func (s *Suite) TestEpochJoinAndLeaveSN() {
	unittest.SkipUnless(s.T(), unittest.TEST_RESOURCE_INTENSIVE, "epochs SN tests should be run on an machine with adequate resources")
	s.runTestEpochJoinAndLeave(flow.RoleConsensus, s.assertNetworkHealthyAfterSNChange)
}

func (s *Suite) runTestEpochJoinAndLeave(role flow.Role, checkNetworkHealth nodeUpdateValidation) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := utils.LocalnetEnv()

	// grab the first container of this node role type, this is the container we will replace
	containerToReplace := s.getContainerToReplace(role)
	require.NotNil(s.T(), containerToReplace)

	// staking our new node and add get the corresponding container for that node
	info, testContainer := s.StakeNewNode(ctx, env, role)

	// use admin transaction to remove node, this simulates a node leaving the network
	s.removeNodeFromProtocol(ctx, env, containerToReplace.Config.NodeID)

	// wait for epoch setup phase before we start our container and pause the old container
	s.WaitForPhase(ctx, flow.EpochPhaseSetup)

	// get latest snapshot and start new container
	snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	testContainer.WriteRootSnapshot(snapshot)
	testContainer.Container.Start(ctx)

	currentEpochFinalView, err := snapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// wait for 5 views after the start of the next epoch before we pause our container to replace
	s.BlockState.WaitForSealedView(s.T(), currentEpochFinalView+5)

	//make sure container to replace removed from smart contract state
	s.assertNodeNotApprovedOrProposed(ctx, env, containerToReplace.Config.NodeID)

	// assert transition to second epoch happened as expected
	// if counter is still 0, epoch emergency fallback was triggered and we can fail early
	s.assertEpochCounter(ctx, 1)

	err = containerToReplace.Pause()
	require.NoError(s.T(), err)

	// wait for 5 views after pausing our container to replace before we assert healthy network
	s.BlockState.WaitForSealedView(s.T(), currentEpochFinalView+10)

	// make sure the network is healthy after adding new node
	checkNetworkHealth(ctx, env, snapshot, info)

	s.net.StopContainers()
}
