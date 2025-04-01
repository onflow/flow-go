package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// CompatibleRangeToMessage converts an accessmodel.CompatibleRange to a protobuf message
func CompatibleRangeToMessage(c *accessmodel.CompatibleRange) *entities.CompatibleRange {
	if c == nil {
		// compatible range is optional, so nil is a valid value
		return nil
	}

	return &entities.CompatibleRange{
		StartHeight: c.StartHeight,
		EndHeight:   c.EndHeight,
	}
}

// MessageToCompatibleRange converts a protobuf message to an accessmodel.CompatibleRange
func MessageToCompatibleRange(c *entities.CompatibleRange) *accessmodel.CompatibleRange {
	if c == nil {
		// compatible range is optional, so nil is a valid value
		return nil
	}

	return &accessmodel.CompatibleRange{
		StartHeight: c.StartHeight,
		EndHeight:   c.EndHeight,
	}
}
