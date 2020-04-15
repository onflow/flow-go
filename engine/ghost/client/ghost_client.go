package client

import (
	"context"
	"io"

	"google.golang.org/grpc"

	ghost "github.com/dapperlabs/flow-go/engine/ghost/protobuf"
	"github.com/dapperlabs/flow-go/model/flow"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
)

// GhostClient is a client for the Ghost Node
type GhostClient struct {
	rpcClient ghost.GhostNodeAPIClient
	close     func() error
	codec     *jsoncodec.Codec
}

func NewGhostClient(addr string) (*GhostClient, error) {

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := ghost.NewGhostNodeAPIClient(conn)

	return &GhostClient{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
		codec:     jsoncodec.NewCodec(),
	}, nil
}

// Close closes the client connection.
func (c *GhostClient) Close() error {
	return c.close()
}

func (c *GhostClient) Send(ctx context.Context, channelID uint8, targetIDs []flow.Identifier, event interface{}) error {

	message, err := c.codec.Encode(event)
	if err != nil {
		return err
	}

	targets := make([][]byte, len(targetIDs))
	for i, t := range targetIDs {
		targets[i] = t[:]
	}

	req := ghost.SendEventRequest{
		ChannelId: uint32(channelID),
		TargetID:  targets,
		Message:   message,
	}

	_, err = c.rpcClient.SendEvent(ctx, &req)
	return err
}

func (c *GhostClient) Subscribe(ctx context.Context) (*FlowMessageStreamReader, error) {
	req := ghost.SubscribeRequest{}
	stream, err := c.rpcClient.Subscribe(ctx, &req)
	if err != nil {
		return nil, err
	}
	return &FlowMessageStreamReader{stream: stream}, nil
}

type FlowMessageStreamReader struct {
	stream ghost.GhostNodeAPI_SubscribeClient
	codec  jsoncodec.Codec
}

func (fmsr *FlowMessageStreamReader) Next() (*flow.Identifier, interface{}, error) {
	msg, err := fmsr.stream.Recv()
	if err == io.EOF {
		// read done.
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	event, err := fmsr.codec.Decode(msg.GetMessage())
	if err != nil {
		return nil, nil, err
	}

	originID := flow.HashToID(msg.GetSenderID())

	return &originID, event, nil
}
