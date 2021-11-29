package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
)

// a convenience wrapper around the http request to make it easy to read request params
type requestDecorator struct {
	*http.Request
	expandFields map[string]bool
	selectFields []string
}

func newRequestDecorator(r *http.Request) *requestDecorator {
	decoratedReq := &requestDecorator{
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

func (rd *requestDecorator) expands(field string) bool {
	return rd.expandFields[field]
}

func (rd *requestDecorator) selects() []string {
	return rd.selectFields
}

func (rd *requestDecorator) getVar(name string) string {
	vars := mux.Vars(rd.Request)
	return vars[name] // todo(sideninja) consider returning err if non-existing
}

func (rd *requestDecorator) getQuery(name string) string {
	return rd.Request.URL.Query().Get(name) // todo(sideninja) consider returning err if non-existing
}

func (rd *requestDecorator) getQueryParam(name string) string {
	return rd.Request.URL.Query().Get(name)
}

func (rd *requestDecorator) getQueryParams(name string) []string {
	param := rd.Request.URL.Query().Get(name)
	// currently, the swagger generated Go REST client is incorrectly doing a `fmt.Sprintf("%v", id)` for the id slice
	// resulting in the client sending the ids in the format [id1 id2 id3...]. This is a temporary workaround to
	// accommodate the client for now. Issue to to fix the client: https://github.com/onflow/flow/issues/698
	param = strings.TrimSuffix(param, "]")
	param = strings.TrimPrefix(param, "[")
	if len(param) == 0 {
		return nil
	}
	params := strings.Split(param, ",")
	return params
}

func (rd *requestDecorator) bodyAs(dst interface{}) error {
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

		case err.Error() == "http: request body too large":
			err := fmt.Errorf("request body must not be larger than 1MB")
			return NewRestError(http.StatusRequestEntityTooLarge, err.Error(), err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		err := fmt.Errorf("request body must only contain a single JSON object")
		return NewBadRequestError(err)
	}

	return nil
}

func (rd *requestDecorator) ids() ([]flow.Identifier, error) {
	return toIDs(rd.getParam("id"))
}

func (rd *requestDecorator) id() (flow.Identifier, error) {
	return toID(rd.getParam("id"))
}
