package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/encodable"
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
		info := val.(*encodable.RandomBeaconPrivKey)
		return transaction.WithTx(operation.InsertMyBeaconPrivateKey(epochCounter, info))
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		epochCounter := key.(uint64)
		var info encodable.RandomBeaconPrivKey
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveMyBeaconPrivateKey(epochCounter, &info)(tx)
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

func (k *DKGKeys) storeTx(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(tx *transaction.Tx) error {
	return k.cache.PutTx(epochCounter, info)
}

func (k *DKGKeys) retrieveTx(epochCounter uint64) func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
	return func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
		val, err := k.cache.Get(epochCounter)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*encodable.RandomBeaconPrivKey), nil
	}
}

func (k *DKGKeys) InsertMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) error {
	return operation.RetryOnConflictTx(k.db, transaction.Update, k.storeTx(epochCounter, info))
}

func (k *DKGKeys) RetrieveMyBeaconPrivateKey(epochCounter uint64) (*encodable.RandomBeaconPrivKey, error) {
	tx := k.db.NewTransaction(false)
	defer tx.Discard()
	return k.retrieveTx(epochCounter)(tx)
}
