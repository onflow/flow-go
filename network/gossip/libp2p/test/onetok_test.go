package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
)

type OneToKTestSuite struct {
	suite.Suite
	ee  map[int][]*EchoEngine
	ids map[int][]flow.Identifier
}

func TestOneToKTestSuite(t *testing.T) {
	suite.Run(t, new(OneToKTestSuite))
}

func (o *OneToKTestSuite) SetupTest() {
	const nodes = 4
	const groups = 2
	//golog.SetAllLoggers(gologging.INFO)
	o.ee = make(map[int][]*EchoEngine)

	subnets, ids, err := CreateSubnets(nodes, groups)
	require.NoError(o.Suite.T(), err)

	o.ids = ids

	// iterates over subnets
	for i := range subnets {
		// iterates over nets in each subnet
		for _, net := range subnets[i] {
			if o.ee[i] == nil {
				o.ee[i] = make([]*EchoEngine, 0)
			}
			e := NewEchoEngine(o.Suite.T(), net, 100, 1)
			o.ee[i] = append(o.ee[i], e)
		}
	}
}

func (o *OneToKTestSuite) TestIntraSubNet() {
	// iterates over subnets
	for i, ee := range o.ee {
		// iterates over each engine of a subnet

		// Sends a message from sender to receiver
		event := &message.Echo{
			Text: fmt.Sprintf("hello subnet %d", i),
		}
		require.NoError(o.Suite.T(), ee[0].con.Submit(event, o.ids[i]...))

	}

	//// evaluates reception of echo request
	//select {
	//case <-receiver.received:
	//	// evaluates reception of message at the other side
	//	// does not evaluate the content
	//	require.NotNil(s.Suite.T(), receiver.originID)
	//	require.NotNil(s.Suite.T(), receiver.event)
	//	assert.Equal(s.Suite.T(), s.ids[sndID].NodeID, receiver.originID)
	//
	//	// evaluates proper reception of event
	//	// casts the received event at the receiver side
	//	rcvEvent, ok := (<-receiver.event).(*message.Echo)
	//	// evaluates correctness of casting
	//	require.True(s.Suite.T(), ok)
	//	// evaluates content of received message
	//	assert.Equal(s.Suite.T(), event, rcvEvent)
	//
	//case <-time.After(10 * time.Second):
	//	assert.Fail(s.Suite.T(), "sender failed to send a message to receiver")
	//}
}
