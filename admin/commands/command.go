package commands

import (
	"context"

	"github.com/onflow/flow-go/admin"
)

// AdminCommand defines the interface expected for admin command handlers.
type AdminCommand interface {
	// Validator is responsible for validating the input of a command, available in
	// the Data field of the request argument. By convention, Validator may set the
	// ValidatorData field on the request, and this will persist when the request
	// is passed to Handler.
	// Returns Invalid if request validation fails
	// Any error returned will abort the command execution and will be returned to the caller with InvalidArg error code.
	// TODO define sentinel error type for expected errors
	Validator(request *admin.CommandRequest) error

	// Handler is responsible for handling the request. It applies any state
	// changes associated with the request and returns any values which should
	// be displayed to the initiator of the request.
	// All errors indicate an invalid request, or benign failure to satisfy the request.
	// Any returned error will be returned to the caller with an error status code.
	// If no error is returned, the first return value is serialized and returned to a caller in an `Output` field.
	// TODO define sentinel error type for expected errors
	Handler(ctx context.Context, request *admin.CommandRequest) (interface{}, error)
}
