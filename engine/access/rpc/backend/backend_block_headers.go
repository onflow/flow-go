package backend

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockHeaders struct {
	headers storage.Headers
	state   protocol.State
}

func (b *backendBlockHeaders) GetLatestBlockHeader(_ context.Context, isSealed bool) (*flow.Header, error) {
	var header *flow.Header
	var err error

	if isSealed {
		// get the latest seal header from storage
		header, err = b.state.Sealed().Head()
	} else {
		// get the finalized header from state
		header, err = b.state.Final().Head()
	}

	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return header, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByID(_ context.Context, id flow.Identifier) (*flow.Header, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return header, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flow.Header, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return header, nil
}
