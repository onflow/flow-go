package rpc

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	protobuf "github.com/dapperlabs/flow-go/integration/ghost/protobuf"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
)

// Handler handles the GRPC calls from a client
// Handler handles the GRPC calls from a client
type Handler struct {
	log        zerolog.Logger
	conduitMap map[int]network.Conduit
	msgChan    chan []byte
	codec      network.Codec
}

var _ protobuf.GhostNodeAPIServer = Handler{}

func NewHandler(log zerolog.Logger, conduitMap map[int]network.Conduit, msgChan chan []byte) *Handler {
	return &Handler{
		log:        log,
		conduitMap: conduitMap,
		msgChan:    msgChan,
		codec:      jsoncodec.NewCodec(),
	}
}

func (h Handler) SendEvent(_ context.Context, req *protobuf.SendEventRequest) (*empty.Empty, error) {

	channelID := req.GetChannelId()

	// find the conduit for the channel ID
	conduit, found := h.conduitMap[int(channelID)]

	if !found {
		return nil, status.Error(codes.InvalidArgument, "conduit not found for given channel id")
	}

	// TODO: The message might have to be decoded to avoid double encoding by the n/w layer
	message := req.GetMessage()
	targetIDs := req.GetTargetID()

	// Convert target ID to flow IDS
	var flowIDs = make([]flow.Identifier, len(targetIDs))
	for i, id := range targetIDs {
		flowIDs[i] = flow.HashToID(id)
	}

	// Submit the message over libp2p
	err := conduit.Submit(message, flowIDs...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to submit message: %v", err)
	}

	return nil, nil
}

// Subscribe streams ALL the libp2p network messages over GRPC
func (h Handler) Subscribe(_ *protobuf.SubscribeRequest, stream protobuf.GhostNodeAPI_SubscribeServer) error {
	for {

		// read the network message from the channel
		msg, ok := <-h.msgChan
		if !ok {
			return nil
		}

		// json encode the message into bytes
		encodedMsg, err := h.codec.Encode(msg)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to encode message: %v", err)
		}

		flowMessage := &protobuf.FlowMessage{
			Message: encodedMsg,
		}

		// send it to the client
		err = stream.Send(flowMessage)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to stream message: %v", err)
		}

		// TODO: Add timeout (and reset it after every message processed)
	}
}
