package flowmc

import (
	"fmt"
	mockproc "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/primary"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type MockEventProcessor struct {

}

func (m *MockEventProcessor) OnForkChoiceTrigger(uint64) { }
func (m *MockEventProcessor) OnEnteringView(v uint64) {
	fmt.Println(v)
}
func (m *MockEventProcessor) OnPassiveTillView(uint64) { }
func (m *MockEventProcessor) OnWaitingForBlockTimeout(uint64) { }
func (m *MockEventProcessor) OnWaitingForVotesTimeout(uint64) { }



func initPaceMaker(view uint64) (*FlowMC, *mockproc.Processor) {
	committee := []types.ID {"Replica0", "Replica1", "Me", "Replica3", "Replica4"}
	primarySelector := primary.NewRoundRobinSelector(committee)
	eventProc := &mockproc.Processor{}
	pm := New("Me", view, primarySelector, eventProc)
	return pm, eventProc
}

func qc(view uint64) *types.QuorumCertificate {
	return &types.QuorumCertificate{View:view}
}

func makeBlockProposal(qcView, blockView uint64) *types.BlockProposal {
	return &types.BlockProposal{
		Block: &types.Block{View: blockView, QC: qc(qcView)},
	}
}


// Test_SkipViewThroughQC tests that PaceMaker skips View when receiving QC with larger View Number
func Test_SkipViewThroughQC(t *testing.T) {
	pm, eventProc := initPaceMaker(3)

	// fill events in channels and specify expected Mock calls
	pm.OnQcFromVotesIncorporated(qc(12))
	eventProc.On("OnEnteringView", uint64(3)).Return().Once()
	eventProc.On("OnPassiveTillView", uint64(12)).Return().Once()

	//eventProc.ExpectedCalls
	// run view and verify emitted events
	pm.ExecuteView()
	//eventProc.AssertCalled(t, "OnEnteringView", 12)
	//eventProc.MethodCalled("OnEnteringView", 1)

	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(13), pm.CurrentView()) // view increment caused by processing qc with view 12
	//eventProc.AssertNumberOfCalls(t, "OnEnteringView", 1)

}

// Test_SkipViewThroughBlock tests that PaceMaker skips View when receiving Block containing QC with larger View Number
func Test_SkipViewThroughBlock(t *testing.T) {
	pm, eventProc := initPaceMaker(3)

	// fill events in channels and specify expected Mock calls
	pm.OnBlockIncorporated(makeBlockProposal(5, 13))
	eventProc.On("OnEnteringView", uint64(3)).Return().Once()
	eventProc.On("OnPassiveTillView", uint64(5)).Return().Once()

	pm.ExecuteView()
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(6), pm.CurrentView()) // view increment caused by processing block's qc with view 5
}

// Test_SkipViewAndProcessBlock tests that PaceMaker:
// - skips View when receiving Block containing QC with larger View Number
// - processes block right after (given that it has the respective view number)
// I.e. PaceMaker is not susceptible to skip-View attack
func Test_SkipViewAndProcessBlock(t *testing.T) {
	pm, eventProc := initPaceMaker(3)

	// fill events in channels and specify expected Mock calls
	pm.OnBlockIncorporated(makeBlockProposal(12, 13))
	eventProc.On("OnEnteringView", uint64(3)).Return().Once()
	eventProc.On("OnPassiveTillView", uint64(12)).Return().Once()

	pm.ExecuteView()
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurrentView()) // view increment caused processing block with view = 14
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldBlocks(t *testing.T) {
	pm, eventProc := initPaceMaker(3)

	// fill events in channels and specify expected Mock calls
	pm.OnBlockIncorporated(makeBlockProposal(1, 2))
	eventProc.On("OnEnteringView", uint64(3)).Return().Once()
	eventProc.On("OnWaitingForBlockTimeout", uint64(3)).Return().Once()

	pm.ExecuteView()
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurrentView()) // view increment caused by timeout
}

// Test_IgnoreBlockDuplicates tests that PaceMaker ignores duplicate blocks
func Test_IgnoreBlockDuplicates(t *testing.T) {
	pm, eventProc := initPaceMaker(2) // I am primary for view 2

	go func() {
		cycleTime := int64(pm.timeout.replicaTimeout * pm.timeout.voteAggregationTimeoutFraction / 20) // we expect 20 cycles of time cycleTime before OnWaitingForVotesTimeout should be triggered
		for i := 0 ; i <= 40; i++ {
			pm.OnBlockIncorporated(makeBlockProposal(1, 2))
			// time.Duration expects an int64 as input which specifies the duration in units of nanoseconds (1E-9)
			time.Sleep(time.Duration(cycleTime * 1E6))
			if (pm.CurrentView() > 2) {
				break
			}
		}
	}()

	eventProc.On("OnEnteringView", uint64(2)).Return().Once()
	eventProc.On("OnForkChoiceTrigger", uint64(2)).Return().Once()
	eventProc.On("OnWaitingForVotesTimeout", uint64(2)).Return().Once()

	pm.ExecuteView()
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurrentView()) // view increment caused by timeout
}


