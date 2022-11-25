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
	//
	// Returns admin.InvalidAdminReqError if request validation fails.
	// Any error returned will abort the command execution.
	// Expected errors will be returned to the caller with InvalidArg error code.
	// Unexpected errors will be returned with Internal error code, but will not be otherwise propagated.
	Validator(request *admin.CommandRequest) error

	// Handler is responsible for handling the request. It applies any state
	// changes associated with the request and returns any values which should
	// be displayed to the initiator of the request.
	//
	// No errors are expected during normal operation.
	// If any error is returned, the command was aborted.
	// Unexpected errors will be returned with Internal error code, but will not be otherwise propagated.
	Handler(ctx context.Context, request *admin.CommandRequest) (any, error)
}
