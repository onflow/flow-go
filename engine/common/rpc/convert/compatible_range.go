package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

// CompatibleRangeToMessage converts a flow.CompatibleRange to a protobuf message
func CompatibleRangeToMessage(c *flow.CompatibleRange) *entities.CompatibleRange {
	if c != nil {
		return &entities.CompatibleRange{
			StartHeight: c.StartHeight,
			EndHeight:   c.EndHeight,
		}
	}

	return nil
}
