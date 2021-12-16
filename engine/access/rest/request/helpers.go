package request

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
	"io"
	"strconv"
	"strings"
)

func parseBody(raw io.Reader, dst interface{}) error {
	//todo(sideninja) validate size

	dec := json.NewDecoder(raw)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			err := fmt.Errorf("request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return rest.NewBadRequestError(err)

		case errors.Is(err, io.ErrUnexpectedEOF):
			err := fmt.Errorf("request body contains badly-formed JSON")
			return rest.NewBadRequestError(err)

		case errors.As(err, &unmarshalTypeError):
			err := fmt.Errorf("request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return rest.NewBadRequestError(err)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			err := fmt.Errorf("Request body contains unknown field %s", fieldName)
			return rest.NewBadRequestError(err)

		case errors.Is(err, io.EOF):
			err := fmt.Errorf("request body must not be empty")
			return rest.NewBadRequestError(err)

		default:
			return err
		}
	}

	if dst == nil {
		return rest.NewBadRequestError(fmt.Errorf("request body must not be empty"))
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		err := fmt.Errorf("request body must only contain a single JSON object")
		return rest.NewBadRequestError(err)
	}

	return nil
}

type GetByIDRequest struct {
	ID flow.Identifier
}

func (g *GetByIDRequest) Build(r *rest.Request) error {
	return g.Parse(
		r.GetQueryParam(idQuery),
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

func fromBase64(bytesStr string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(bytesStr)
}

func fromUint64(number uint64) string {
	return fmt.Sprintf("%d", number)
}

func toUint64(uint64Str string) (uint64, error) {
	return strconv.ParseUint(uint64Str, 10, 64)
}
