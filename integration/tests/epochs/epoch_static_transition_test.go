package epochs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

func TestEpochStaticTransition(t *testing.T) {
	suite.Run(t, new(StaticEpochTransitionSuite))
}

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
