package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type DKGKeys struct {
	db    *badger.DB
	cache *Cache
}

func NewDKGKeys(collector module.CacheMetrics, db *badger.DB) (*DKGKeys, error) {

	err := operation.EnsureSecretDB(db)
	if err != nil {
		return nil, fmt.Errorf("cannot instantiate key storage in non-secret db: %w", err)
	}

	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		epochCounter := key.(uint64)
		info := val.(*dkg.DKGParticipantPriv)
		return transaction.WithTx(operation.InsertMyDKGPrivateInfo(epochCounter, info))
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		epochCounter := key.(uint64)
		var info dkg.DKGParticipantPriv
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveMyDKGPrivateInfo(epochCounter, &info)(tx)
			return &info, err
		}
	}

	dkgKeys := &DKGKeys{
		db: db,
		cache: newCache(collector,
			metrics.ResourceDKGKey,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return dkgKeys, nil
}

func (k *DKGKeys) storeTx(epochCounter uint64, info *dkg.DKGParticipantPriv) func(tx *transaction.Tx) error {
	return k.cache.PutTx(epochCounter, info)
}

func (k *DKGKeys) retrieveTx(epochCounter uint64) func(tx *badger.Txn) (*dkg.DKGParticipantPriv, error) {
	return func(tx *badger.Txn) (*dkg.DKGParticipantPriv, error) {
		val, err := k.cache.Get(epochCounter)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*dkg.DKGParticipantPriv), nil
	}
}

// InsertMyDKGPrivateInfo insert the private key to database when DKG succeeded.
func (k *DKGKeys) InsertMyDKGPrivateInfo(epochCounter uint64, info *dkg.DKGParticipantPriv) error {
	return operation.RetryOnConflictTx(k.db, transaction.Update, k.storeTx(epochCounter, info))
}

// InsertNoDKGPrivateInfo insert a record in database to indicate that when DKG was completed,
// the node failed DKG, there is no DKG key generated.
func (k *DKGKeys) InsertNoDKGPrivateInfo(epochCounter uint64) error {
	return operation.RetryOnConflictTx(k.db, transaction.Update, k.storeTx(epochCounter, nil))
}

// receive the DKG key for the given Epoch, it returns:
// - (key, true, nil) when DKG was completed, and a DKG private key was saved
// - (nil, false, nil) when DKG was completed, but no DKG private key was saved
// - (nil, false, storage.ErrNotFound) no DKG private key is found in database, it's unknown whether a DKG key was generated
// - (nil, false, error) for other exceptions
func (k *DKGKeys) RetrieveMyDKGPrivateInfo(epochCounter uint64) (*dkg.DKGParticipantPriv, bool, error) {
	tx := k.db.NewTransaction(false)
	defer tx.Discard()
	priv, err := k.retrieveTx(epochCounter)(tx)
	if err != nil {
		return nil, false, err
	}
	if priv == nil {
		return nil, false, nil
	}
	return priv, true, nil
}
