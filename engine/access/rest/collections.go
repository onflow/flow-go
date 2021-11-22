package rest

import (
	"github.com/onflow/flow-go/access"
)

func getCollectionByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	collection, err := backend.GetCollectionByID(r.Context(), id)
	if err != nil {
		return nil, NewBadRequestError("transaction fetching error", err)
	}

	return collectionResponse(collection), nil
}
