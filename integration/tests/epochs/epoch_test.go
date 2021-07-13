package epochs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	state "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage/badger"
)

func TestEpochs(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestViewsProgress asserts epoch state transitions over two full epochs
// without any nodes joining or leaving.
func (s *Suite) TestViewsProgress() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// phaseCheck is a utility struct that contains information about the
	// final view and final block for epoch phases.
	type phaseCheck struct {
		epoch      uint64
		phase      flow.EpochPhase
		finalView  uint64          // the final view of the phase as defined by the EpochSetup
		finalBlock flow.Identifier // the parent of the first sealed block that has a view greater or equal than finalView
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
			proposal := s.BlockState.WaitForSealedView(s.T(), v.finalView)
			v.finalBlock = proposal.Header.ParentID
		}

		phaseChecks = append(phaseChecks, epochViews...)
	}

	s.net.StopContainers()

	consensusContainers := []*testnet.Container{}
	for _, c := range s.net.Containers {
		if c.Config.Role == flow.RoleConsensus {
			consensusContainers = append(consensusContainers, c)
		}
	}

	for _, c := range consensusContainers {
		containerState, err := openContainerState(c)
		require.NoError(s.T(), err)

		for _, v := range phaseChecks {
			snapshot := containerState.AtBlockID(v.finalBlock)

			epoch := snapshot.Epochs().Current()

			currentEpochCounter, err := epoch.Counter()
			require.NoError(s.T(), err)
			require.Equal(s.T(), v.epoch, currentEpochCounter, fmt.Sprintf("wrong epoch at view %d", v.finalView))

			currentPhase, err := snapshot.Phase()
			require.NoError(s.T(), err)
			require.Equal(s.T(), v.phase, currentPhase, fmt.Sprintf("wrong phase at view %d", v.finalView))
		}
	}
}

func openContainerState(container *testnet.Container) (*state.State, error) {
	db, err := container.DB()
	if err != nil {
		return nil, err
	}

	metrics := metrics.NewNoopCollector()
	index := badger.NewIndex(metrics, db)
	headers := badger.NewHeaders(metrics, db)
	seals := badger.NewSeals(metrics, db)
	results := badger.NewExecutionResults(metrics, db)
	receipts := badger.NewExecutionReceipts(metrics, db, results)
	guarantees := badger.NewGuarantees(metrics, db)
	payloads := badger.NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := badger.NewBlocks(db, headers, payloads)
	setups := badger.NewEpochSetups(metrics, db)
	commits := badger.NewEpochCommits(metrics, db)
	statuses := badger.NewEpochStatuses(metrics, db)

	return state.OpenState(
		metrics,
		db,
		headers,
		seals,
		results,
		blocks,
		setups,
		commits,
		statuses,
	)
}
