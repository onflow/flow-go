package unittest

import (
	"github.com/stretchr/testify/mock"

	module "github.com/onflow/flow-go/module/mock"
	netint "github.com/onflow/flow-go/network"
	network "github.com/onflow/flow-go/network/mock"
)

// RegisterNetwork returns a mocked network and conduit
func RegisterNetwork() (*module.Network, *network.Conduit) {
	con := &network.Conduit{}

	// set up network module mock
	net := &module.Network{}
	net.On("Register", mock.Anything, mock.Anything).Return(
		func(code string, engine netint.Engine) netint.Conduit {
			return con
		},
		nil,
	)

	return net, con
}
