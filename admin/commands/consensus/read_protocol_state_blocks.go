package consensus

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

var ReadProtocolStateCommand commands.AdminCommand = commands.AdminCommand{
	Handler: func(ctx context.Context, req *admin.CommandRequest) error {

		return nil
	},
	Validator: func(req *admin.CommandRequest) error {

		return nil
	},
}
