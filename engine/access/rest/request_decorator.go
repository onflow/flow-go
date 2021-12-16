package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
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

func (rd *Request) getScriptRequest() (models.GetScriptRequest, error) {
	var getScriptRequest models.GetScriptRequest
	err := getScriptRequest.Build(rd)
	return getScriptRequest, err
}

func (rd *Request) getBlockRequest() (models.GetBlockRequest, error) {
	var getBlockRequest models.GetBlockRequest
	err := getBlockRequest.Build(rd)
	return getBlockRequest, err
}

func (rd *Request) getCollectionRequest() (models.GetCollectionRequest, error) {
	var getCollectionRequest models.GetCollectionRequest
	err := getCollectionRequest.Build(rd)
	return getCollectionRequest, err
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

func (rd *Request) bodyAs(dst interface{}) error {
	//todo(sideninja) validate size

	dec := json.NewDecoder(rd.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			err := fmt.Errorf("request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return NewBadRequestError(err)

		case errors.Is(err, io.ErrUnexpectedEOF):
			err := fmt.Errorf("request body contains badly-formed JSON")
			return NewBadRequestError(err)

		case errors.As(err, &unmarshalTypeError):
			err := fmt.Errorf("request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return NewBadRequestError(err)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			err := fmt.Errorf("Request body contains unknown field %s", fieldName)
			return NewBadRequestError(err)

		case errors.Is(err, io.EOF):
			err := fmt.Errorf("request body must not be empty")
			return NewBadRequestError(err)

		default:
			return err
		}
	}

	if dst == nil {
		return NewBadRequestError(fmt.Errorf("request body must not be empty"))
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		err := fmt.Errorf("request body must only contain a single JSON object")
		return NewBadRequestError(err)
	}

	return nil
}

func (rd *Request) ids() ([]flow.Identifier, error) {
	return toIDs(rd.GetVar("id"))
}

func (rd *Request) id() (flow.Identifier, error) {
	return toID(rd.GetVar("id"))
}
