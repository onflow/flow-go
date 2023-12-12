package operation

import "github.com/dgraph-io/badger/v2"

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
		err := db.Update(func(txn *badger.Txn) error {
			writeBatch := db.NewWriteBatch()
			return batchRemoveByPrefix(makePrefix(prefix))(txn, writeBatch)
		})

		if err != nil {
			return err
		}
	}

	return nil
}
