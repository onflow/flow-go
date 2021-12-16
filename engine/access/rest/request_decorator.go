package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"net/http"
)

// decorateRequest takes http request and applies functions to produce our custom
// request object decorated with values we need
func decorateRequest(r *http.Request) *request.Request {
	decoratedReq := &request.Request{
		Request: r,
	}

	if expandFields, found := middleware.GetFieldsToExpand(r); found {
		decoratedReq.ExpandFields = sliceToMap(expandFields)
	}

	if selectFields, found := middleware.GetFieldsToSelect(r); found {
		decoratedReq.SelectFields = selectFields
	}

	return decoratedReq
}

func sliceToMap(values []string) map[string]bool {
	valueMap := make(map[string]bool, len(values))
	for _, v := range values {
		valueMap[v] = true
	}
	return valueMap
}
