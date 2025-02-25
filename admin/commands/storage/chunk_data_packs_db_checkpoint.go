package storage

import (
	"context"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*ChunksCheckpointCommand)(nil)

// ChunksCheckpointCommand creates a checkpoint for pebble database for querying the data
// while keeping the node alive.
type ChunksCheckpointCommand struct {
	chunkDataPacks *pebble.DB
}

func NewChunksCheckpointCommand(chunkDataPacks *pebble.DB) commands.AdminCommand {
	return &ChunksCheckpointCommand{
		chunkDataPacks: chunkDataPacks,
	}
}

func (c *ChunksCheckpointCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return nil, admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	targetDir, ok := input["target-dir"]
	if !ok {
		return nil, admin.NewInvalidAdminReqErrorf("the \"target-dir\" field is required")
	}
	targetDirStr, ok := targetDir.(string)
	if !ok {
		return nil, admin.NewInvalidAdminReqErrorf("the \"target-dir\" field must be string")
	}

	err := c.chunkDataPacks.Checkpoint(targetDirStr)
	if err != nil {
		return nil, admin.NewInvalidAdminReqErrorf("failed to create checkpoint at %v: %w", targetDir, err)
	}

	return fmt.Errorf("successfully created checkpoint db checkpoint at %v", targetDirStr), nil
}

func (c *ChunksCheckpointCommand) Validator(req *admin.CommandRequest) error {
	return nil
}
