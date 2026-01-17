package checkpoint_list_tries

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpoint string
	flagFast       bool
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-list-tries",
	Short: "Lists tries (root hashes, base64 encoded) of tries stored in a checkpoint",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint")
	Cmd.Flags().BoolVar(&flagFast, "fast", false,
		"read only root hashes without loading the entire checkpoint (faster for large files)")
}

func run(*cobra.Command, []string) {

	if flagFast {
		log.Info().Msgf("reading root hashes from checkpoint %v (fast mode)", flagCheckpoint)
		rootHashes, err := wal.LoadCheckpointRootHashes(flagCheckpoint, log.Logger)
		if err != nil {
			log.Fatal().Err(err).Msg("error while reading checkpoint root hashes")
		}
		log.Info().Msgf("read root hashes, total tries: %v", len(rootHashes))

		for _, rootHash := range rootHashes {
			fmt.Printf("trie root hash: %s\n", rootHash)
		}
	} else {
		log.Info().Msgf("loading checkpoint %v", flagCheckpoint)
		tries, err := wal.LoadCheckpoint(flagCheckpoint, log.Logger)
		if err != nil {
			log.Fatal().Err(err).Msg("error while loading checkpoint")
		}
		log.Info().Msgf("checkpoint loaded, total tries: %v", len(tries))

		for _, trie := range tries {
			fmt.Printf("trie root hash: %s\n", trie.RootHash())
		}
	}
}
