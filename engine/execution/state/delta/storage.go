package delta

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

type Storage interface {
	GetRegister(key flow.RegisterID) (value flow.RegisterValue, exists bool, err error)
	CommitBlockDelta(blockHeight uint64, delta Delta) error
	Bootstrap(blockHeight uint64, registers []flow.RegisterEntry) error
	BlockHeight() (uint64, error)
}

// this is conflict free key
var BadgerStoreHeightKey = []byte("height")

const BadgerStoreBootstrapBatchSize = 100

type BadgerStore struct {
	db *badger.DB
}

var _ Storage = &BadgerStore{}

func NewBadgerStore(db *badger.DB) (*BadgerStore, error) {
	store := &BadgerStore{db: db}
	err := store.initHeight()
	return store, err
}

func (s *BadgerStore) GetRegister(key flow.RegisterID) (val flow.RegisterValue, found bool, err error) {
	err = s.db.View(
		func(tx *badger.Txn) error {
			k := []byte(key.String())
			item, err := tx.Get(k)
			if errors.Is(err, badger.ErrKeyNotFound) {
				found = false
				return nil
			}
			if err != nil {
				found = false
				return fmt.Errorf("could not get data: %w", err)
			}
			_ = item.Value(func(v []byte) error {
				found = true
				val = flow.RegisterValue(v)
				return nil
			})
			return nil
		},
	)
	return
}

func (s *BadgerStore) BlockHeight() (height uint64, err error) {
	err = s.db.View(
		func(tx *badger.Txn) error {
			item, err := tx.Get(BadgerStoreHeightKey)
			if err != nil {
				return fmt.Errorf("could not get data: %w", err)
			}
			err = item.Value(func(v []byte) error {
				height = uint64(binary.BigEndian.Uint64(v))
				return nil
			})
			return err
		},
	)
	return
}

func (s *BadgerStore) initHeight() error {
	err := s.db.Update(
		func(tx *badger.Txn) error {
			var err error
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, 0)
			err = tx.Set(BadgerStoreHeightKey, buf)
			if err != nil {
				return fmt.Errorf("could not init height: %w", err)
			}
			return nil
		},
	)
	return err
}

func (s *BadgerStore) CommitBlockDelta(blockHeight uint64, delta Delta) error {
	err := s.db.Update(
		// TODO deal with when tx batch size grows big
		func(tx *badger.Txn) error {
			var err error
			for key, value := range delta.Data {
				k := []byte(key.String())
				if len(value) == 0 {
					err = tx.Delete(k)
					if err != nil {
						return fmt.Errorf("could not delete data: %w", err)
					}
					continue
				}
				err = tx.Set(k, value[:])
				if err != nil {
					return fmt.Errorf("could not set data: %w", err)
				}
			}

			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, blockHeight)
			err = tx.Set(BadgerStoreHeightKey, buf)
			if err != nil {
				return fmt.Errorf("could not update latest height: %w", err)
			}

			return nil
		},
	)
	return err
}

func (s *BadgerStore) Bootstrap(blockHeight uint64, registers []flow.RegisterEntry) error {
	// TODO deal with when tx batch size grows big
	err := s.db.Update(
		func(tx *badger.Txn) error {
			var err error
			for _, reg := range registers {
				k := []byte(reg.Key.String())
				if len(reg.Value) == 0 {
					err = tx.Delete(k)
					if err != nil {
						return fmt.Errorf("could not delete data: %w", err)
					}
					continue
				}
				err = tx.Set(k, reg.Value[:])
				if err != nil {
					return fmt.Errorf("could not set data: %w", err)
				}
			}

			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, blockHeight)
			err = tx.Set(BadgerStoreHeightKey, buf)
			if err != nil {
				return fmt.Errorf("could not update latest height: %w", err)
			}

			return nil
		},
	)
	return err
}
