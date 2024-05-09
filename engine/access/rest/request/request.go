package request

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
)

// Request a convenience wrapper around the http request to make it easy to read request query params
type Request struct {
	*http.Request
	ExpandFields map[string]bool
	selectFields []string
	Chain        flow.Chain
}

func (rd *Request) GetScriptRequest() (GetScript, error) {
	var req GetScript
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetBlockRequest() (GetBlock, error) {
	var req GetBlock
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetBlockByIDsRequest() (GetBlockByIDs, error) {
	var req GetBlockByIDs
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetBlockPayloadRequest() (GetBlockPayload, error) {
	var req GetBlockPayload
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetCollectionRequest() (GetCollection, error) {
	var req GetCollection
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetAccountRequest() (GetAccount, error) {
	var req GetAccount
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetAccountKeyRequest() (GetAccountKey, error) {
	var req GetAccountKey
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetExecutionResultByBlockIDsRequest() (GetExecutionResultByBlockIDs, error) {
	var req GetExecutionResultByBlockIDs
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetExecutionResultRequest() (GetExecutionResult, error) {
	var req GetExecutionResult
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetTransactionRequest() (GetTransaction, error) {
	var req GetTransaction
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetTransactionResultRequest() (GetTransactionResult, error) {
	var req GetTransactionResult
	err := req.Build(rd)
	return req, err
}

func (rd *Request) GetEventsRequest() (GetEvents, error) {
	var req GetEvents
	err := req.Build(rd)
	return req, err
}

func (rd *Request) CreateTransactionRequest() (CreateTransaction, error) {
	var req CreateTransaction
	err := req.Build(rd)
	return req, err
}

func (rd *Request) SubscribeEventsRequest() (SubscribeEvents, error) {
	var req SubscribeEvents
	err := req.Build(rd)
	return req, err
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
		decoratedReq.ExpandFields = sliceToMap(expandFields)
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
