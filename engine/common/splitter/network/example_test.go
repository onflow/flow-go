package network_test

import (
	"fmt"
	"math/rand"

	"github.com/rs/zerolog"

	splitterNetwork "github.com/onflow/flow-go/engine/common/splitter/network"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

func Example() {
	// create a mock network
	net := unittest.NewNetwork()

	// create a splitter network
	logger := zerolog.Nop()
	splitterNet := splitterNetwork.NewNetwork(net, logger)

	// generate a random origin ID
	var id flow.Identifier
	rand.Seed(0)
	rand.Read(id[:])

	// create engines
	engineProcessFunc := func(engineID int) unittest.EngineProcessFunc {
		return func(channel network.Channel, originID flow.Identifier, event interface{}) error {
			fmt.Printf("Engine %d received message: channel=%v, originID=%v, event=%v\n", engineID, channel, originID, event)
			return nil
		}
	}
	engine1 := unittest.NewEngine().OnProcess(engineProcessFunc(1))
	engine2 := unittest.NewEngine().OnProcess(engineProcessFunc(2))
	engine3 := unittest.NewEngine().OnProcess(engineProcessFunc(3))

	// register engines with splitter network
	channel := network.Channel("foo-channel")
	splitterNet.Register(channel, engine1)
	splitterNet.Register(channel, engine2)
	splitterNet.Register(channel, engine3)

	// send message to network
	net.Send(channel, id, "foo")

	// Unordered output:
	// Engine 1 received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
	// Engine 2 received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
	// Engine 3 received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
}
