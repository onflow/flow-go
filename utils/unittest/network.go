package unittest

import (
	"github.com/stretchr/testify/mock"

	module "github.com/dapperlabs/flow-go/module/mock"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
)

// RegisterNetwork returns a mocked network and conduit
func RegisterNetwork() (*module.Network, *network.Conduit) {
	con := &network.Conduit{}

	// set up network module mock
	net := &module.Network{}
	net.On("Register", mock.Anything, mock.Anything).Return(
		func(code uint8, engine netint.Engine) netint.Conduit {
			return con
		},
		nil,
	)

	return net, con
}
