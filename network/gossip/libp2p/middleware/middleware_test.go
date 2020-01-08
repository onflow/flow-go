package middleware

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/mock"
)

func TestSendAndReceive(t *testing.T) {
	count := 2
	ids, mws := createAndStartMiddleWares(t, count)
	require.Len(t, ids, count)
	require.Len(t, mws, count)
	msg := json.Envelope{Code: json.CodeRequest, Data: []byte(`{"key": "hello", "value": "world"}`)}
	//time.Sleep(10 * time.Minute)
	mws[0].Send(ids[count-1], msg)
}

func createAndStartMiddleWares(t *testing.T, count int) ([]flow.Identifier, []*Middleware) {
	var mws []*Middleware
	var ids []flow.Identifier
	for i := 0; i < count; i++ {

		var target [32]byte
		target[0] = byte(i + 1)
		targetID := flow.Identifier(target)
		ids = append(ids, targetID)

		logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
		codec := json.NewCodec()

		mw, err := New(logger, codec, uint(count-1), "0.0.0.0:0", targetID)
		require.NoError(t, err)

		mws = append(mws, mw)
	}

	var overlays []*mock.Overlay
	for i := 0; i < count; i++ {
		overlay := &mock.Overlay{}
		target := i + 1
		if i == count-1 {
			target = 0
		}
		ip, port := mws[target].libP2PNode.GetIPPort()
		flowID := flow.Identity{NodeID: ids[target], Address: fmt.Sprintf("%s:%s", ip, port)}
		overlay.On("Identity").Return(flowID, nil)
		overlay.On("Handshake", mockery.Anything).Return(flowID.NodeID, nil)
		overlay.On("Receive", mockery.Anything).Return(nil).Run(func(args mockery.Arguments) {
			fmt.Printf(" Recd: %s from %s", args[1], args[0])
		})
		overlays = append(overlays, overlay)
	}

	for i := 0; i < count; i++ {
		mws[i].Start(overlays[i])
	}

	return ids, mws

}
