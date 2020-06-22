package unittest

import (
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/model/flow"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

// HeadersFromMap creates a storage header mock that backed by a given map
func HeadersFromMap(headerDB map[flow.Identifier]*flow.Header) *storage.Headers {
	headers := &storage.Headers{}
	headers.On("Store", mock.Anything).Return(
		func(header *flow.Header) error {
			headerDB[header.ID()] = header
			return nil
		},
	)
	headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return headerDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := headerDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	return headers
}
