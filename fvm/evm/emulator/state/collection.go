package state

import (
	"github.com/onflow/atree"
)

type CollectionProvider struct {
	rootAddr atree.Address
	storage  *atree.PersistentSlabStorage
}

func NewCollectionProvider(
	rootAddr atree.Address,
	storage *atree.PersistentSlabStorage,
) *CollectionProvider {
	return &CollectionProvider{
		rootAddr: rootAddr,
		storage:  storage,
	}
}

func (cp *CollectionProvider) GetCollection(storageIDBytes []byte) (*Collection, error) {
	storageID, err := atree.NewStorageIDFromRawBytes(storageIDBytes)
	if err != nil {
		return nil, err
	}
	omap, err := atree.NewMapWithRootID(cp.storage, storageID, atree.NewDefaultDigesterBuilder())
	if err != nil {
		return nil, err
	}
	return &Collection{
		omap:           omap,
		storage:        cp.storage,
		storageIDBytes: storageIDBytes,
	}, nil
}

func (cp *CollectionProvider) NewCollection() (*Collection, error) {
	omap, err := atree.NewMap(cp.storage, cp.rootAddr, atree.NewDefaultDigesterBuilder(), emptyTypeInfo{})
	storageIDBytes := make([]byte, StorageIDSize)
	_, err = omap.StorageID().ToRawBytes(storageIDBytes)
	if err != nil {
		return nil, err
	}
	return &Collection{
		storage:        cp.storage,
		omap:           omap,
		storageIDBytes: storageIDBytes,
	}, nil
}

type Collection struct {
	omap           *atree.OrderedMap
	storage        *atree.PersistentSlabStorage
	storageIDBytes []byte
}

func (c *Collection) StorageIDBytes() []byte {
	return c.storageIDBytes
}

func (c *Collection) Get(key []byte) ([]byte, error) {
	// first check if we have the key
	// this avoids getting a NotFound error
	found, err := c.omap.Has(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	data, err := c.omap.Get(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		return nil, err
	}

	value, err := data.StoredValue(c.omap.Storage)
	if err != nil {
		return nil, err
	}

	return value.(ByteStringValue).Bytes(), nil
}

func (c *Collection) Set(key, value []byte) error {
	existingValueStorable, err := c.omap.Set(compare, hashInputProvider, NewByteStringValue(key), NewByteStringValue(value))
	if err != nil {
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := c.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) Remove(key []byte) error {
	_, existingValueStorable, err := c.omap.Remove(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := c.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) Destroy() error {
	// TODO: implement deep remove
	return nil
}
