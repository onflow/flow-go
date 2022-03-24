package extract

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger/common/hash"
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

		db := common.InitStorage(flagDatadir)
		defer db.Close()

		cache := &metrics.NoopCollector{}
		commits := badger.NewCommits(cache, db)

		stateCommitment, err = getStateCommitment(commits, blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get state commitment for block")
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
	}

	if len(flagBlockHash) == 0 && len(flagStateCommitment) == 0 {
		// read state commitment from root checkpoint

		f, err := os.Open(path.Join(flagExecutionStateDir, bootstrap.FilenameWALRootCheckpoint))
		if err != nil {
			log.Fatal().Err(err).Msg("invalid root checkpoint")
		}

		const (
			encMagicSize     = 2
			encVersionSize   = 2
			crcLength        = 4
			encNodeCountSize = 8
			encTrieCountSize = 2
			headerSize       = encMagicSize + encVersionSize
		)

		// read checkpoint version
		header := make([]byte, headerSize)
		n, err := f.Read(header)
		if err != nil || n != headerSize {
			log.Fatal().Err(err).Msg("failed to read version from root checkpoint")
		}

		magic := binary.BigEndian.Uint16(header)
		version := binary.BigEndian.Uint16(header[encMagicSize:])

		if magic != wal.MagicBytes {
			log.Fatal().Err(err).Msg("invalid magic bytes in root checkpoint")
		}

		if version <= 3 {
			_, err = f.Seek(-(hash.HashLen + crcLength), 2 /* relative from end */)
			if err != nil {
				log.Fatal().Err(err).Msg("invalid root checkpoint")
			}
		} else {
			_, err = f.Seek(-(hash.HashLen + encNodeCountSize + encTrieCountSize + crcLength), 2 /* relative from end */)
			if err != nil {
				log.Fatal().Err(err).Msg("invalid root checkpoint")
			}
		}

		n, err = f.Read(stateCommitment[:])
		if err != nil || n != hash.HashLen {
			log.Fatal().Err(err).Msg("failed to read state commitment from root checkpoint")
		}
	}

	log.Info().Msgf("Block state commitment: %s", hex.EncodeToString(stateCommitment[:]))

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
		!flagNoMigration,
		!flagNoReport,
	)

	if err != nil {
		log.Fatal().Err(err).Msgf("error extracting the execution state: %s", err.Error())
	}
}
