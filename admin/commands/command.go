package commands

import "github.com/onflow/flow-go/admin"

type AdminCommand interface {
	Handler() admin.CommandHandler
	Validator() admin.CommandValidator
}
