package cmd

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var listTriesCmd = &cobra.Command{
	Use:   "list-tries",
	Short: "lists root hashes of all tries",
	Run:   listTries,
}

func init() {
	RootCmd.AddCommand(listTriesCmd)
}

func listTries(*cobra.Command, []string) {
	startTime := time.Now()

	// load execution state
	forest, err := initForest()
	if err != nil {
		log.Fatal().Err(err).Msg("error while loading execution state")
	}

	tries, err := forest.GetTries()
	if err != nil {
		log.Fatal().Err(err).Msg("error while getting tries")
	}

	for _, trie := range tries {
		fmt.Printf("%s\n", trie.StringRootHash())
	}

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
