package compare_checkpoint

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpoint1 string
	flagCheckpoint2 string
)

var Cmd = &cobra.Command{
	Use:   "compare-checkpoint",
	Short: "Compare two checkpoint files",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagCheckpoint1, "checkpoint1", "",
		"first checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint1")

	Cmd.Flags().StringVar(&flagCheckpoint2, "checkpoint2", "",
		"second checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint2")
}

func run(*cobra.Command, []string) {

	log.Info().Msgf("loading checkpoint %v, %v", flagCheckpoint1, flagCheckpoint2)
	file1, err := os.Open(flagCheckpoint1)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = file1.Close()
	}()

	file2, err := os.Open(flagCheckpoint2)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = file2.Close()
	}()

	err = wal.CompareV5(file1, file2)
	if err != nil {
		log.Fatal().Err(err).Msg("error while loading checkpoint")
	}
}
