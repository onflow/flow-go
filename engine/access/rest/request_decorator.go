package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/request"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/middleware"
)

// Request a convenience wrapper around the http request to make it easy to read request query params
type Request struct {
	*http.Request
	expandFields map[string]bool
	selectFields []string
}

// decorateRequest takes http request and applies functions to produce our custom
// request object decorated with values we need
func decorateRequest(r *http.Request) *Request {
	decoratedReq := &Request{
		Request: r,
	}

	if expandFields, found := middleware.GetFieldsToExpand(r); found {
		decoratedReq.expandFields = sliceToMap(expandFields)
	}

	if selectFields, found := middleware.GetFieldsToSelect(r); found {
		decoratedReq.selectFields = selectFields
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

func (rd *Request) getScriptRequest() (request.GetScript, error) {
	var req request.GetScript
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getBlockRequest() (request.GetBlock, error) {
	var req request.GetBlock
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getCollectionRequest() (request.GetCollection, error) {
	var req request.GetCollection
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getAccountRequest() (request.GetAccount, error) {
	var req request.GetAccount
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getExecutionResultByBlockIDsRequest() (request.GetExecutionResultByBlockIDs, error) {
	var req request.GetExecutionResultByBlockIDs
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getExecutionResultRequest() (request.GetExecutionResult, error) {
	var req request.GetExecutionResult
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getTransactionRequest() (request.GetTransaction, error) {
	var req request.GetTransaction
	err := req.Build(rd)
	return req, err
}

func (rd *Request) getTransactionResultRequest() (request.GetTransactionResult, error) {
	var req request.GetTransactionResult
	err := req.Build(rd)
	return req, err
}

func (rd *Request) createTransactionRequest() (request.CreateTransaction, error) {
	var req request.CreateTransaction
	err := req.Build(rd)
	return req, err
}

func (rd *Request) Expands(field string) bool {
	return rd.expandFields[field]
}

func (rd *Request) selects() []string {
	return rd.selectFields
}

func (rd *Request) GetVar(name string) string {
	vars := mux.Vars(rd.Request)
	return vars[name] // todo(sideninja) consider returning err if non-existing
}

func (rd *Request) GetQueryParam(name string) string {
	return rd.Request.URL.Query().Get(name) // todo(sideninja) consider returning err if non-existing
}

func (rd *Request) GetQueryParams(name string) []string {
	param := rd.Request.URL.Query().Get(name)
	// currently, the swagger generated Go REST client is incorrectly doing a `fmt.Sprintf("%v", id)` for the id slice
	// resulting in the client sending the ids in the format [id1 id2 id3...]. This is a temporary workaround to
	// accommodate the client for now by doing a strings.Fields if commas are not present.
	// Issue to to fix the client: https://github.com/onflow/flow/issues/698
	param = strings.TrimSuffix(param, "]")
	param = strings.TrimPrefix(param, "[")
	if len(param) == 0 {
		return nil
	}
	if strings.Contains(param, ",") {
		return strings.Split(param, ",")
	}
	return strings.Fields(param)
}
