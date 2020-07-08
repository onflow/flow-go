package list_tries

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/ledger/outright/mtrie"
)

var cmd = &cobra.Command{
	Use:   "list-tries",
	Short: "Lists root hashes of all tries",
	Run:   run,
}

var stateLoader func() *mtrie.MForest = nil

func Init(f func() *mtrie.MForest) *cobra.Command {
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

	for _, trie := range tries {
		fmt.Printf("%s\n", trie.StringRootHash())
	}

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
