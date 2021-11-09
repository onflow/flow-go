package badger

import (
	"time"

	"github.com/dgraph-io/badger/v2/options"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger2"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
)

type Options struct {
	dsbadger.Options

	// TTL to set for each block
	TTL time.Duration
}

func DefaultOptions(path string) *Options {
	opts := Options{dsbadger.DefaultOptions, 0}

	// These are overridden in NewDatastore, but included here for a consistent interface
	opts.Dir = path
	opts.ValueDir = path

	// Don't sync writes to disk immediately, wait until MaxTableSize is reached or db is close
	// This means that if the node crashes, some data will be lost. Since the blockstore is a
	// effectively a local cache of shared data, any blocks lost in a crash will be resynced on boot
	opts.SyncWrites = false

	// Disable compression, since the blocks are already compressed
	opts.Compression = options.None

	return &opts
}

// WithTTL returns a new Options value with WithTTL set to the given value.
//
// TTL sets the expiration time for all newly added blocks. After expiration, the blocks will no
// longer be retrievable and will be removed by garbage collection.
//
// The default value is 0, which means no TTL.
func (o *Options) WithTTL(ttl time.Duration) *Options {
	o.TTL = ttl
	return o
}

type blockstore struct {
	bstore.Blockstore
	ds  *dsbadger.Datastore
	ttl time.Duration
}

func NewBlockstore(opts *Options) (*blockstore, error) {
	d, err := dsbadger.NewDatastore(opts.Dir, &opts.Options)
	if err != nil {
		return nil, err
	}

	return &blockstore{
		Blockstore: bstore.NewBlockstore(d),
		ds:         d,
		ttl:        opts.TTL,
	}, nil
}

// Put adds a block to the blockstore
func (bs *blockstore) Put(block blocks.Block) error {
	if bs.ttl == 0 {
		return bs.Blockstore.Put(block)
	}

	k := cidToDsKeyWithPrefix(block.Cid())
	if exists, err := bs.ds.Has(k); err == nil && exists {
		return nil // already stored.
	}
	return bs.ds.PutWithTTL(k, block.RawData(), bs.ttl)
}

// PutMany adds multiple blocks to the blockstore at once
func (bs *blockstore) PutMany(blocks []blocks.Block) error {
	if bs.ttl == 0 {
		return bs.Blockstore.PutMany(blocks)
	}

	t, err := bs.ds.Batch()
	if err != nil {
		return err
	}

	for _, b := range blocks {
		k := cidToDsKeyWithPrefix(b.Cid())
		if exists, err := bs.ds.Has(k); err == nil && exists {
			continue
		}

		if err = bs.ds.PutWithTTL(k, b.RawData(), bs.ttl); err != nil {
			return err
		}
	}
	return t.Commit()
}

// The ipfs blockstore implementation adds a prefix to all keys, which is wrapped within
// a layer of abstraction in the library. In order to add TTLs to block entries, we need
// to explicitly add the prefix when interacting with the datastore directly.
func cidToDsKeyWithPrefix(k cid.Cid) ds.Key {
	prefix := bstore.BlockPrefix.String()
	key := dshelp.CidToDsKey(k).String()
	return ds.KeyWithNamespaces([]string{prefix, key})
}
