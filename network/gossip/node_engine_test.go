package gossip

// This file contains a set of tests against the adaptability of the Engine-Conduits to the gossip node.
import (
	"context"
	"errors"
	"github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

// A mock engine that implements the engine interface (see engine.go for more details)
type helloEngine struct {
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

	he.event = str
	he.sender = originID
	return nil
}

// TestSendEngine tests the sendEngine function
func TestSendEngine(t *testing.T) {

	mockCodec := &mock.Codec{}
	event := "hello"
	mockCodec.On("Encode", event).Return([]byte(event), nil)
	mockCodec.On("Decode", []byte(event)).Return(event, nil)

	n := NewNode(WithLogger(zerolog.New(ioutil.Discard)), WithCodec(mockCodec), WithAddress(defaultAddress))

	he := &helloEngine{}
	_, err := n.Register(1, he)
	require.Nil(t, err, "could not registry engine to node")

	eventBytes, err := n.codec.Encode(event)
	require.Nil(t, err, "could not encode event")

	eprBytes, err := proto.Marshal(&messages.EventProcessRequest{EngineID: 1, Event: eventBytes, SenderID: ""})
	require.Nil(t, err, "could not marshal event request")

	_, err = n.sendEngine(context.Background(), eprBytes)
	require.Nil(t, err, "could not send event process request to engine")
}
