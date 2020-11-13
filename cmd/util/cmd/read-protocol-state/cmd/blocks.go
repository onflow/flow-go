package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var (
	flagHeight  uint64
	flagBlockID string
	flagFinal   bool
	flagSealed  bool
)

var Cmd = &cobra.Command{
	Use:   "blocks",
	Short: "Read block from protocol state",
	Run:   run,
}

func init() {
	rootCmd.AddCommand(Cmd)

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"Block height")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "",
		"Block ID (hex-encoded, 64 characters)")

	Cmd.Flags().BoolVar(&flagFinal, "final", false,
		"get finalized block")

	Cmd.Flags().BoolVar(&flagSealed, "sealed", false,
		"get sealed block")
}

type Reader struct {
	state  protocol.State
	blocks storage.Blocks
}

func NewReader(state protocol.State, storages *storage.All) *Reader {
	return &Reader{
		state:  state,
		blocks: storages.Blocks,
	}
}

func (r *Reader) getBlockByHeader(header *flow.Header) (*flow.Block, error) {
	blockID := header.ID()
	block, err := r.blocks.ByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block by ID %v: %w", blockID, err)
	}
	return block, nil
}

func (r *Reader) GetBlockByHeight(height uint64) (*flow.Block, error) {
	header, err := r.state.AtHeight(height).Head()
	if err != nil {
		return nil, fmt.Errorf("could not get header by height: %v, %w", height, err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *Reader) GetFinal() (*flow.Block, error) {
	header, err := r.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *Reader) GetSealed() (*flow.Block, error) {
	header, err := r.state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get sealed block, %w", err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *Reader) GetBlockByID(blockID flow.Identifier) (*flow.Block, error) {
	header, err := r.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("could not get header by blockID: %v, %w", blockID, err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func run(*cobra.Command, []string) {
	db := common.InitStorage(flagDatadir)
	defer db.Close()

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol state")
	}

	reader := NewReader(state, storages)

	if flagHeight > 0 {
		log.Info().Msgf("get block by height: %v", flagHeight)
		block, err := reader.GetBlockByHeight(flagHeight)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get block by height")
		}

		common.PrettyPrintEntity(block)
		return
	}

	if flagBlockID != "" {
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("malformed block ID: %v", flagBlockID)
		}
		log.Info().Msgf("get block by ID: %v", blockID)
		block, err := reader.GetBlockByID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get block by ID")
		}
		common.PrettyPrintEntity(block)
		return
	}

	if flagFinal {
		log.Info().Msgf("get last finalized block")
		block, err := reader.GetFinal()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get finalized block")
		}
		common.PrettyPrintEntity(block)
		return
	}

	if flagSealed {
		log.Info().Msgf("get last sealed block")
		block, err := reader.GetSealed()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get sealed block")
		}
		common.PrettyPrintEntity(block)
		return
	}

	log.Fatal().Msgf("missing flag, try --final or --sealed or --height or --block-id")
}
