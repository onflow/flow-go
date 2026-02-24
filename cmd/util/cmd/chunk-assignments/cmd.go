package chunk_assignments

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/storage"
)

var (
	flagDatadir string
	flagBlockID string
	flagAlpha   uint
)

var Cmd = &cobra.Command{
	Use:   "chunk-assignments",
	Short: "Print verification node assignments for each chunk in a block",
	Long: `Print which verification nodes are assigned to verify each chunk for a given block.

This utility computes the chunk assignment using the Public Chunk Assignment algorithm,
which deterministically assigns verification nodes to chunks based on the block's randomness.

Requires:
- A protocol database containing the block and its execution result
- The incorporating block must have a valid child (for randomness source)`,
	Run: run,
}

func init() {
	common.InitDataDirFlag(Cmd, &flagDatadir)

	Cmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block ID to get chunk assignments for")
	Cmd.Flags().UintVar(&flagAlpha, "alpha", flow.DefaultChunkAssignmentAlpha, "number of verifiers assigned per chunk")
	_ = Cmd.MarkFlagRequired("block-id")
}

// ChunkAssignment represents the assignment of verifiers to a single chunk
type ChunkAssignment struct {
	ChunkIndex uint64                `json:"chunk_index"`
	ChunkID    flow.Identifier       `json:"chunk_id"`
	Verifiers  []VerifierAssignment  `json:"verifiers"`
}

// VerifierAssignment represents a single verifier assigned to a chunk
type VerifierAssignment struct {
	NodeID  flow.Identifier `json:"node_id"`
	Address string          `json:"address,omitempty"`
}

// BlockChunkAssignments represents all chunk assignments for a block
type BlockChunkAssignments struct {
	BlockID           flow.Identifier   `json:"block_id"`
	BlockHeight       uint64            `json:"block_height"`
	ResultID          flow.Identifier   `json:"result_id"`
	Alpha             uint              `json:"alpha"`
	TotalChunks       int               `json:"total_chunks"`
	TotalVerifiers    int               `json:"total_verifiers"`
	ChunkAssignments  []ChunkAssignment `json:"chunk_assignments"`
}

func run(*cobra.Command, []string) {
	blockID, err := flow.HexStringToIdentifier(flagBlockID)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid block ID format")
	}

	lockManager := storage.MakeSingletonLockManager()

	err = common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("could not open protocol state: %w", err)
		}

		// Get the block header
		header, err := storages.Headers.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not get block header for %v: %w", blockID, err)
		}

		// Get the execution result for this block
		result, err := storages.Results.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not get execution result for block %v: %w", blockID, err)
		}

		// Create chunk assigner
		assigner, err := chunks.NewChunkAssigner(flagAlpha, state)
		if err != nil {
			return fmt.Errorf("could not create chunk assigner: %w", err)
		}

		// Compute assignments (using the block itself as the incorporating block)
		assignment, err := assigner.Assign(result, blockID)
		if err != nil {
			return fmt.Errorf("could not compute chunk assignments: %w", err)
		}

		// Get verifier identities for additional context
		verifiers, err := state.AtBlockID(result.BlockID).Identities(
			filter.And(
				filter.HasInitialWeight[flow.Identity](true),
				filter.HasRole[flow.Identity](flow.RoleVerification),
				filter.IsValidCurrentEpochParticipant,
			))
		if err != nil {
			return fmt.Errorf("could not get verifiers: %w", err)
		}
		verifierMap := verifiers.Lookup()

		// Build output
		output := BlockChunkAssignments{
			BlockID:        blockID,
			BlockHeight:    header.Height,
			ResultID:       result.ID(),
			Alpha:          flagAlpha,
			TotalChunks:    assignment.Len(),
			TotalVerifiers: len(verifiers),
		}

		// Get assignments for each chunk
		for i := 0; i < assignment.Len(); i++ {
			chunkIdx := uint64(i)
			chunk, ok := result.Chunks.ByIndex(chunkIdx)
			if !ok {
				return fmt.Errorf("chunk index %d not found in result", i)
			}

			assignedVerifiers, err := assignment.Verifiers(chunkIdx)
			if err != nil {
				return fmt.Errorf("could not get verifiers for chunk %d: %w", i, err)
			}

			chunkAssignment := ChunkAssignment{
				ChunkIndex: chunkIdx,
				ChunkID:    chunk.ID(),
				Verifiers:  make([]VerifierAssignment, 0, len(assignedVerifiers)),
			}

			for verifierID := range assignedVerifiers {
				va := VerifierAssignment{
					NodeID: verifierID,
				}
				// Add address if available
				if identity, ok := verifierMap[verifierID]; ok {
					va.Address = identity.Address
				}
				chunkAssignment.Verifiers = append(chunkAssignment.Verifiers, va)
			}

			output.ChunkAssignments = append(output.ChunkAssignments, chunkAssignment)
		}

		// Output as JSON
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			return fmt.Errorf("could not encode output: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to compute chunk assignments")
	}
}
