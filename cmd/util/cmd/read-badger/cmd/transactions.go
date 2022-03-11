package cmd

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagMaxBlockHeight uint64
var flagFixTransactionResultsIndex bool

func init() {
	rootCmd.AddCommand(transactionsCmd)
	rootCmd.AddCommand(transactionsDuplicateCmd)

	transactionsCmd.Flags().StringVarP(&flagTransactionID, "id", "i", "", "the id of the transaction")
	_ = transactionsCmd.MarkFlagRequired("id")

	transactionsDuplicateCmd.Flags().Uint64Var(&flagMaxBlockHeight, "max-height", 0, "maximum block height (skip blocks with height above this value")
	transactionsDuplicateCmd.Flags().BoolVar(&flagFixTransactionResultsIndex, "fix", false, "fix missing entries in (block_id, transaction_index) index, display duplicates otherwise")
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
	Short: "report/fix duplicated transactions",
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

		missingHeights := map[uint64]struct{}{}

		blockNo := 0
		totalTxs := 0

		indexExistingEntries := 0
		indexNewEntries := 0

		hasSystemTx := false

		systemTxID := flow.Identifier{}

		batch := badger.NewBatch(db)

		_, err := headers.FindHeaders(func(header *flow.Header) bool {

			if flagMaxBlockHeight > 0 && header.Height > flagMaxBlockHeight {
				return false
			}

			if !hasSystemTx {

				systemTx, err := blueprints.SystemChunkTransaction(header.ChainID.Chain())
				if err != nil {
					panic(fmt.Sprintf("error while constructing system chunk tx: %s", err))
				}
				systemTxID = systemTx.ID()

				hasSystemTx = true
			}

			blockID := header.ID()

			block, err := storages.Blocks.ByID(blockID)
			if err != nil {
				panic(fmt.Sprintf("header %s if here but not block", blockID.String()))
			}

			missingHeights[block.Header.Height] = struct{}{}

			txIndex := uint32(0)
			for i, guarantee := range block.Payload.Guarantees {
				lightCollection, err := storages.Collections.LightByID(guarantee.CollectionID)
				if err != nil {

					log.Warn().Msgf("cannot get light collection %s for block %s (%d), trying full collection: %s", guarantee.CollectionID, blockID.String(), header.Height, err)

					collection, err := storages.Collections.ByID(guarantee.CollectionID)

					if err != nil {
						panic(fmt.Sprintf("cannot get light and normal collection %s for block %s (%d): %s", guarantee.CollectionID, blockID.String(), header.Height, err))
					}
					light := collection.Light()
					lightCollection = &light
				}

				txs := make([]flow.Identifier, len(lightCollection.Transactions))

				copy(txs, lightCollection.Transactions)

				// add system tx to last collection in block
				if i == len(block.Payload.Guarantees)+1 {
					txs = append(txs, systemTxID)
				}

				for _, txID := range txs {

					if _, has := megaMap[txID]; !has {
						megaMap[txID] = make(map[flow.Identifier]struct{}, 0)
					}

					if _, has := megaMap[txID][blockID]; has {
						panic(fmt.Sprintf("duplicated tx %s in block %s", txID.String(), blockID.String()))
					}

					megaMap[txID][blockID] = struct{}{}

					_, err := storages.TransactionResults.ByBlockIDTransactionIndex(blockID, txIndex)
					if err != nil {
						if errors.Is(err, storage.ErrNotFound) {
							if flagFixTransactionResultsIndex {
								transactionResult, err := storages.TransactionResults.ByBlockIDTransactionID(blockID, txID)
								if err != nil {
									panic(fmt.Sprintf("cannot get transaction results by (block_id, tx_id) (%s, %s): %s", blockID.String(), txID.String(), err))
								}

								result := operation.BatchIndexTransactionResult(blockID, txIndex, transactionResult)
								err = result(batch.GetWriter())
								if err != nil {
									panic(fmt.Sprintf("cannot batch index tx results by  (block_id, tx_index) (%s, %d): %s", blockID.String(), txIndex, err))
								}
							}
							delete(missingHeights, block.Header.Height) // assume existing entries means whole block is mapped for a height
							indexNewEntries++
						} else {
							panic(fmt.Sprintf("error while querying transaction result by (block_id, tx_index) (%s, %d): %s", blockID.String(), txIndex, err))
						}
					} else {
						delete(missingHeights, block.Header.Height) // assume existing entries means whole block is mapped for a height
						indexExistingEntries++
					}

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

		if err != nil {
			log.Error().Err(err).Msgf("could not iterate blocks")
			return
		}

		log.Info().Int("blocks", blockNo).Int("total_tx", totalTxs).Msg("processing progress")

		err = batch.Flush()
		if err != nil {
			log.Error().Err(err).Msg("cannot flush write batch")
		}

		if len(missingHeights) > 0 {
			log.Info().Int("missing_heights_count", len(missingHeights)).Msg("missing block heights")
		} else {
			log.Info().Msg("all heights mapped")
		}

		log.Info().Int("existing_entries", indexExistingEntries).Int("new_entries", indexNewEntries).Msg("index fixed")
	},
}
