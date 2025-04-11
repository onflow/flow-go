package common

import (
	"context"
	"strings"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

// GetAccessAPIClient is a helper function that creates client API for AccessAPI service
func GetAccessAPIClient(address string) (accessproto.AccessAPIClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return accessproto.NewAccessAPIClient(conn), nil
}

// GetWSClient is a helper function that creates a websocket client
func GetWSClient(ctx context.Context, address string) (*websocket.Conn, error) {
	// helper func to create WebSocket client
	client, _, err := websocket.DefaultDialer.DialContext(ctx, strings.Replace(address, "http", "ws", 1), nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}
