package dkg

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/suite"
)

func TestWithEmulator(t *testing.T) {
	suite.Run(t, new(DKGSuite))
}

func (s *DKGSuite) TestHappyPath() {

	// The EpochSetup event is received at view 100. The phase transitions are
	// at views 150, 200, and 250. In between phase transitions, the controller
	// calls the DKG smart-contract every 10 views.
	//
	// VIEWS
	// setup      : 100
	// polling    : 110 120 130 140 150
	// Phase1Final: 150
	// polling    : 160 170 180 190 200
	// Phase2Final: 200
	// polling    : 210 220 230 240 250
	// Phase3Final: 250
	// final

	// create the EpochSetup that will trigger the next DKG run with all the
	// desired parameters
	epochSetup := flow.EpochSetup{
		Counter:            999,
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 250,
		FinalView:          300,
		Participants:       s.netIDs,
		RandomSource:       []byte("random bytes for seed"),
	}
	firstBlock := &flow.Header{View: 100}

	for _, node := range s.nodes {
		node.setEpochSetup(s.T(), epochSetup, firstBlock)
	}

	for _, n := range s.nodes {
		n.Ready()
	}

	// trigger the EpochSetupPhaseStarted event for all nodes, effectively
	// starting the next DKG run
	for _, n := range s.nodes {
		n.ProtocolEvents.EpochSetupPhaseStarted(epochSetup.Counter, firstBlock)
	}

	// XXX do something

	for _, n := range s.nodes {
		n.Done()
	}
}
