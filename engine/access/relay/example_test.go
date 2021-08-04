package relay_test

import (
	"fmt"
	"math/rand"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/relay"
	splitterNetwork "github.com/onflow/flow-go/engine/common/splitter/network"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

func Example() {
	// create a mock network
	net := unittest.NewNetwork()

	// create splitter network
	logger := zerolog.Nop()
	splitterNet := splitterNetwork.NewNetwork(net, logger)

	// generate a random origin ID
	var id flow.Identifier
	rand.Seed(0)
	rand.Read(id[:])

	// create engines
	engineProcessFunc := func(engineName string) unittest.EngineProcessFunc {
		return func(channel network.Channel, originID flow.Identifier, event interface{}) error {
			fmt.Printf("Engine %v received message: channel=%v, originID=%v, event=%v\n", engineName, channel, originID, event)
			return nil
		}
	}
	fooEngine := unittest.NewEngine().OnProcess(engineProcessFunc("Foo"))
	barEngine := unittest.NewEngine().OnProcess(engineProcessFunc("Bar"))

	// register engines on the splitter network
	fooChannel := network.Channel("foo-channel")
	barChannel := network.Channel("bar-channel")
	splitterNet.Register(fooChannel, fooEngine)
	splitterNet.Register(barChannel, barEngine)

	// create another network that messages will be relayed to
	relayNet := unittest.NewNetwork().OnPublish(func(channel network.Channel, event interface{}, targetIDs ...flow.Identifier) error {
		fmt.Printf("Message published to relay network: channel=%v, event=%v, targetIDs=%v\n", channel, event, targetIDs)
		return nil
	})

	// create relay engine
	channels := network.ChannelList{fooChannel, barChannel}
	relay.New(logger, channels, splitterNet, relayNet)

	// send messages to network
	net.Send(fooChannel, id, "foo")
	net.Send(barChannel, id, "bar")

	// Unordered output:
	// Message published to relay network: channel=foo-channel, event=foo, targetIDs=[0000000000000000000000000000000000000000000000000000000000000000]
	// Engine Foo received message: channel=foo-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=foo
	// Message published to relay network: channel=bar-channel, event=bar, targetIDs=[0000000000000000000000000000000000000000000000000000000000000000]
	// Engine Bar received message: channel=bar-channel, originID=0194fdc2fa2ffcc041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75, event=bar
}
