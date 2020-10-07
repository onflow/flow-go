package extract

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/state/delta"
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

	log.Info().Msgf("Block state commitment: %s", hex.EncodeToString(stateCommitment))

	mappings, err := getMappingsFromDatabase(db)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get mapping for a database")
	}

	mappingFromFile, err := ReadMegamappings(flagMappingsFile)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load mappings from a file")
	}

	for k, v := range mappingFromFile {
		mappings[k] = v
	}

	fixedMappings, err := FixMappings(mappings)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot fix mappings")
	}

	err = extractExecutionState(flagExecutionStateDir, stateCommitment, flagOutputDir, log.Logger, fixedMappings)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate checkpoint with state commitment")
	}
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}

func fullKeyHash(owner, controller, key string) string {
	hasher := hash.NewSHA2_256()
	return string(hasher.ComputeHash([]byte(fullKey(owner, controller, key))))
}

func FixMappings(mappings map[string]delta.Mapping) (map[string]delta.Mapping, error) {
	fixed := make(map[string]delta.Mapping, len(mappings))

	for k, v := range mappings {
		rehashed := fullKeyHash(v.Owner, v.Controller, v.Key)

		if rehashed == k { //all good
			fixed[k] = v
			continue
		}
		rehashedNoController := fullKeyHash(v.Owner, "", v.Key)
		if rehashedNoController == k {
			fixed[k] = delta.Mapping{
				Owner:      v.Owner,
				Key:        v.Key,
				Controller: "",
			}
			fixed[rehashed] = v
			continue
		}
		return nil, fmt.Errorf("key %x cannot be reconstructed", k)
	}

	return fixed, nil
}
