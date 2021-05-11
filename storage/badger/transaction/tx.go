package transaction

import (
	dbbadger "github.com/dgraph-io/badger/v2"
)

type Tx struct {
	DBTxn     *dbbadger.Txn
	callbacks []func()
}

// OnSucceed adds a callback to execute after the batch has
// been successfully flushed.
// useful for implementing the cache where we will only cache
// after the batch has been successfully flushed
func (b *Tx) OnSucceed(callback func()) {
	b.callbacks = append(b.callbacks, callback)
}

func Update(db *dbbadger.DB, f func(*Tx) error) error {
	dbTxn := db.NewTransaction(true)
	defer dbTxn.Discard()

	tx := &Tx{DBTxn: dbTxn}
	err := f(tx)
	if err != nil {
		return err
	}

	err = dbTxn.Commit()
	if err != nil {
		return err
	}

	for _, callback := range tx.callbacks {
		callback()
	}
	return nil
}
