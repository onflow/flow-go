package commands

import "github.com/onflow/flow-go/admin"

type AdminCommand struct {
	Handler   admin.CommandHandler
	Validator admin.CommandValidator
}
