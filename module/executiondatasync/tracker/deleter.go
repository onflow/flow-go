package tracker

import (
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

type Deleter struct {
	storage *Storage

	blobService network.BlobService
}

func (p *Deleter) delete(ctx irrecoverable.SignalerContext, height uint64) {
	p.storage.ProtectDelete()
	defer p.storage.UnprotectDelete()

	cids, delete, err := p.storage.PrepareForDeletion(height)
	if err != nil {
		// TODO: log error
		// Do we try again?
		// Or do we throw?
	}

	for _, cid := range cids {
		if err := p.blobService.DeleteBlob(ctx, cid); err != nil {
			// TODO: log error
			// Do we throw?
			// TODO: confirm that deleting a non-existing blob is not an error
		}
	}

	if err := delete(); err != nil {
		// TODO: log error
		// Do we throw?
	}
}
