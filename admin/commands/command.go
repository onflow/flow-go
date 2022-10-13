package commands

import (
	"context"

	"github.com/onflow/flow-go/admin"
)

type AdminCommand interface {

	// Handler method is the main method of an admin command. It is called only after a successful validation
	// by Validator method.
	// request parameter is the same object as passed to Validator method, so field ValidatorData can be used
	// to pass the validated parameters.
	// Any returned error will be returned to the caller with an error status code.
	// If no error is returned, the first return value is serialized and returned to a caller in an `Output` field.
	Handler(ctx context.Context, request *admin.CommandRequest) (interface{}, error)

	// Validator method is responsible for validating the input of a command, available in Data field of the request
	// argument.
	// Any error returned will abort the command execution and will be returned to the caller with InvalidArg error code
	// The same request object will be later passed to Handler method, so this method can set ValidatorData field with the
	// validated data needed to run the command
	Validator(request *admin.CommandRequest) error
}
