package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
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
	// number of total engines
	const engines = 10

	// number of total subnets
	const groups = 5

	o.ee = make(map[int][]*EchoEngine)

	subnets, ids, err := CreateSubnets(engines, 2, 1)
	require.NoError(o.Suite.T(), err)

	o.ids = ids

	// iterates over subnets
	// registered makes sure that each engine is registered only once
	// an engine may be shared between several subnets
	registered := make(map[*libp2p.Network]struct{})
	for i := range subnets {
		// iterates over nets in each subnet
		for _, net := range subnets[i] {
			if o.ee[i] == nil {
				o.ee[i] = make([]*EchoEngine, 0)
			}

			// register the engine if it has not yet been registered
			if _, ok := registered[net]; !ok {
				e := NewEchoEngine(o.Suite.T(), net, 100, 1, false)
				o.ee[i] = append(o.ee[i], e)
				registered[net] = struct{}{}
			}
		}
	}
}

func (o *OneToKTestSuite) TestIntraSubNet() {
	timeout := 2 * time.Second

	// iterates over subnets
	for i, ee := range o.ee {
		// Sends a message from the first engine of each subnet to
		// the entire engines in the SAME subnet
		sender := ee[0]
		event := &message.Echo{
			Text: fmt.Sprintf("hello subnet %d", i),
		}
		require.NoError(o.Suite.T(), sender.con.Submit(event, o.ids[i]...))

	}

	// wg locks the main thread temporarily for go routines at each
	// receiver engine
	wg := &sync.WaitGroup{}

	// evaluates correct reception of event at the cluster
	for i, se := range o.ee {
		// event keeps copy of event disseminated in this cluster
		event := fmt.Sprintf("hello subnet %d", i)
		// sender keeps id of the node that disseminated event in this cluster
		sender := o.ids[i][0]

		for index, e := range se {
			if index == 0 {
				continue
			}
			wg.Add(1)
			go func(ec *EchoEngine) {
				defer wg.Done()

				select {
				case <-ec.received:
					// echo engine receives an event
					// evaluates event has an origin id
					require.NotNil(o.Suite.T(), ec.originID)
					// evaluates event against nil value
					require.NotNil(o.Suite.T(), ec.event)
					// evaluates origin id of message against sender id
					assert.Equal(o.Suite.T(), sender, ec.originID)
					// evaluates number of events node received, should be 1
					assert.Equal(o.Suite.T(), 1, len(ec.seen))
					// evaluates content of event against correct content
					// engine should seen the event of its subnet exactly once
					assert.Equal(o.Suite.T(), 1, ec.seen[event])

				case <-time.After(timeout):
					// timeout happened with no reception of event at this receiver
					assert.Fail(o.Suite.T(), fmt.Sprintf("timout exceeded on waiting for message"))
				}
			}(e)
		}
	}

	// locks main thread temporarily upon a timeout
	wg.Wait()
}

func (o *OneToKTestSuite) TestInterSubNet() {
	timeout := 10 * time.Second

	// making list of all nodes without duplication of those shared
	// between subsets
	allset := make(map[flow.Identifier]struct{})
	for subnet := range o.ids {
		for _, id := range o.ids[subnet] {
			allset[id] = struct{}{}
		}
	}

	sender := o.ee[0][0]
	senderID := o.ids[0][0]
	delete(allset, senderID)

	all := make([]flow.Identifier, 0)
	for id := range allset {
		all = append(all, id)
	}

	// iterates over subnets
	event := &message.Echo{
		Text: fmt.Sprintf("hello all"),
	}

	require.NoError(o.Suite.T(), sender.con.Submit(event, all...))

	// wg locks the main thread temporarily for go routines at each
	// receiver engine
	wg := &sync.WaitGroup{}

	// evaluates correct reception of event at the cluster
	for subIndex, se := range o.ee {
		for eIndex, e := range se {
			if e == sender {
				// skips sender
				continue
			}

			if _, ok := allset[o.ids[subIndex][eIndex]]; !ok {
				// skips already processed nodes
				continue
			}
			delete(allset, o.ids[subIndex][eIndex])

			wg.Add(1)
			go func(ec *EchoEngine, sub, eindex int) {
				defer wg.Done()

				select {
				case <-ec.received:
					// echo engine receives an event
					// evaluates event has an origin id
					require.NotNil(o.Suite.T(), ec.originID)
					// evaluates event against nil value
					require.NotNil(o.Suite.T(), ec.event)
					// evaluates origin id of message against sender id
					assert.Equal(o.Suite.T(), senderID, ec.originID)
					// evaluates number of events node received, should be 1
					assert.Equal(o.Suite.T(), len(ec.seen), 1)
					// evaluates content of event against correct content
					// engine should seen the event of its subnet exactly once
					assert.Equal(o.Suite.T(), ec.seen[event.Text], 1)

				case <-time.After(timeout):
					// timeout happened with no reception of event at this receiver
					assert.Fail(o.Suite.T(), fmt.Sprintf("timout exceeded on waiting for message %d %s", sub, o.ids[sub][eindex]))
				}
			}(e, subIndex, eIndex)
		}
	}

	// locks main thread temporarily upon a timeout
	wg.Wait()
}
