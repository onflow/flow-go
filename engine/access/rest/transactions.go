package rest

import (
	"github.com/gorilla/mux"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"net/http"
)

// TransactionByID retrieves a transaction by provided transaction ID.
func (a *APIHandler) TransactionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := models.IDFromRequest(vars["id"])
	if err != nil {
		return
	}

	tx, err := a.backend.GetTransaction(r.Context(), id)
	if err != nil {
		return
	}

	res, err := models.TransactionToJSON(tx, nil)
	if err != nil {
		return
	}

	a.jsonResponse(w, res, a.logger)
}

// NewTransaction creates a new transaction from provided payload.
func (a *APIHandler) NewTransaction(w http.ResponseWriter, r *http.Request) {
	tx, err := models.TransactionFromRequest(r.Body)
	if err != nil {
		return
	}

	err = a.backend.SendTransaction(r.Context(), tx)
	if err != nil {
		return
	}

	res, err := models.TransactionToJSON(tx, nil)
	if err != nil {
		return
	}

	a.jsonResponse(w, res, a.logger)
}
