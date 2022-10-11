package commands

import (
	"context"

	"github.com/onflow/flow-go/admin"
)

// AdminCommand defines the interface expected for admin command handlers.
type AdminCommand interface {
	// Validator is responsible for validating that the input forms a valid request.
	// By convention, Validator may set the ValidatorData field on the request, and
	// this will persist when the request is passed to Handler.
	// All errors indicate an invalid request.
	// TODO define sentinel error type for expected errors
	Validator(request *admin.CommandRequest) error
	// Handler is responsible for handling the request. It applies any state
	// changes associated with the request and returns any values which should
	// be displayed to the initiator of the request.
	// All errors indicate an invalid request, or benign failure to satisfy the request.
	// TODO define sentinel error type for expected errors
	Handler(ctx context.Context, request *admin.CommandRequest) (interface{}, error)
}
