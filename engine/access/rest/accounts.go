package rest

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
)

func getAccount(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	address, err := toAddress(vars["address"])
	if err != nil {
		return nil, NewBadRequestError("invalid address", err)
	}

	account, err := backend.GetAccount(r.Context(), address)
	if err != nil {
		return nil, NewBadRequestError("account fetching error", err)
	}

	return accountResponse(account), nil
}
