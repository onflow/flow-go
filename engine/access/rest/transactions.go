package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/onflow/flow-go/engine/access/rest/models"
)

// TransactionByID retrieves a transaction by provided transaction ID.
func (a *APIHandler) TransactionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := models.IDFromRequest(vars["id"])
	if err != nil {
		a.errorResponse(w, http.StatusBadRequest, err.Error(), a.logger) // todo use refactor err func
		return
	}

	tx, err := a.backend.GetTransaction(r.Context(), id)
	if err != nil {
		a.errorResponse(w, http.StatusBadRequest, err.Error(), a.logger)
		return
	}

	res, err := models.TransactionToJSON(tx, nil)
	if err != nil {
		a.errorResponse(w, http.StatusBadRequest, err.Error(), a.logger)
		return
	}

	a.jsonResponse(w, res, a.logger)
}

// NewTransaction creates a new transaction from provided payload.
func (a *APIHandler) NewTransaction(w http.ResponseWriter, r *http.Request) {
	tx, err := models.TransactionFromRequest(r.Body)
	if err != nil {
		a.errorResponse(w, http.StatusBadRequest, err.Error(), a.logger) // todo use refactor err func
		return
	}

	err = a.backend.SendTransaction(r.Context(), tx)
	if err != nil {
		a.errorResponse(w, http.StatusBadRequest, err.Error(), a.logger)
		return
	}

	res, err := models.TransactionToJSON(tx, nil)
	if err != nil {
		a.errorResponse(w, http.StatusBadRequest, err.Error(), a.logger)
		return
	}

	a.jsonResponse(w, res, a.logger)
}
