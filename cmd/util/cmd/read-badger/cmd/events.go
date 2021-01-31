package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagEventType string

func init() {
	rootCmd.AddCommand(eventsCmd)

	eventsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the events")
	_ = eventsCmd.MarkFlagRequired("block-id")

	// optional flags
	eventsCmd.Flags().StringVarP(&flagEventType, "event-type", "e", "", "the type of event")
	eventsCmd.Flags().StringVarP(&flagTransactionID, "transaction-id", "t", "", "the transaction id of which to query the events")
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Read events from badger",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		if flagEventType != "" && flagTransactionID != "" {
			log.Fatal().Msg("provide only one of --transaction-id or --event-type")
			return
		}

		log.Info().Msgf("got flag block id: %s", flagBlockID)
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block id")
		}

		if flagTransactionID != "" {
			log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
			transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed transaction id")
			}

			log.Info().Msgf("getting events for block id: %v, transaction id: %v", blockID, transactionID)
			events, err := storages.Events.ByBlockIDTransactionID(blockID, transactionID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get events for block id: %v, transaction id: %v", blockID, transactionID)
			}

			for _, event := range events {
				common.PrettyPrint(event)
			}
			return
		}

		if flagEventType != "" {
			validEvents := map[string]bool{
				"flow.AccountCreated": true,
				"flow.AccountUpdated": true,
				"flow.EpochCommit":    true,
				"flow.EpochSetup":     true,
			}
			if _, ok := validEvents[flagEventType]; ok {
				log.Info().Msgf("getting events for block id: %v, event type: %s", blockID, flagEventType)
				events, err := storages.Events.ByBlockIDEventType(blockID, flow.EventType(flagEventType))
				if err != nil {
					log.Fatal().Err(err).Msgf("could not get events for block id: %v, event type: %s", blockID, flagEventType)
				}

				for _, event := range events {
					common.PrettyPrint(event)
				}
				return
			}

			log.Fatal().Msgf("not a valid event type: %s", flagEventType)
			return
		}

		// just fetch events for block
		log.Info().Msgf("getting events for block id: %v", blockID)
		events, err := storages.Events.ByBlockID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get events for block id: %v", blockID)
		}

		for _, event := range events {
			common.PrettyPrint(event)
		}
	},
}
