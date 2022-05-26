package list_tries

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/mtrie"
)

var cmd = &cobra.Command{
	Use:   "list-tries",
	Short: "Lists root hashes of all tries",
	Run:   run,
}

var stateLoader func() *mtrie.Forest = nil

func Init(f func() *mtrie.Forest) *cobra.Command {
	stateLoader = f

	return cmd
}

func run(*cobra.Command, []string) {
	startTime := time.Now()

	mForest := stateLoader()

	tries, err := mForest.GetTries()
	if err != nil {
		log.Fatal().Err(err).Msg("error while getting tries")
	}

	log.Info().Msgf("print root hash for %v tries", len(tries))

	for i, trie := range tries {
		log.Info().Msgf("%v-th trie hash: %s", i, trie.RootHash())
	}

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
