package extract

import (
	"encoding/hex"
	"fmt"
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagStateCommitment   string
	flagDatadir           string
	flagChain             string
	flagNoMigration       bool
	flagNoReport          bool
	flagVersion           int
)

func getChain(chainName string) (chain flow.Chain, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid chain: %s", r)
		}
	}()
	chain = flow.ChainID(chainName).Chain()
	return
}

var Cmd = &cobra.Command{
	Use:   "execution-state-extract",
	Short: "Reads WAL files and generates the checkpoint containing state commitment for given block hash",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write new Execution State to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"state commitment (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagBlockHash, "block-hash", "",
		"Block hash (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")

	Cmd.Flags().BoolVar(&flagNoMigration, "no-migration", false,
		"don't migrate the state")

	Cmd.Flags().BoolVar(&flagNoReport, "no-report", false,
		"don't report the state")

	Cmd.Flags().IntVar(&flagVersion, "version", 6, "checkpoint version")
}

func run(*cobra.Command, []string) {
	var stateCommitment flow.StateCommitment

	if len(flagBlockHash) > 0 && len(flagStateCommitment) > 0 {
		log.Fatal().Msg("cannot run the command with both block hash and state commitment as inputs, only one of them should be provided")
		return
	}

	if len(flagBlockHash) > 0 {
		blockID, err := flow.HexStringToIdentifier(flagBlockHash)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block hash")
		}

		log.Info().Msgf("extracting state by block ID: %v", blockID)

		db := common.InitStorage(flagDatadir)
		defer db.Close()

		cache := &metrics.NoopCollector{}
		commits := badger.NewCommits(cache, db)

		stateCommitment, err = getStateCommitment(commits, blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot get state commitment for block %v", blockID)
		}
	}

	if len(flagStateCommitment) > 0 {
		var err error
		stateCommitmentBytes, err := hex.DecodeString(flagStateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get decode the state commitment")
		}
		stateCommitment, err = flow.ToStateCommitment(stateCommitmentBytes)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid state commitment length")
		}

		log.Info().Msgf("extracting state by state commitment: %x", stateCommitment)
	}

	if len(flagBlockHash) == 0 && len(flagStateCommitment) == 0 {
		// read state commitment from root checkpoint

		f, err := os.Open(path.Join(flagExecutionStateDir, bootstrap.FilenameWALRootCheckpoint))
		if err != nil {
			log.Fatal().Err(err).Msg("invalid root checkpoint")
		}
		defer f.Close()

		rootHash, err := wal.ReadLastTrieRootHashFromCheckpoint(f)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to read last root hash in root checkpoint: %s", err.Error())
		}

		stateCommitment, err = flow.ToStateCommitment(rootHash[:])
		if err != nil {
			log.Fatal().Err(err).Msg("failed to convert state commitment from last root hash in root checkpoint")
		}
	}

	log.Info().Msgf("Extracting state from %s, exporting root checkpoint to %s, version: %v",
		flagExecutionStateDir,
		path.Join(flagOutputDir, bootstrap.FilenameWALRootCheckpoint),
		flagVersion)

	log.Info().Msgf("Block state commitment: %s from %v, output dir: %s",
		hex.EncodeToString(stateCommitment[:]),
		flagExecutionStateDir,
		flagOutputDir)

	// err := ensureCheckpointFileExist(flagExecutionStateDir)
	// if err != nil {
	// 	log.Fatal().Err(err).Msgf("cannot ensure checkpoint file exist in folder %v", flagExecutionStateDir)
	// }

	chain, err := getChain(flagChain)
	if err != nil {
		log.Fatal().Err(err).Msgf("invalid chain name")
	}

	err = extractExecutionState(
		flagExecutionStateDir,
		stateCommitment,
		flagOutputDir,
		log.Logger,
		chain,
		flagVersion,
		!flagNoMigration,
		!flagNoReport,
	)

	if err != nil {
		log.Fatal().Err(err).Msgf("error extracting the execution state: %s", err.Error())
	}
}

// func ensureCheckpointFileExist(dir string) error {
// 	checkpoints, err := wal.Checkpoints(dir)
// 	if err != nil {
// 		return fmt.Errorf("could not find checkpoint files: %v", err)
// 	}
//
// 	if len(checkpoints) != 0 {
// 		log.Info().Msgf("found checkpoint %v files: %v", len(checkpoints), checkpoints)
// 		return nil
// 	}
//
// 	has, err := wal.HasRootCheckpoint(dir)
// 	if err != nil {
// 		return fmt.Errorf("could not check has root checkpoint: %w", err)
// 	}
//
// 	if has {
// 		log.Info().Msg("found root checkpoint file")
// 		return nil
// 	}
//
// 	return fmt.Errorf("no checkpoint file was found, no root checkpoint file was found")
// }
