package commands

import (
	"context"

	"github.com/onflow/flow-go/admin"
)

type AdminCommand interface {
	Handler(ctx context.Context, request *admin.CommandRequest) (interface{}, error)
	Validator(request *admin.CommandRequest) error
}
