package storage

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*ProtocolSnapshotCommand)(nil)

type protocolSnapshotData struct {
	blocksToSkip uint
}

// ProtocolSnapshotCommand is a command that generates a protocol snapshot for a checkpoint (usually latest checkpoint)
// This command is only available for execution node
type ProtocolSnapshotCommand struct {
	logger        zerolog.Logger
	state         protocol.State
	headers       storage.Headers
	seals         storage.Seals
	checkpointDir string // the directory where the checkpoint is stored
}

func NewProtocolSnapshotCommand(
	logger zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	checkpointDir string,
) *ProtocolSnapshotCommand {
	return &ProtocolSnapshotCommand{
		logger:        logger,
		state:         state,
		headers:       headers,
		seals:         seals,
		checkpointDir: checkpointDir,
	}
}

func (s *ProtocolSnapshotCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	validated, ok := req.ValidatorData.(*protocolSnapshotData)
	if !ok {
		return nil, fmt.Errorf("fail to parse validator data")
	}

	blocksToSkip := validated.blocksToSkip

	s.logger.Info().Uint("blocksToSkip", blocksToSkip).Msgf("admintool: generating protocol snapshot")

	snapshot, sealedHeight, commit, err := common.GenerateProtocolSnapshotForCheckpoint(
		s.logger, s.state, s.headers, s.seals, s.checkpointDir, blocksToSkip)
	if err != nil {
		return nil, fmt.Errorf("could not generate protocol snapshot for checkpoint, checkpointDir %v: %w",
			s.checkpointDir, err)
	}

	header, err := snapshot.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get header from snapshot: %w", err)
	}

	serializable, err := inmem.FromSnapshot(snapshot)
	if err != nil {
		return nil, fmt.Errorf("could not convert snapshot to serializable: %w", err)
	}

	s.logger.Info().
		Uint64("finalized_height", header.Height). // finalized height
		Hex("finalized_block_id", logging.Entity(header)).
		Uint64("sealed_height", sealedHeight).
		Hex("sealed_commit", commit[:]). // not the commit for the finalized height, but for the sealed height
		Uint("blocks_to_skip", blocksToSkip).
		Msgf("admintool: protocol snapshot generated successfully")

	return commands.ConvertToMap(serializable.Encodable())
}

func (s *ProtocolSnapshotCommand) Validator(req *admin.CommandRequest) error {
	// blocksToSkip is the number of blocks to skip when iterating the sealed heights to find the state commitment
	// in the checkpoint file.
	// default is 0
	validated := &protocolSnapshotData{
		blocksToSkip: uint(0),
	}

	input, ok := req.Data.(map[string]interface{})
	if ok {
		data, ok := input["blocks-to-skip"]

		if ok {
			n, ok := data.(float64)
			if !ok {
				return fmt.Errorf("could not parse blocks-to-skip: %v", data)
			}
			validated.blocksToSkip = uint(n)
		}
	}

	req.ValidatorData = validated

	return nil
}
