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
	return vars[name] // todo(sideninja) check if exists
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
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return NewBadRequestError(msg, err)

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			return NewBadRequestError(msg, err)

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return NewBadRequestError(msg, err)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return NewBadRequestError(msg, err)

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return NewBadRequestError(msg, err)

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return NewRestError(http.StatusRequestEntityTooLarge, msg, err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return NewBadRequestError(msg, err)
	}

	return nil
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
