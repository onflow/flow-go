package cruisectl

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"time"
)

// measurement represents one measurement of block rate and error.
// A measurement is taken each time the view changes for any reason.
type measurement struct {
	view            uint64    // v
	time            time.Time // t[v]
	blockRate       float64   // r[v]
	aveBlockRate    float64   // r_N[v]
	targetBlockRate float64   // r_SP
}

type epochInfo struct {
	curEpochFinalView        uint64
	curEpochTargetSwitchover time.Time
	nextEpochFinalView       *uint64
}

// BlockRateController dynamically adjusts the block rate delay of this node,
// based on the measured block rate of the consensus committee as a whole, in
// order to achieve a target overall block rate.
type BlockRateController struct {
	cm     *component.ComponentManager
	config *Config

	lastMeasurement *measurement
	epochInfo
}

func NewBlockRateController() (*BlockRateController, error) {
	return nil, nil
}

// OnViewChange handles events from HotStuff.
func (ctl *BlockRateController) OnViewChange(oldView, newView uint64) {
	// TODO
}

func (ctl *BlockRateController) EpochSetupPhaseStarted(currentEpochCounter uint64, first *flow.Header) {
	// TODO
}

func (ctl *BlockRateController) EpochEmergencyFallbackTriggered() {
	// TODO
}
