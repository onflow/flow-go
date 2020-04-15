package client

import (
	"context"
	"io"

	"google.golang.org/grpc"

	ghost "github.com/dapperlabs/flow-go/engine/ghost/protobuf"
)

// GhostClient is a client for the Ghost Node
type GhostClient struct {
	rpcClient ghost.GhostNodeAPIClient
	close     func() error
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
	}, nil
}

// Close closes the client connection.
func (c *GhostClient) Close() error {
	return c.close()
}

func (c *GhostClient) Send(ctx context.Context, channelID uint8, targetIDs [][]byte, message []byte) error {

	req := ghost.SendEventRequest{
		ChannelId: uint32(channelID),
		TargetID:  targetIDs,
		Message:   message,
	}

	_, err := c.rpcClient.SendEvent(ctx, &req)
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
}

func (fmsr *FlowMessageStreamReader) Next() (*ghost.FlowMessage, error) {
	msg, err := fmsr.stream.Recv()
	if err == io.EOF {
		// read done.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return msg, err
}
