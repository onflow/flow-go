package enginetest

import (
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/peerstable"
	"github.com/dapperlabs/flow-go/network/gossip/testnet"

	"github.com/dapperlabs/flow-go/network/mock"
	protocols "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A mock engine to demonstrate the correctness of adding engine-conduit notion to the gossip network
type helloEngine struct {
	wg     *sync.WaitGroup
	event  string
	sender string
}

func (he *helloEngine) Identify(event interface{}) ([]byte, error) {
	return nil, nil
}
func (he *helloEngine) Retrieve(eventID []byte) (interface{}, error) {
	return nil, nil
}

func (he *helloEngine) Process(originID string, event interface{}) error {

	str, ok := event.(string)
	if !ok {
		return errors.New("could not cast event to string")
	}

	if originID == "" {
		return errors.New("origin Id is empty")
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

	// mapping the first port to the id of 10 and the second port to the id of 20
	pt.Add("10", network[0])
	pt.Add("20", network[1])

	mockCodec := &mock.Codec{}

	mockCodec.On("Encode","Hello!").Return([]byte("Hello!"),nil)
	mockCodec.On("Encode","Hey").Return([]byte("Hey"),nil)

	mockCodec.On("Decode",[]byte("Hello!")).Return("Hello!",nil)
	mockCodec.On("Decode",[]byte("Hey")).Return("Hey",nil)

	opts := []gossip.Option{
		gossip.WithLogger(zerolog.New(ioutil.Discard)),
		gossip.WithPeerTable(pt),
		gossip.WithCodec(mockCodec),
		gossip.WithAddress(network[0]),
		gossip.WithStaticFanoutSize(2),
	}

	// configuring and running the first node
	n1 := gossip.NewNode(opts...)
	sp1, err := protocols.NewGServer(n1)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}
	n1.SetProtocol(sp1)
	require.Nil(t, err, "could not get listener")
	go func() { _ = n1.Serve(ln[0]) }()

	opts[3] = gossip.WithAddress(network[1])

	// configuring and running the second node
	n2 := gossip.NewNode(opts...)
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
	err = conduit1.Submit("Hello!", "20")
	require.Nil(t, err, "conduit could not submit event")

	err = conduit2.Submit("Hey", "10")
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
	}

	// Make sure that each engine receives their respective events
	assert.Equal(t, "Hello!", he2.event, "received wrong event")
	assert.Equal(t, "10", he2.sender, "received from wrong sender")

	assert.Equal(t, "Hey", he1.event, "received wrong event")
	assert.Equal(t, "20", he1.sender, "received from wrong sender")
}
