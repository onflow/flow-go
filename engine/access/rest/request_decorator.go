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
	expandFields map[string]bool // todo(sideninja) discuss removing bool and replacing with string array
	selectFields map[string]bool // todo(sideninja) discuss removing bool and replacing with string array
}

func newRequestDecorator(r *http.Request) *requestDecorator {
	decoratedReq := &requestDecorator{
		Request: r,
	}
	decoratedReq.expandFields, _ = middleware.GetFieldsToExpand(r) // todo(sideninja) discuss moving to here or in general out of middlewares since it's not a middleware anymore
	decoratedReq.selectFields, _ = middleware.GetFieldsToSelect(r) // todo(sideninja) discuss moving to here or in general out of middlewares since it's not a middleware anymore
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
	return vars[name] // todo(sideninja) consider returning err if non-existing
}

func (rd *requestDecorator) getQuery(name string) string {
	return rd.Request.URL.Query().Get(name) // todo(sideninja) consider returning err if non-existing
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
	return toIDs(rd.getParam("id"))
}

func (rd *requestDecorator) id() (flow.Identifier, error) {
	return toID(rd.getParam("id"))
}
