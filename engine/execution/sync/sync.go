package sync

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

type StateSynchronizer interface {
	//PersistDelta(blockID flow.Identifier, set flow.RegisterDelta) error
	DeltaRange(
		startID, endID flow.Identifier,
		onDelta func(delta *messages.ExecutionStateDelta) error,
	) error
}

func NewStateSynchronizer(
	state state.ReadOnlyExecutionState,
//registerDeltas storage.RegisterDeltas,
) StateSynchronizer {
	return &stateSync{
		execState: state,
		//registerDeltas: registerDeltas,
	}
}

type stateSync struct {
	execState state.ReadOnlyExecutionState
	//registerDeltas storage.RegisterDeltas
}

// walk the chain to find a path between blocks
func (ss *stateSync) findDeltasToSend(startID, endID flow.Identifier) ([]*messages.ExecutionStateDelta, error) {

	if startID == endID {
		return nil, nil
	}

	//find lowest common block
	startDelta, err := ss.execState.RetrieveStateDelta(startID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve state delta for blockID %s: %w", startID, err)
	}

	endDelta, err := ss.execState.RetrieveStateDelta(endID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve state delta for blockID %s: %w", endID, err)
	}


	// start from higher block
	highDelta := endDelta
	lowDelta := startDelta

	// list of deltas to be send
	// TODO - loading everything into memory might not be a best idea, but will do for now
	higherChainDelta := []*messages.ExecutionStateDelta{endDelta}
	lowerChainDelta := []*messages.ExecutionStateDelta{startDelta}
	var finalChainDelta []*messages.ExecutionStateDelta

	// assume that end block is higher then start
	// but other option are also supported, so handy variable to distinguish later
	reversedHeights := false

	if startDelta.Block.Height > endDelta.Block.Height {
		highDelta, lowDelta = lowDelta, highDelta
		higherChainDelta, lowerChainDelta = lowerChainDelta, higherChainDelta
		reversedHeights = true
	}

	// worst case we should reach genesis, block common for all
	for {
		highParentDelta, err := ss.execState.RetrieveStateDelta(highDelta.Block.ParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve state delta for blockID %s: %w", highDelta.Block.ParentID, err)
		}

		if highParentDelta.Block.ID() == lowDelta.Block.ID() {
			//we found a simple path

			if reversedHeights {
				return nil, fmt.Errorf("invalid request. Start block higher then lower on the same chain")
			}

			finalChainDelta = higherChainDelta
			break
		}

		if highDelta.Block.Height <= lowDelta.Block.Height {
			// reached end block height without match
			// now descent both paths until we find common parent
			lowParentDelta, err := ss.execState.RetrieveStateDelta(lowDelta.Block.ParentID)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve state delta for blockID %s: %w", lowDelta.Block.ParentID, err)
			}

			if lowParentDelta.Block.ID() == highParentDelta.Block.ID() {
				//found parent block
				break
			}
			lowerChainDelta = append(lowerChainDelta, lowParentDelta)
			lowDelta = lowParentDelta
		}
		higherChainDelta = append(higherChainDelta, highParentDelta)
		highDelta = highParentDelta
	}

	if reversedHeights {
		finalChainDelta = lowerChainDelta
	} else {
		finalChainDelta = higherChainDelta
	}

	// reverse order in place
	for i, j := 0, len(finalChainDelta)-1; i < j; i, j = i+1, j-1 {
		finalChainDelta[i], finalChainDelta[j] = finalChainDelta[j], finalChainDelta[i]
	}

	return finalChainDelta, nil
}

func (ss *stateSync) DeltaRange(
	startID, endID flow.Identifier,
	onDelta func(delta *messages.ExecutionStateDelta) error,
) error {

	// if range is zero, we're done
	deltas, err := ss.findDeltasToSend(startID, endID)
	if err != nil {
		return fmt.Errorf("cannot find deltas to send: %w", err)
	}

	for _, delta := range deltas {
		err := onDelta(delta)
		if err != nil {
			return fmt.Errorf("error while processing delta handler: %w", err)
		}
	}

	return nil
}
