package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AdminClient is a simple client for interacting with the Flow admin server
type AdminClient struct {
	client *http.Client
	url    string
}

// Request is the request to the admin server.
type Request struct {
	CommandName string `json:"commandName"`
	Data        any    `json:"data,omitempty"`
}

// Response is the response from the admin server.
type Response struct {
	Output any `json:"output"`
}

// AdminClientOption is a function that configures an admin client.
type AdminClientOption func(c *AdminClient)

// WithHTTPClient configures the admin client to use the provided HTTP client.
func WithHTTPClient(client *http.Client) AdminClientOption {
	return func(c *AdminClient) {
		c.client = client
	}
}

// WithTLS configures the admin client to use TLS when sending requests.
func WithTLS(enabled bool) AdminClientOption {
	return func(c *AdminClient) {
		c.url = strings.Replace(c.url, "http://", "https://", 1)
	}
}

// NewAdminClient creates a new admin client.
func NewAdminClient(serverAddr string, opts ...AdminClientOption) *AdminClient {
	c := &AdminClient{
		client: &http.Client{},
		url:    fmt.Sprintf("http://%s/admin/run_command", serverAddr),
	}

	for _, apply := range opts {
		apply(c)
	}

	return c
}

// Ping sends a ping command to the server and returns an error if the response is not "pong".
func (c *AdminClient) Ping(ctx context.Context) error {
	response, err := c.send(ctx, Request{
		CommandName: "ping",
	})
	if err != nil {
		return err
	}

	if response.Output != "pong" {
		return fmt.Errorf("unexpected response: %v", response.Output)
	}

	return nil
}

// RunCommand sends a command to the server and returns the response.
func (c *AdminClient) RunCommand(ctx context.Context, commandName string, data any) (*Response, error) {
	response, err := c.send(ctx, Request{
		CommandName: commandName,
		Data:        data,
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c *AdminClient) send(ctx context.Context, req Request) (*Response, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	resp, err := c.client.Post(c.url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result Response
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &result, nil
}
