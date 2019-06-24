package client

import (
	"fmt"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-emulator/gen/grpc/services/accessv1"
)

type Client struct {
	conn       *grpc.ClientConn
	grpcClient accessv1.BambooAccessAPIClient
}

func New(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(addr)
	if err != nil {
		return nil, err
	}

	grpcClient := accessv1.NewBambooAccessAPIClient(conn)

	return &Client{
		conn:       conn,
		grpcClient: grpcClient,
	}, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

// LogCommands displays all the usable commands to a client.
func LogCommands() {
	// TODO: log all help commands available to Client
	fmt.Println("here are all the commands you can use!")
}

// CreateAccount creates an account for the client.
func CreateAccount() {
	// TODO: create a new account in the client's wallet
}
