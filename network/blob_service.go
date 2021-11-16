package network

import (
	"context"
	"io"

	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	blocks "github.com/ipfs/go-block-format"
	blockservice "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
)

type Blob = blocks.Block

var NewBlob = blocks.NewBlock

// BlobGetter is the common interface shared between blobservice sessions and
// the blobservice.
type BlobGetter interface {
	// GetBlob gets the requested blob.
	GetBlob(ctx context.Context, c cid.Cid) (Blob, error)

	// GetBlobs does a batch request for the given cids, returning blobs as
	// they are found, in no particular order.
	//
	// It may not be able to find all requested blobs (or the context may
	// be canceled). In that case, it will close the channel early. It is up
	// to the consumer to detect this situation and keep track which blobs
	// it has received and which it hasn't.
	GetBlobs(ctx context.Context, ks ...cid.Cid) <-chan Blob
}

// BlobService is a hybrid blob datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlobService interface {
	io.Closer
	BlobGetter

	// AddBlob puts a given blob to the underlying datastore
	AddBlob(ctx context.Context, b Blob) error

	// AddBlobs adds a slice of blobs at the same time using batching
	// capabilities of the underlying datastore whenever possible.
	AddBlobs(ctx context.Context, bs ...Blob) error

	// DeleteBlob deletes the given blob from the blobservice.
	DeleteBlob(ctx context.Context, c cid.Cid) error

	// GetSession creates a new session that allows for controlled exchange of wantlists to decrease the bandwidth overhead.
	GetSession(ctx context.Context) BlobGetter
}

type blobService struct {
	blockService blockservice.BlockService
}

var _ BlobService = (*blobService)(nil)

func NewBlobService(
	parent context.Context,
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	ds datastore.Batching,
) BlobService {
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))
	bstore := blockstore.NewBlockstore(ds)

	return &blobService{
		blockService: blockservice.New(bstore, bitswap.New(parent, bsNetwork, bstore)),
	}
}

func (bs *blobService) GetBlob(ctx context.Context, c cid.Cid) (Blob, error) {
	return bs.blockService.GetBlock(ctx, c)
}

func (bs *blobService) GetBlobs(ctx context.Context, ks ...cid.Cid) <-chan Blob {
	return bs.blockService.GetBlocks(ctx, ks)
}

func (bs *blobService) AddBlob(ctx context.Context, b Blob) error {
	return bs.blockService.AddBlock(ctx, b)
}

func (bs *blobService) AddBlobs(ctx context.Context, blobs ...Blob) error {
	return bs.blockService.AddBlocks(ctx, blobs)
}

func (bs *blobService) DeleteBlob(ctx context.Context, c cid.Cid) error {
	return bs.blockService.DeleteBlock(ctx, c)
}

func (bs *blobService) Close() error {
	return bs.blockService.Close()
}

func (bs *blobService) GetSession(ctx context.Context) BlobGetter {
	return &blobServiceSession{blockservice.NewSession(ctx, bs.blockService)}
}

type blobServiceSession struct {
	session *blockservice.Session
}

var _ BlobGetter = (*blobServiceSession)(nil)

func (s *blobServiceSession) GetBlob(ctx context.Context, c cid.Cid) (Blob, error) {
	return s.session.GetBlock(ctx, c)
}

func (s *blobServiceSession) GetBlobs(ctx context.Context, ks ...cid.Cid) <-chan Blob {
	return s.session.GetBlocks(ctx, ks)
}
