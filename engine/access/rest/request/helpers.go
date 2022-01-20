package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

func parseBody(raw io.Reader, dst interface{}) error {
	dec := json.NewDecoder(raw)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return fmt.Errorf("request body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			return fmt.Errorf("request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("request body contains unknown field %s", fieldName)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("request body must not be empty")
		default:
			return err
		}
	}

	if dst == nil {
		return fmt.Errorf("request body must not be empty")
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return fmt.Errorf("request body must only contain a single JSON object")
	}

	return nil
}

type GetByIDRequest struct {
	ID flow.Identifier
}

func (g *GetByIDRequest) Build(r *Request) error {
	return g.Parse(
		r.GetVar(idQuery),
	)
}

func (g *GetByIDRequest) Parse(rawID string) error {
	var id ID
	err := id.Parse(rawID)
	if err != nil {
		return err
	}
	g.ID = id.Flow()

	return nil
}
