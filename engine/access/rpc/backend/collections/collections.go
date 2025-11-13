package collections

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Collections exposes collection-related Access API endpoints backed by the node's collection storage.
type Collections struct {
	collections storage.Collections
}

func NewCollectionsBackend(collections storage.Collections) *Collections {
	return &Collections{
		collections: collections,
	}
}

// GetCollectionByID returns a light collection by its ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: if the collection is not found.
func (c *Collections) GetCollectionByID(ctx context.Context, colID flow.Identifier) (*flow.LightCollection, error) {
	col, err := c.collections.LightByID(colID)
	if err != nil {
		// Collections are retrieved asynchronously as we finalize blocks, so it is possible to get
		// a storage.ErrNotFound for a collection within a finalized block. Clients should retry.
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("collection", fmt.Errorf("please retry for collection in finalized block: %w", err))
	}

	return col, nil
}

// GetFullCollectionByID returns a full collection by its ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: if the collection is not found.
func (c *Collections) GetFullCollectionByID(ctx context.Context, colID flow.Identifier) (*flow.Collection, error) {
	col, err := c.collections.ByID(colID)
	if err != nil {
		// Collections are retrieved asynchronously as we finalize blocks, so it is possible to get
		// a storage.ErrNotFound for a collection within a finalized block. Clients should retry.
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("collection", fmt.Errorf("please retry for collection in finalized block: %w", err))
	}

	return col, nil
}
