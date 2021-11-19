package badger

import (
	"context"
	"time"

	"github.com/dgraph-io/badger/v2/options"
	ds "github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger2"
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

	// Disable compression, since data is already compressed
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

type datastore struct {
	*dsbadger.Datastore
	ttl time.Duration
}

func NewDatastore(opts *Options) (*datastore, error) {
	d, err := dsbadger.NewDatastore(opts.Dir, &opts.Options)
	if err != nil {
		return nil, err
	}

	return &datastore{
		Datastore: d,
		ttl:       opts.TTL,
	}, nil
}

// Put adds data to the datastore using a default TTL if configured
func (d *datastore) Put(ctx context.Context, key ds.Key, value []byte) error {
	if d.ttl == 0 {
		return d.Datastore.Put(ctx, key, value)
	}

	return d.Datastore.PutWithTTL(ctx, key, value, d.ttl)
}
