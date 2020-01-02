package enginetest

import (
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/peerstable"
	protocols "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"
	"github.com/dapperlabs/flow-go/network/gossip/testnet"
	"github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// A mock engine to demonstrate the correctness of adding engine-conduit notion to the gossip network
type helloEngine struct {
	wg     *sync.WaitGroup
	event  string
	sender flow.Identifier
}

func (he *helloEngine) SubmitLocal(event interface{}) {
	panic("not implemented")
}

func (he *helloEngine) Submit(originID flow.Identifier, event interface{}) {
	panic("not implemented")
}

func (he *helloEngine) ProcessLocal(event interface{}) error {
	panic("not implemented")
}

func (he *helloEngine) Process(originID flow.Identifier, event interface{}) error {

	str, ok := event.(string)
	if !ok {
		return errors.New("could not cast event to string")
	}

	if event == nil {
		return errors.New("event is empty")
	}

	he.event = str
	he.sender = originID

	he.wg.Done()
	return nil
}

// TestConduit makes two nodes, each having an instance of the helloEngine
// Each engine on a node sends a message to the other node
// The test verifies the correctness of the message delivery
func TestConduit(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	pt, err := peerstable.NewPeersTable()
	require.Nil(t, err, "could not create peers table")

	// picking two random available ports
	ln, network := testnet.FindPorts(2)

	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()

	// mapping the first port to the first id and the second port to the second id
	pt.Add(id1, network[0])
	pt.Add(id2, network[1])

	mockCodec := &mock.Codec{}

	mockCodec.On("Encode", "Hello!").Return([]byte("Hello!"), nil)
	mockCodec.On("Encode", "Hey").Return([]byte("Hey"), nil)

	mockCodec.On("Decode", []byte("Hello!")).Return("Hello!", nil)
	mockCodec.On("Decode", []byte("Hey")).Return("Hey", nil)

	opts := []gossip.Option{
		gossip.WithLogger(zerolog.New(ioutil.Discard)),
		gossip.WithPeerTable(pt),
		gossip.WithCodec(mockCodec),
		gossip.WithStaticFanoutSize(2),
	}

	// configuring and running the first node
	opts1 := append(opts,
		gossip.WithNodeID(id1),
		gossip.WithAddress(network[0]),
	)
	n1 := gossip.NewNode(opts1...)
	sp1, err := protocols.NewGServer(n1)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}
	n1.SetProtocol(sp1)
	require.Nil(t, err, "could not get listener")
	go func() { _ = n1.Serve(ln[0]) }()

	// configuring and running the second node
	opts2 := append(opts,
		gossip.WithNodeID(id2),
		gossip.WithAddress(network[1]),
	)
	n2 := gossip.NewNode(opts2...)
	sp2, err := protocols.NewGServer(n2)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}
	n2.SetProtocol(sp2)

	require.Nil(t, err, "could not get listener")
	go func() { _ = n2.Serve(ln[1]) }()

	// registering the hello engine at both nodes
	he1 := &helloEngine{wg: &wg}
	conduit1, err := n1.Register(1, he1)
	require.Nil(t, err, "could not registry engine")

	he2 := &helloEngine{wg: &wg}
	conduit2, err := n2.Register(1, he2)
	require.Nil(t, err, "could not registry engine")

	// sending a message from each engine to the other engine
	err = conduit1.Submit("Hello!", id2)
	require.Nil(t, err, "conduit could not submit event")

	err = conduit2.Submit("Hey", id1)
	require.Nil(t, err, "conduit could not submit event")

	// Wait or timeout
	c := make(chan struct{})

	go func() {
		defer close(c)
		wg.Wait()
	}()

	// blocking test till either a timeout or all the engines
	select {
	case <-c:
	case <-time.After(1 * time.Second):
		t.Error("messages timed out")
		t.FailNow()
	}

	// Make sure that each engine receives their respective events
	assert.Equal(t, "Hello!", he2.event, "received wrong event")
	assert.Equal(t, id1, he2.sender, "received from wrong sender")

	assert.Equal(t, "Hey", he1.event, "received wrong event")
	assert.Equal(t, id2, he1.sender, "received from wrong sender")
}
