package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
)

var (
	flagHeight   uint64
	flagBlockID  string
	flagFinal    bool
	flagSealed   bool
	flagExecuted bool
)

var Cmd = &cobra.Command{
	Use:   "blocks",
	Short: "Read block from protocol state",
	RunE:  runE,
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

	Cmd.Flags().BoolVar(&flagExecuted, "executed", false,
		"get last executed and sealed block (execution node only)")
}

type Reader struct {
	state   protocol.State
	blocks  storage.Blocks
	commits storage.Commits
}

func NewReader(state protocol.State, blocks storage.Blocks, commits storage.Commits) *Reader {
	return &Reader{
		state:   state,
		blocks:  blocks,
		commits: commits,
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

func (r *Reader) GetRoot() (*flow.Block, error) {
	header := r.state.Params().SealedRoot()

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

// IsExecuted returns true if the block is executed
// this only works for execution node.
func (r *Reader) IsExecuted(blockID flow.Identifier) (bool, error) {
	_, err := r.commits.ByBlockID(blockID)
	if err == nil {
		return true, nil
	}

	// statecommitment not exists means the block hasn't been executed yet
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}

	return false, err
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()
	return common.WithStorage(flagDatadir, func(db storage.DB) error {
		chainID, err := badgerstate.GetChainID(db)
		if err != nil {
			return err
		}
		storages := common.InitStorages(db, chainID)
		state, err := common.OpenProtocolState(lockManager, db, storages)

		if err != nil {
			log.Fatal().Err(err).Msg("could not init protocol state")
		}

		reader := NewReader(state, storages.Blocks, storages.Commits)

		// making sure only one flag is being used
		err = checkOnlyOneFlagIsUsed(flagHeight, flagBlockID, flagFinal, flagSealed, flagExecuted)
		if err != nil {
			return fmt.Errorf("could not get block: %w", err)
		}

		if flagHeight > 0 {
			log.Info().Msgf("get block by height: %v", flagHeight)
			block, err := reader.GetBlockByHeight(flagHeight)
			if err != nil {
				log.Fatal().Err(err).Msg("could not get block by height")
			}

			common.PrettyPrintEntity(block)
			return nil
		}

		if flagBlockID != "" {
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block ID: %v: %w", flagBlockID, err)
			}
			log.Info().Msgf("get block by ID: %v", blockID)
			block, err := reader.GetBlockByID(blockID)
			if err != nil {
				return fmt.Errorf("could not get block by ID: %w", err)
			}
			common.PrettyPrintEntity(block)
			return nil
		}

		if flagFinal {
			log.Info().Msgf("get last finalized block")
			block, err := reader.GetFinal()
			if err != nil {
				return fmt.Errorf("could not get finalized block: %w", err)
			}
			common.PrettyPrintEntity(block)
			return nil
		}

		if flagSealed {
			log.Info().Msgf("get last sealed block")
			block, err := reader.GetSealed()
			if err != nil {
				return fmt.Errorf("could not get sealed block: %w", err)
			}
			common.PrettyPrintEntity(block)
			return nil
		}

		if flagExecuted {
			log.Info().Msgf("get last executed and sealed block")
			sealed, err := reader.GetSealed()
			if err != nil {
				return fmt.Errorf("could not get sealed block: %w", err)
			}

			root, err := reader.GetRoot()
			if err != nil {
				return fmt.Errorf("could not get root block: %w", err)
			}

			// find the last executed and sealed block
			for h := sealed.Height; h >= root.Height; h-- {
				block, err := reader.GetBlockByHeight(h)
				if err != nil {
					return fmt.Errorf("could not get block by height: %v: %w", h, err)
				}

				executed, err := reader.IsExecuted(block.ID())
				if err != nil {
					log.Fatal().Err(err).Msgf("could not check block executed or not: %v", h)
				}

				if executed {
					common.PrettyPrintEntity(block)
					return nil
				}
			}

			return fmt.Errorf("could not find executed block")
		}

		return fmt.Errorf("missing flag, try --final or --sealed or --height or --executed or --block-id, note that only one flag can be used at a time")
	})
}

func checkOnlyOneFlagIsUsed(height uint64, blockID string, final, sealed, executed bool) error {
	flags := make([]string, 0, 5)
	if height > 0 {
		flags = append(flags, "height")
	}
	if blockID != "" {
		flags = append(flags, "blockID")
	}
	if final {
		flags = append(flags, "final")
	}
	if sealed {
		flags = append(flags, "sealed")
	}
	if executed {
		flags = append(flags, "executed")
	}

	if len(flags) != 1 {
		return fmt.Errorf("only one flag can be used at a time, used flags: %v", flags)
	}

	return nil
}
