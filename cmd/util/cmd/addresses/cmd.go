package addresses

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

var (
	flagChain     string
	flagCount     int
	flagSeparator string
)

var Cmd = &cobra.Command{
	Use:   "addresses",
	Short: "generate addresses for a chain",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().IntVar(&flagCount, "count", 1, "Count")
	_ = Cmd.MarkFlagRequired("count")

	Cmd.Flags().StringVar(&flagSeparator, "separator", ",", "Separator to use between addresses")
}

func run(*cobra.Command, []string) {
	chain := flow.ChainID(flagChain).Chain()

	generator := chain.NewAddressGenerator()

	for i := 0; i < flagCount; i++ {
		address, err := generator.NextAddress()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate address")
		}

		str := address.Hex()

		if i > 0 {
			print(flagSeparator)
		}
		print(str)
	}
}
