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

// a convenience wrapper around the http request to make it easy to read request query params
type request struct {
	*http.Request
	expandFields map[string]bool
	selectFields []string
}

// decorateRequest takes http request and applies functions to produce our custom
// request object decorated with values we need
func decorateRequest(r *http.Request) *request {
	decoratedReq := &request{
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

func (rd *request) expands(field string) bool {
	return rd.expandFields[field]
}

func (rd *request) selects() []string {
	return rd.selectFields
}

func (rd *request) getVar(name string) string {
	vars := mux.Vars(rd.Request)
	return vars[name] // todo(sideninja) consider returning err if non-existing
}

func (rd *request) getQueryParam(name string) string {
	return rd.Request.URL.Query().Get(name) // todo(sideninja) consider returning err if non-existing
}

func (rd *request) getQueryParams(name string) ([]string, error) {
	param := rd.Request.URL.Query().Get(name)

	return toStringArray(param)
}

func (rd *request) bodyAs(dst interface{}) error {
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

func (rd *request) ids() ([]flow.Identifier, error) {
	rawIDs := rd.getVar("id")
	ids, err := toStringArray(rawIDs)
	if err != nil {
		return nil, err
	}

	return toIDs(ids)
}

func (rd *request) id() (flow.Identifier, error) {
	return toID(rd.getVar("id"))
}
