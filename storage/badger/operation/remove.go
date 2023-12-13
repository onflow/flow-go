package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"
)

func RemoveAll(db *badger.DB) error {
	prefixes := []byte{
		codeExecutedBlock,
		codeIndexExecutionResultByBlock,
		codeCommit,
		codeExecutionStateInteractions,
		codeChunkDataPack,
		codeEvent,
		codeTransactionResult,
		codeTransactionResultIndex,
		codeExecutionResult,
		codeExecutionReceiptMeta,
		codeExecutedBlock,
		codeComputationResults,
		codeServiceEvent,
	}

	for _, prefix := range prefixes {
		log.Info().Msgf("start removing: %v", prefix)
		err := db.Update(func(txn *badger.Txn) error {
			writeBatch := db.NewWriteBatch()
			return batchRemoveByPrefix(makePrefix(prefix))(txn, writeBatch)
		})

		if err != nil {
			return err
		}
		log.Info().Msgf("finish removing: %v", prefix)
	}

	return nil
}
