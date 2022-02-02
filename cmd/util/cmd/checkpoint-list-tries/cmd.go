package checkpoint_list_tries

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpoint string
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
}

func run(*cobra.Command, []string) {

	tries, err := wal.LoadCheckpoint(flagCheckpoint)
	if err != nil {
		log.Fatal().Err(err).Msg("error while loading checkpoint")
	}

	for _, trie := range tries {
		fmt.Printf("%x\n", trie.RootHash())
	}
}
