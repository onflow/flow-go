package testnet

import (
	"context"
	"fmt"

	"github.com/antihax/optional"

	swagger "github.com/onflow/flow/openapi/experimental/go-client-generated"
)

// ExperimentalAPIClient wraps the generated OpenAPI client for the experimental REST API,
// providing higher-level methods for common operations.
type ExperimentalAPIClient struct {
	client *swagger.APIClient
}

// NewExperimentalAPIClient creates a new [ExperimentalAPIClient] targeting the given base URL.
//
// No error returns are expected during normal operation.
func NewExperimentalAPIClient(baseURL string) (*ExperimentalAPIClient, error) {
	cfg := swagger.NewConfiguration()
	cfg.BasePath = baseURL
	client := swagger.NewAPIClient(cfg)
	return &ExperimentalAPIClient{client: client}, nil
}

// GetAccountTransactions fetches a single page of account transactions for the given address.
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetAccountTransactions(
	ctx context.Context,
	address string,
	opts *swagger.AccountsApiGetAccountTransactionsOpts,
) (*swagger.AccountTransactionsResponse, error) {
	resp, _, err := c.client.AccountsApi.GetAccountTransactions(ctx, address, opts)
	if err != nil {
		return nil, fmt.Errorf("API request failed for account %s: %w", address, err)
	}
	return &resp, nil
}

// GetAllAccountTransactions paginates through all account transactions for the given address
// and returns the accumulated results. The `pageSize` controls how many transactions are
// fetched per request. An optional `roles` filter restricts results to transactions where
// the account had the specified role. An optional `expand` parameter controls which nested
// fields (e.g. "transaction", "result") are included in each response entry.
//
// No error returns are expected during normal operation.
func (c *ExperimentalAPIClient) GetAllAccountTransactions(
	ctx context.Context,
	address string,
	pageSize int,
	roles *swagger.Role,
	expand *[]string,
) ([]swagger.AccountTransaction, error) {
	var all []swagger.AccountTransaction

	opts := buildOpts(int32(pageSize), nil, roles, expand)

	for {
		resp, err := c.GetAccountTransactions(ctx, address, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get account transactions page: %w", err)
		}

		all = append(all, resp.Transactions...)
		if resp.NextCursor == "" {
			break
		}
		cursor := resp.NextCursor
		opts = buildOpts(int32(pageSize), &cursor, roles, expand)
	}
	return all, nil
}

// buildOpts constructs [swagger.AccountsApiGetAccountTransactionsOpts] from the given parameters.
func buildOpts(
	limit int32,
	cursor *string,
	roles *swagger.Role,
	expand *[]string,
) *swagger.AccountsApiGetAccountTransactionsOpts {
	opts := &swagger.AccountsApiGetAccountTransactionsOpts{
		Limit: optional.NewInt32(limit),
	}
	if cursor != nil {
		opts.Cursor = optional.NewInterface(*cursor)
	}
	if roles != nil {
		opts.Roles = optional.NewInterface(*roles)
	}
	if expand != nil {
		opts.Expand = optional.NewInterface(*expand)
	}
	return opts
}
