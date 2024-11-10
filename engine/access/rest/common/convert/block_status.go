package convert

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const (
	Finalized = "finalized"
	Sealed    = "sealed"
)

func ParseBlockStatus(blockStatus string) (flow.BlockStatus, error) {
	switch blockStatus {
	case Finalized:
		return flow.BlockStatusFinalized, nil
	case Sealed:
		return flow.BlockStatusSealed, nil
	}
	return flow.BlockStatusUnknown, fmt.Errorf("invalid 'block_status', must be '%s' or '%s'", Finalized, Sealed)
}
