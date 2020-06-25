package extract

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagDatadir           string
)

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

	Cmd.Flags().StringVar(&flagBlockHash, "block-hash", "",
		"Block hash (hex-encoded, 64 characters)")
	_ = Cmd.MarkFlagRequired("block-hash")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")
}

func run(*cobra.Command, []string) {

	blockID, err := flow.HexStringToIdentifier(flagBlockHash)
	if err != nil {
		log.Fatal().Err(err).Msg("malformed block hash")
	}

	db := common.InitStorage(flagDatadir)
	defer db.Close()

	cache := &metrics.NoopCollector{}
	commits := badger.NewCommits(cache, db)

	stateCommitment, err := getStateCommitment(commits, blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get state commitment for block")
	}

	fmt.Printf("%x\n", stateCommitment)

	err = extractExecutionState(flagExecutionStateDir, stateCommitment, flagOutputDir, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate checkpoint with state commitment")
	}

	time.Sleep(2 * time.Second)
}
