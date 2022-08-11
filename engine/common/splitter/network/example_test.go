package network_test

import (
	"fmt"
	"math/rand"

	"github.com/rs/zerolog"

	splitterNetwork "github.com/onflow/flow-go/engine/common/splitter/network"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	testnet "github.com/onflow/flow-go/utils/unittest/network"
)

func Example() {
	// create a mock network
	net := testnet.NewNetwork()

	// create a splitter network
	logger := zerolog.Nop()
	splitterNet := splitterNetwork.NewNetwork(net, logger)

	// generate a random origin ID
	var id flow.Identifier
	rand.Seed(0)
	rand.Read(id[:])

	// create engines
	engineProcessFunc := func(engineID int) testnet.EngineProcessFunc {
		return func(channel network.Channel, originID flow.Identifier, event interface{}) error {
			fmt.Printf("Engine %d received message: channel=%v, originID=%v, event=%v\n", engineID, channel, originID, event)
			return nil
		}
	}
	engine1 := testnet.NewEngine().OnProcess(engineProcessFunc(1))
	engine2 := testnet.NewEngine().OnProcess(engineProcessFunc(2))
	engine3 := testnet.NewEngine().OnProcess(engineProcessFunc(3))

	// register engines with splitter network
	channel := network.Channel("foo-channel")
	_, err := splitterNet.Register(channel, engine1)
	if err != nil {
		fmt.Println(err)
	}
	_, err = splitterNet.Register(channel, engine2)
	if err != nil {
		fmt.Println(err)
	}
	_, err = splitterNet.Register(channel, engine3)
	if err != nil {
		fmt.Println(err)
	}

	// send message to network
	err = net.Send(channel, id, "foo")
	if err != nil {
		fmt.Println(err)
	}

	// Unordered output:
	// Engine 1 received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
	// Engine 2 received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
	// Engine 3 received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
}
