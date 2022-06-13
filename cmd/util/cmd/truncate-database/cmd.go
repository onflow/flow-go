package truncate_database

import (
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagDatadir string
	flagFlatten bool
)

var Cmd = &cobra.Command{
	Use:   "truncate-database",
	Short: "Truncates protocol state database (Possible data loss!)",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().BoolVar(&flagFlatten, "flatten", false, "flatten the database")

}

func run(*cobra.Command, []string) {

	log.Info().Msg("Opening database with truncate")

	db := common.InitStorageWithTruncate(flagDatadir, true)
	defer db.Close()

	if flagFlatten {
		err := db.Flatten(8)
		if err != nil {
			log.Error().Err(err).Msg("Error while flattening DB")
			return
		}
	}

	log.Info().Msg("Truncated")
}
