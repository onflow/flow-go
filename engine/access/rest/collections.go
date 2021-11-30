package rest

import (
	"github.com/onflow/flow-go/access"
)

func getCollectionByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	collection, err := backend.GetCollectionByID(r.Context(), id)
	if err != nil {
		return nil, err
	}

	return collectionResponse(collection, link), nil
}
