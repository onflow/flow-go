package network

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
)

// BlobGetter is the common interface shared between blobservice sessions and
// the blobservice.
type BlobGetter interface {
	// GetBlob gets the requested blob.
	GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error)

	// GetBlobs does a batch request for the given cids, returning blobs as
	// they are found, in no particular order.
	//
	// It may not be able to find all requested blobs (or the context may
	// be canceled). In that case, it will close the channel early. It is up
	// to the consumer to detect this situation and keep track which blobs
	// it has received and which it hasn't.
	GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob
}

// BlobService is a hybrid blob datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlobService interface {
	component.Component
	BlobGetter

	// AddBlob puts a given blob to the underlying datastore
	AddBlob(ctx context.Context, b blobs.Blob) error

	// AddBlobs adds a slice of blobs at the same time using batching
	// capabilities of the underlying datastore whenever possible.
	AddBlobs(ctx context.Context, bs []blobs.Blob) error

	// DeleteBlob deletes the given blob from the blobservice.
	DeleteBlob(ctx context.Context, c cid.Cid) error

	// GetSession creates a new session that allows for controlled exchange of wantlists to decrease the bandwidth overhead.
	GetSession(ctx context.Context) BlobGetter

	// TriggerReprovide updates the BlobService's provider entries in the DHT
	TriggerReprovide(ctx context.Context) error
}

type BlobServiceOption func(BlobService)
