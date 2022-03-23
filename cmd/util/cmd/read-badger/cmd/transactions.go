package cmd

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/storage/badger"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(transactionsCmd)
	rootCmd.AddCommand(transactionsDuplicateCmd)

	transactionsCmd.Flags().StringVarP(&flagTransactionID, "id", "i", "", "the id of the transaction")
	_ = transactionsCmd.MarkFlagRequired("id")
}

var transactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "get transaction by ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
		transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
		if err != nil {
			log.Error().Err(err).Msg("malformed transaction id")
			return
		}

		log.Info().Msgf("getting transaction by id: %v", transactionID)
		tx, err := storages.Transactions.ByID(transactionID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get transaction with id: %v", transactionID)
			return
		}

		common.PrettyPrintEntity(tx)
	},
}

var transactionsDuplicateCmd = &cobra.Command{
	Use:   "transactions-duplicates",
	Short: "report duplicated transactions",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		headers, ok := storages.Headers.(*badger.Headers)
		if !ok {
			log.Error().Msg("unsupported headers type")
			return
		}

		// txID -> list of blocks
		megaMap := map[flow.Identifier]map[flow.Identifier]struct{}{}

		blockNo := 0
		totalTxs := 0

		_, err := headers.FindHeaders(func(header *flow.Header) bool {

			blockID := header.ID()

			block, err := storages.Blocks.ByID(blockID)
			if err != nil {
				panic(fmt.Sprintf("header %s if here but not block", blockID.String()))
			}
			txIndex := 0
			for _, guarantee := range block.Payload.Guarantees {
				lightCollection, err := storages.Collections.LightByID(guarantee.CollectionID)
				if err != nil {
					panic(fmt.Sprintf("cannot get light collection %s %s", guarantee.CollectionID, err))
				}
				for _, txID := range lightCollection.Transactions {

					if _, has := megaMap[txID]; !has {
						megaMap[txID] = make(map[flow.Identifier]struct{}, 0)
					}

					if _, has := megaMap[txID][blockID]; has {
						panic(fmt.Sprintf("duplicated tx %s in block %s", txID.String(), blockID.String()))
					}

					megaMap[txID][blockID] = struct{}{}

					totalTxs++
					txIndex++
				}
			}

			blockNo++

			if blockNo%10000 == 0 {
				log.Info().Int("blocks", blockNo).Int("total_tx", totalTxs).Msg("processing progress")
			}

			return false
		})

		for txID := range megaMap {
			l := len(megaMap[txID])
			if l > 1 {
				blockIDs := make([]string, 0, l)
				for blockID := range megaMap[txID] {
					blockIDs = append(blockIDs, blockID.String())
				}
				log.Info().Str("tx_id", txID.String()).Msgf("transaction duplicated in different %d blocks: %s", l, strings.Join(blockIDs, ", "))
			}
		}

		if err != nil {
			log.Error().Err(err).Msgf("could not iterate blocks")
			return
		}

	},
}
