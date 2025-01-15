package common

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/common/middleware"
	"github.com/onflow/flow-go/model/flow"
)

// Request a convenience wrapper around the http request to make it easy to read request query params
type Request struct {
	*http.Request
	ExpandFields map[string]bool
	selectFields []string
	Chain        flow.Chain
}

func (rd *Request) Expands(field string) bool {
	return rd.ExpandFields[field]
}

func (rd *Request) Selects() []string {
	return rd.selectFields
}

func (rd *Request) GetVar(name string) string {
	vars := mux.Vars(rd.Request)
	return vars[name]
}

func (rd *Request) GetVars(name string) []string {
	vars := mux.Vars(rd.Request)
	return toStringArray(vars[name])
}

func (rd *Request) GetQueryParam(name string) string {
	return rd.Request.URL.Query().Get(name)
}

func (rd *Request) GetQueryParams(name string) []string {
	param := rd.Request.URL.Query().Get(name)
	return toStringArray(param)
}

// Decorate takes http request and applies functions to produce our custom
// request object decorated with values we need
func Decorate(r *http.Request, chain flow.Chain) *Request {
	decoratedReq := &Request{
		Request: r,
		Chain:   chain,
	}

	if expandFields, found := middleware.GetFieldsToExpand(r); found {
		decoratedReq.ExpandFields = SliceToMap(expandFields)
	}

	if selectFields, found := middleware.GetFieldsToSelect(r); found {
		decoratedReq.selectFields = selectFields
	}

	return decoratedReq
}

func toStringArray(in string) []string {
	// currently, the swagger generated Go REST client is incorrectly doing a `fmt.Sprintf("%v", id)` for the id slice
	// resulting in the client sending the ids in the format [id1 id2 id3...]. This is a temporary workaround to
	// accommodate the client for now by doing a strings.Fields if commas are not present.
	// Issue to to fix the client: https://github.com/onflow/flow/issues/698
	in = strings.TrimSuffix(in, "]")
	in = strings.TrimPrefix(in, "[")
	var out []string

	if len(in) == 0 {
		return []string{}
	}

	if strings.Contains(in, ",") {
		out = strings.Split(in, ",")
	} else {
		out = strings.Fields(in)
	}

	return out
}
