package rest

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
)

// a convenience wrapper around the http request to make it easy to read request params
type requestDecorator struct {
	*http.Request
	expandFields map[string]bool
	selectFields map[string]bool
}

func newRequestDecorator(r *http.Request) *requestDecorator {
	decoratedReq := &requestDecorator{
		Request: r,
	}
	decoratedReq.expandFields, _ = middleware.GetFieldsToExpand(r)
	decoratedReq.selectFields, _ = middleware.GetFieldsToSelect(r)
	return decoratedReq
}

func (rd *requestDecorator) expands(field string) bool {
	return rd.expandFields[field]
}

func (rd *requestDecorator) selects(field string) bool {
	return rd.selectFields[field]
}

func (rd *requestDecorator) getParam(name string) string {
	vars := mux.Vars(rd.Request)
	return vars[name] // todo(sideninja) check if exists
}

func (rd *requestDecorator) ids() ([]flow.Identifier, error) {
	vars := mux.Vars(rd.Request)
	return toIDs(vars["id"])
}

func (rd *requestDecorator) id() (flow.Identifier, error) {
	ids, err := rd.ids()
	if err != nil {
		return flow.Identifier{}, err
	}
	if len(ids) != 1 {
		return flow.Identifier{}, fmt.Errorf("invalid number of IDs")
	}
	return ids[0], nil
}
