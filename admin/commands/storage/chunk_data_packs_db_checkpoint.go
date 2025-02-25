package storage

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var _ commands.AdminCommand = (*ChunksCheckpointCommand)(nil)

// ChunksCheckpointCommand creates a checkpoint for pebble database for querying the data
// while keeping the node alive.
type ChunksCheckpointCommand struct {
	checkpointDir  string
	chunkDataPacks *pebble.DB
}

func NewChunksCheckpointCommand(dir string, chunkDataPacks *pebble.DB) commands.AdminCommand {
	return &ChunksCheckpointCommand{
		checkpointDir:  dir,
		chunkDataPacks: chunkDataPacks,
	}
}

func (c *ChunksCheckpointCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	log.Info().Msgf("admintool: creating chunkDataPacks database checkpoint")

	targetDir := nextTmpFolder(c.checkpointDir)

	log.Info().Msgf("admintool: creating chunkDataPacks database checkpoint at: %v", targetDir)

	err := c.chunkDataPacks.Checkpoint(targetDir)
	if err != nil {
		return nil, admin.NewInvalidAdminReqErrorf("failed to create checkpoint at %v: %w", targetDir, err)
	}

	log.Info().Msgf("admintool: successfully created chunkDataPacks database checkpoint at: %v", targetDir)

	return fmt.Sprintf("successfully created chunkDataPacks db checkpoint at %v", targetDir), nil
}

func (c *ChunksCheckpointCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

func nextTmpFolder(dir string) string {
	// use timestamp as folder name
	folderName := time.Now().Format("2006-01-02_15-04-05")
	return path.Join(dir, folderName)
}
