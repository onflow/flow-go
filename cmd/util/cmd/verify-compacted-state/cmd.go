// Package verify_compacted_state provides a post-run verification command for the
// compact-execution-state utility.  It checks that an execution-state directory
// satisfies the "compacted sealed state" invariants:
//
//  1. The highest executed block in the protocol DB equals the last sealed block.
//  2. The latest checkpoint in the execution-state directory contains exactly one trie
//     whose root hash equals the sealed block's state commitment.
//  3. The WAL has no non-empty segments after the segment whose number matches the
//     checkpoint name.
package verify_compacted_state

import (
	"errors"
	"fmt"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger"
	flowWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var (
	flagDatadir          string
	flagExecutionStateDir string
)

var Cmd = &cobra.Command{
	Use:   "verify-compacted-state",
	Short: "Verify the invariants of a compacted execution state produced by compact-execution-state",
	Long: `verify-compacted-state checks that the execution-state directory satisfies three invariants:
  1. The highest executed block recorded in the protocol DB equals the last sealed block.
  2. The latest checkpoint in the execution-state directory contains exactly one trie
     whose root hash matches the sealed block's state commitment.
  3. The WAL has no non-empty segment after the segment identified by the checkpoint name.`,
	RunE: runE,
}

func init() {
	common.InitDataDirFlag(Cmd, &flagDatadir)
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "/var/flow/data/execution",
		"directory containing the execution state WAL and checkpoints")
	_ = Cmd.MarkFlagRequired("execution-state-dir")
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	log.Info().
		Str("datadir", flagDatadir).
		Str("execution-state-dir", flagExecutionStateDir).
		Msg("starting verify-compacted-state")

	var errCount int
	fail := func(msg string, args ...any) {
		log.Error().Msgf("FAIL: "+msg, args...)
		errCount++
	}

	// ── Resolve anchor: sealed block and executed block ───────────────────────
	var sealedHeader *flow.Header
	var sealedCommitment flow.StateCommitment
	var executedBlockID flow.Identifier

	if err := common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("cannot open protocol state: %w", err)
		}

		sealedHead, err := state.Sealed().Head()
		if err != nil {
			return fmt.Errorf("cannot get sealed head: %w", err)
		}
		root := state.Params().SealedRoot()

		// Walk backward from sealed tip to find the last executed sealed block.
		for h := sealedHead.Height; h >= root.Height; h-- {
			header, err := state.AtHeight(h).Head()
			if err != nil {
				return fmt.Errorf("cannot get header at height %d: %w", h, err)
			}
			commit, err := storages.Commits.ByBlockID(header.ID())
			if err == nil {
				sealedHeader = header
				sealedCommitment = commit
				break
			}
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("cannot check commitment at height %d: %w", h, err)
			}
		}
		if sealedHeader == nil {
			return fmt.Errorf("no executed sealed block found")
		}

		if err := operation.RetrieveExecutedBlock(db.Reader(), &executedBlockID); err != nil {
			return fmt.Errorf("cannot retrieve executed block ID: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	// ── Check 1: executed height == sealed height ────────────────────────────
	sealedBlockID := sealedHeader.ID()
	if executedBlockID == sealedBlockID {
		log.Info().
			Uint64("height", sealedHeader.Height).
			Hex("block-id", sealedBlockID[:]).
			Msg("OK: executed block equals sealed block")
	} else {
		fail("executed block %v does not match sealed block %v (height %d)",
			executedBlockID, sealedBlockID, sealedHeader.Height)
	}

	// ── Check 2: latest checkpoint has exactly one trie with root hash == C ──
	checkpointNums, _, err := flowWAL.ListCheckpoints(flagExecutionStateDir)
	if err != nil {
		return fmt.Errorf("cannot list checkpoints: %w", err)
	}
	if len(checkpointNums) == 0 {
		fail("no checkpoint files found in %s", flagExecutionStateDir)
	} else {
		// Find the latest (highest-numbered) checkpoint.
		latestNum := checkpointNums[0]
		for _, n := range checkpointNums[1:] {
			if n > latestNum {
				latestNum = n
			}
		}
		latestName := flowWAL.NumberToFilename(latestNum)

		tries, err := flowWAL.OpenAndReadCheckpointV6(flagExecutionStateDir, latestName, log.Logger)
		if err != nil {
			fail("cannot read checkpoint %s: %v", latestName, err)
		} else if len(tries) != 1 {
			fail("checkpoint %s must contain exactly 1 trie, found %d", latestName, len(tries))
		} else {
			checkpointHash := ledger.RootHash(tries[0].RootHash())
			expected := ledger.RootHash(sealedCommitment)
			if checkpointHash.Equals(expected) {
				log.Info().
					Str("checkpoint", latestName).
					Str("root-hash", checkpointHash.String()).
					Msg("OK: checkpoint root hash matches sealed state commitment")
			} else {
				fail("checkpoint %s root hash %v does not match sealed commitment %v",
					latestName, checkpointHash, expected)
			}

			// ── Check 3: no non-empty WAL segment after latestNum ─────────────
			first, last, err := prometheusWAL.Segments(flagExecutionStateDir)
			if err != nil {
				return fmt.Errorf("cannot enumerate WAL segments: %w", err)
			}

			if last <= latestNum {
				log.Info().
					Int("last-segment", last).
					Int("checkpoint-segment", latestNum).
					Msg("OK: no WAL segments after checkpoint segment")
			} else {
				// Check segments after latestNum for non-empty content.
				nonEmpty := 0
				for seg := latestNum + 1; seg <= last; seg++ {
					seg := seg
					sr, err := prometheusWAL.NewSegmentsRangeReader(
						log.Logger,
						prometheusWAL.SegmentRange{Dir: flagExecutionStateDir, First: seg, Last: seg},
					)
					if err != nil {
						fail("cannot open segment %d: %v", seg, err)
						continue
					}
					reader := prometheusWAL.NewReader(sr)
					hasRecords := reader.Next()
					sr.Close()
					if hasRecords {
						nonEmpty++
					}
				}

				if nonEmpty == 0 {
					log.Info().
						Int("first-segment", first).
						Int("last-segment", last).
						Int("checkpoint-segment", latestNum).
						Msg("OK: all WAL segments after checkpoint segment are empty")
				} else {
					fail("%d non-empty WAL segment(s) found after checkpoint segment %d", nonEmpty, latestNum)
				}
			}
		}
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	if errCount > 0 {
		return fmt.Errorf("verify-compacted-state found %d problem(s); see logs above", errCount)
	}

	log.Info().Msg("verify-compacted-state: all checks passed")
	return nil
}

