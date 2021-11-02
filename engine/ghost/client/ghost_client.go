package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"

	ghost "github.com/onflow/flow-go/engine/ghost/protobuf"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
)

// GhostClient is a client for the ghost node.
//
// The ghost node is a special node type, used for testing purposes. It can
// "impersonate" any other node role, send messages to other nodes on the
// network, and listen to broadcast messages.
//
// NOTE: currently the ghost node is limited to 1-K messages (ie. messages sent
// to at least 2 other nodes). The ghost node WILL NOT receive a 1-1 message,
// unless the message is explicitly sent to it.
type GhostClient struct {
	rpcClient ghost.GhostNodeAPIClient
	close     func() error
	codec     network.Codec
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

func (c *GhostClient) Send(ctx context.Context, channel network.Channel, event interface{}, targetIDs ...flow.Identifier) error {

	message, err := c.codec.Encode(event)
	if err != nil {
		return fmt.Errorf("could not encode event: %w", err)
	}

	var targets [][]byte
	for _, t := range targetIDs {
		id := make([]byte, len(t))
		copy(id, t[:])
		targets = append(targets, id)
	}

	req := ghost.SendEventRequest{
		ChannelId: channel.String(),
		TargetID:  targets,
		Message:   message,
	}

	_, err = c.rpcClient.SendEvent(ctx, &req)
	if err != nil {
		return fmt.Errorf("failed to send event to the ghost node: %w", err)
	}
	return nil
}

func (c *GhostClient) Subscribe(ctx context.Context) (*FlowMessageStreamReader, error) {
	req := ghost.SubscribeRequest{}
	stream, err := c.rpcClient.Subscribe(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe for events: %w", err)
	}
	return &FlowMessageStreamReader{stream: stream, codec: c.codec}, nil
}

type FlowMessageStreamReader struct {
	stream ghost.GhostNodeAPI_SubscribeClient
	codec  network.Codec
}

func (fmsr *FlowMessageStreamReader) Next() (flow.Identifier, interface{}, error) {
	msg, err := fmsr.stream.Recv()
	if errors.Is(err, io.EOF) {
		// read done.
		return flow.ZeroID, nil, fmt.Errorf("end of stream reached: %w", err)
	}
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failed to read stream: %w", err)
	}

	event, err := fmsr.codec.Decode(msg.GetMessage())
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failed to decode event: %w", err)
	}

	originID := flow.HashToID(msg.GetSenderID())

	return originID, event, nil
}
