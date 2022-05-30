package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/storage/badger/operation"
)

func init() {
	rootCmd.AddCommand(dropClusterRefHeightIndexCmd)

}

var dropClusterRefHeightIndexCmd = &cobra.Command{
	Use:   "drop-cluster-ref-height-index",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		_, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("dropping cluster block ref height index")
		err := db.Update(operation.DropClusterBlockByReferenceHeightIndex())
		if err != nil {
			log.Error().Err(err).Msgf("could not drop index")
			return
		}
		log.Info().Msgf("successfully dropped cluster block ref height index")
	},
}
