package gossip

// This file contains a set of tests against the adaptability of the Engine-Conduits to the gossip node.
import (
	"context"
	"errors"
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/protobuf/gossip/messages"
)

// A mock engine that implements the engine interface (see engine.go for more details)
type helloEngine struct {
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

	eprBytes, err := proto.Marshal(&messages.EventProcessRequest{ChannelID: 1, Event: eventBytes, SenderID: ""})
	require.Nil(t, err, "could not marshal event request")

	_, err = n.sendEngine(context.Background(), eprBytes)
	require.Nil(t, err, "could not send event process request to engine")

}
