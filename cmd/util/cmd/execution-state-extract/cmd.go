package extract

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagDatadir           string
	flagMappingsFile      string
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

	Cmd.Flags().StringVar(&flagMappingsFile, "mappings-file", "",
		"file containing mappings from previous Sporks")
	_ = Cmd.MarkFlagRequired("mappings-file")
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

	mappings, err := getMappingsFromDatabase(db)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get mapping for a database")
	}

	mappingFromFile, err := readMegamappings(flagMappingsFile)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load mappings from a file")
	}

	for k, v := range mappingFromFile {
		mappings[k] = v
	}

	err = extractExecutionState(flagExecutionStateDir, stateCommitment, flagOutputDir, log.Logger, mappings)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate checkpoint with state commitment")
	}
}
