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

var _ commands.AdminCommand = (*PebbleDBCheckpointCommand)(nil)

// PebbleDBCheckpointCommand creates a checkpoint for pebble database for querying the data
// while keeping the node alive.
type PebbleDBCheckpointCommand struct {
	checkpointDir string
	dbname        string // dbname is for logging purposes only
	pebbleDB      *pebble.DB
}

func NewPebbleDBCheckpointCommand(checkpointDir string, dbname string, pebbleDB *pebble.DB) *PebbleDBCheckpointCommand {
	return &PebbleDBCheckpointCommand{
		checkpointDir: checkpointDir,
		dbname:        dbname,
		pebbleDB:      pebbleDB,
	}
}

func (c *PebbleDBCheckpointCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	log.Info().Msgf("admintool: creating %v database checkpoint", c.dbname)

	targetDir := nextTmpFolder(c.checkpointDir)

	log.Info().Msgf("admintool: creating %v database checkpoint at: %v", c.dbname, targetDir)

	err := c.pebbleDB.Checkpoint(targetDir)
	if err != nil {
		return nil, admin.NewInvalidAdminReqErrorf("failed to create %v pebbledb checkpoint at %v: %w", c.dbname, targetDir, err)
	}

	log.Info().Msgf("admintool: successfully created %v database checkpoint at: %v", c.dbname, targetDir)

	return fmt.Sprintf("successfully created %v db checkpoint at %v", c.dbname, targetDir), nil
}

func (c *PebbleDBCheckpointCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

func nextTmpFolder(dir string) string {
	// use timestamp as folder name
	folderName := time.Now().Format("2006-01-02_15-04-05")
	return path.Join(dir, folderName)
}
