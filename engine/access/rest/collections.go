package rest

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
)

func getCollectionByID(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	id, err := toID(vars["id"])
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	collection, err := backend.GetCollectionByID(r.Context(), id)
	if err != nil {
		return nil, NewBadRequestError("transaction fetching error", err)
	}

	return collectionResponse(collection), nil
}
