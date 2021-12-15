package models

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
	"io"
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

func fromUint64(number uint64) string {
	return fmt.Sprintf("%d", number)
}

func toBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}
