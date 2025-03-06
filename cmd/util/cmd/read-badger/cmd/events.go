package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
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
	Short: "Read events",
	Run: func(cmd *cobra.Command, args []string) {
		err := WithStorage(func(db storage.DB) error {
			events := store.NewEvents(metrics.NewNoopCollector(), db)

			if flagEventType != "" && flagTransactionID != "" {
				return fmt.Errorf("provide only one of --transaction-id or --event-type")
			}

			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block id: %w", err)
			}

			if flagTransactionID != "" {
				log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
				transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
				if err != nil {
					return fmt.Errorf("malformed traansaction id: %w", err)
				}

				log.Info().Msgf("getting events for block id: %v, transaction id: %v", blockID, transactionID)
				events, err := events.ByBlockIDTransactionID(blockID, transactionID)
				if err != nil {
					return fmt.Errorf("could not get events for block id: %v, transaction id: %v: %w", blockID, transactionID, err)
				}

				for _, event := range events {
					common.PrettyPrint(event)
				}
				return nil
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
					events, err := events.ByBlockIDEventType(blockID, flow.EventType(flagEventType))
					if err != nil {
						return fmt.Errorf("could not get events for block id: %v, event type: %s, %w", blockID, flagEventType, err)
					}

					for _, event := range events {
						common.PrettyPrint(event)
					}
					return nil
				}

				return fmt.Errorf("not a valid event type: %s", flagEventType)
			}

			// just fetch events for block
			log.Info().Msgf("getting events for block id: %v", blockID)
			evts, err := events.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("could not get events for block id: %v: %w", blockID, err)
			}

			for _, event := range evts {
				common.PrettyPrint(event)
			}

			return nil
		})

		if err != nil {
			log.Error().Err(err).Msg("could not get events")
		}
	},
}
