package blobs

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipfs/go-ipld-format"
)

type Blobstore interface {
	DeleteBlob(context.Context, cid.Cid) error
	Has(context.Context, cid.Cid) (bool, error)
	Get(context.Context, cid.Cid) (Blob, error)

	// GetSize returns the CIDs mapped BlobSize
	GetSize(context.Context, cid.Cid) (int, error)

	// Put puts a given blob to the underlying datastore
	Put(context.Context, Blob) error

	// PutMany puts a slice of blobs at the same time using batching
	// capabilities of the underlying datastore whenever possible.
	PutMany(context.Context, []Blob) error

	// AllKeysChan returns a channel from which
	// the CIDs in the Blobstore can be read. It should respect
	// the given context, closing the channel if it becomes Done.
	AllKeysChan(ctx context.Context) (<-chan cid.Cid, error)

	// HashOnRead specifies if every read blob should be
	// rehashed to make sure it matches its CID.
	HashOnRead(enabled bool)
}

var ErrNotFound = errors.New("blobstore: blob not found")

type blobstoreImpl struct {
	bs blockstore.Blockstore
}

func NewBlobstore(ds datastore.Batching) *blobstoreImpl {
	return &blobstoreImpl{bs: blockstore.NewBlockstore(ds)}
}

func (bs *blobstoreImpl) DeleteBlob(ctx context.Context, c cid.Cid) error {
	return bs.bs.DeleteBlock(ctx, c)
}

func (bs *blobstoreImpl) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return bs.bs.Has(ctx, c)
}

func (bs *blobstoreImpl) Get(ctx context.Context, c cid.Cid) (Blob, error) {
	blob, err := bs.bs.Get(ctx, c)
	if ipld.IsNotFound(err) {
		return nil, ErrNotFound
	}

	return blob, err
}

func (bs *blobstoreImpl) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	return bs.bs.GetSize(ctx, c)
}

func (bs *blobstoreImpl) Put(ctx context.Context, blob Blob) error {
	return bs.bs.Put(ctx, blob)
}

func (bs *blobstoreImpl) PutMany(ctx context.Context, blobs []Blob) error {
	return bs.bs.PutMany(ctx, blobs)
}

func (bs *blobstoreImpl) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return bs.bs.AllKeysChan(ctx)
}

func (bs *blobstoreImpl) HashOnRead(enabled bool) {
	bs.bs.HashOnRead(enabled)
}

// NoopBlobstore is a Blobstore that does nothing, which is useful for calculating
// BlockExecutionData IDs without storing the data.
type NoopBlobstore struct{}

func NewNoopBlobstore() *NoopBlobstore {
	return &NoopBlobstore{}
}

func (n *NoopBlobstore) DeleteBlob(context.Context, cid.Cid) error {
	return nil
}

func (n *NoopBlobstore) Has(context.Context, cid.Cid) (bool, error) {
	return false, nil
}

func (n *NoopBlobstore) Get(context.Context, cid.Cid) (Blob, error) {
	return nil, nil
}

func (n *NoopBlobstore) GetSize(context.Context, cid.Cid) (int, error) {
	return 0, nil
}

func (n *NoopBlobstore) Put(context.Context, Blob) error {
	return nil
}

func (n *NoopBlobstore) PutMany(context.Context, []Blob) error {
	return nil
}

func (n *NoopBlobstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, nil
}

func (n *NoopBlobstore) HashOnRead(enabled bool) {}
