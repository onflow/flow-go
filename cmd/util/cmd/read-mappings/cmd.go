package read_mappings

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagDatadir      string
	flagMappingsFile string
)

var Cmd = &cobra.Command{
	Use:   "read-mappings",
	Short: "Read mapping from Protocol database and saves them to a file",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagMappingsFile, "mappings-file", "",
		"file to write mappings to")
	_ = Cmd.MarkFlagRequired("mappings-file")
}

func run(*cobra.Command, []string) {

	db := common.InitStorage(flagDatadir)
	defer db.Close()

	mappings, err := getMappingsFromDatabase(db)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get mapping for a database")
	}

	err = WriteMegamappings(mappings, flagMappingsFile)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot save mappings to a file")
	}

}
