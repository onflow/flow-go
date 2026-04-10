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

// GetAccountFungibleTransfers fetches a single page of fungible token transfers for the given address.
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetAccountFungibleTransfers(
	ctx context.Context,
	address string,
	opts *swagger.AccountsApiGetAccountFungibleTransfersOpts,
) (*swagger.AccountFungibleTransfersResponse, error) {
	resp, _, err := c.client.AccountsApi.GetAccountFungibleTransfers(ctx, address, opts)
	if err != nil {
		return nil, fmt.Errorf("FT transfers API request failed for account %s: %w", address, err)
	}
	return &resp, nil
}

// GetAllAccountFungibleTransfers paginates through all fungible token transfers for the given address.
//
// No error returns are expected during normal operation.
func (c *ExperimentalAPIClient) GetAllAccountFungibleTransfers(
	ctx context.Context,
	address string,
	pageSize int,
	opts *swagger.AccountsApiGetAccountFungibleTransfersOpts,
) ([]swagger.FungibleTokenTransfer, error) {
	var all []swagger.FungibleTokenTransfer

	if opts == nil {
		opts = &swagger.AccountsApiGetAccountFungibleTransfersOpts{}
	}
	opts.Limit = optional.NewInt32(int32(pageSize))

	for {
		resp, err := c.GetAccountFungibleTransfers(ctx, address, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get FT transfers page: %w", err)
		}

		all = append(all, resp.Transfers...)
		if resp.NextCursor == "" {
			break
		}
		opts.Cursor = optional.NewInterface(resp.NextCursor)
	}
	return all, nil
}

// GetAccountNonFungibleTransfers fetches a single page of non-fungible token transfers for the given address.
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetAccountNonFungibleTransfers(
	ctx context.Context,
	address string,
	opts *swagger.AccountsApiGetAccountNonFungibleTransfersOpts,
) (*swagger.AccountNonFungibleTransfersResponse, error) {
	resp, _, err := c.client.AccountsApi.GetAccountNonFungibleTransfers(ctx, address, opts)
	if err != nil {
		return nil, fmt.Errorf("NFT transfers API request failed for account %s: %w", address, err)
	}
	return &resp, nil
}

// GetAllAccountNonFungibleTransfers paginates through all non-fungible token transfers for the given address.
//
// No error returns are expected during normal operation.
func (c *ExperimentalAPIClient) GetAllAccountNonFungibleTransfers(
	ctx context.Context,
	address string,
	pageSize int,
	opts *swagger.AccountsApiGetAccountNonFungibleTransfersOpts,
) ([]swagger.NonFungibleTokenTransfer, error) {
	var all []swagger.NonFungibleTokenTransfer

	if opts == nil {
		opts = &swagger.AccountsApiGetAccountNonFungibleTransfersOpts{}
	}
	opts.Limit = optional.NewInt32(int32(pageSize))

	for {
		resp, err := c.GetAccountNonFungibleTransfers(ctx, address, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get NFT transfers page: %w", err)
		}

		all = append(all, resp.Transfers...)
		if resp.NextCursor == "" {
			break
		}
		opts.Cursor = optional.NewInterface(resp.NextCursor)
	}
	return all, nil
}

// GetContractByIdentifier fetches the latest deployment of the contract with the given identifier.
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetContractByIdentifier(
	ctx context.Context,
	identifier string,
	opts *swagger.ContractsApiGetContractByIdentifierOpts,
) (*swagger.ContractDeployment, error) {
	resp, _, err := c.client.ContractsApi.GetContractByIdentifier(ctx, identifier, opts)
	if err != nil {
		return nil, fmt.Errorf("contracts API request failed for %s: %w", identifier, err)
	}
	return &resp, nil
}

// GetContractDeployments fetches a single page of deployment history for the given contract.
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetContractDeployments(
	ctx context.Context,
	identifier string,
	opts *swagger.ContractsApiGetContractDeploymentsOpts,
) (*swagger.ContractDeploymentsResponse, error) {
	resp, _, err := c.client.ContractsApi.GetContractDeployments(ctx, identifier, opts)
	if err != nil {
		return nil, fmt.Errorf("contract deployments API request failed for %s: %w", identifier, err)
	}
	return &resp, nil
}

// GetAllContractDeployments paginates through all deployment history pages for the given contract
// and returns the accumulated results.
//
// No error returns are expected during normal operation.
func (c *ExperimentalAPIClient) GetAllContractDeployments(
	ctx context.Context,
	identifier string,
	pageSize int,
) ([]swagger.ContractDeployment, error) {
	var all []swagger.ContractDeployment
	opts := &swagger.ContractsApiGetContractDeploymentsOpts{
		Limit: optional.NewInt32(int32(pageSize)),
	}
	for {
		resp, err := c.GetContractDeployments(ctx, identifier, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get contract deployments page: %w", err)
		}
		all = append(all, resp.Deployments...)
		if resp.NextCursor == "" {
			break
		}
		opts.Cursor = optional.NewInterface(resp.NextCursor)
	}
	return all, nil
}

// GetContracts fetches a single page of contracts (latest deployment per contract).
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetContracts(
	ctx context.Context,
	opts *swagger.ContractsApiGetContractsOpts,
) (*swagger.ContractsResponse, error) {
	resp, _, err := c.client.ContractsApi.GetContracts(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("contracts list API request failed: %w", err)
	}
	return &resp, nil
}

// GetAllContracts paginates through all contracts pages and returns the accumulated results.
// expand optionally specifies fields to expand inline (e.g. []string{"code"}).
//
// No error returns are expected during normal operation.
func (c *ExperimentalAPIClient) GetAllContracts(
	ctx context.Context,
	pageSize int,
	expand []string,
) ([]swagger.ContractDeployment, error) {
	var all []swagger.ContractDeployment
	opts := &swagger.ContractsApiGetContractsOpts{
		Limit: optional.NewInt32(int32(pageSize)),
	}
	if len(expand) > 0 {
		opts.Expand = optional.NewInterface(expand)
	}
	for {
		resp, err := c.GetContracts(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get contracts page: %w", err)
		}
		all = append(all, resp.Contracts...)
		if resp.NextCursor == "" {
			break
		}
		opts.Cursor = optional.NewInterface(resp.NextCursor)
	}
	return all, nil
}

// GetContractsByAccount fetches a single page of contracts deployed to the given account.
//
// Expected error returns during normal operation:
//   - Returns an error with the HTTP status code and response body for non-200 responses.
func (c *ExperimentalAPIClient) GetContractsByAccount(
	ctx context.Context,
	address string,
	opts *swagger.ContractsApiGetContractsByAccountOpts,
) (*swagger.ContractsResponse, error) {
	resp, _, err := c.client.ContractsApi.GetContractsByAccount(ctx, address, opts)
	if err != nil {
		return nil, fmt.Errorf("contracts by account API request failed for %s: %w", address, err)
	}
	return &resp, nil
}

// GetAllContractsByAccount paginates through all contract pages for the given account and
// returns the accumulated results.
// expand optionally specifies fields to expand inline (e.g. []string{"code"}).
//
// No error returns are expected during normal operation.
func (c *ExperimentalAPIClient) GetAllContractsByAccount(
	ctx context.Context,
	address string,
	pageSize int,
	expand []string,
) ([]swagger.ContractDeployment, error) {
	var all []swagger.ContractDeployment
	opts := &swagger.ContractsApiGetContractsByAccountOpts{
		Limit: optional.NewInt32(int32(pageSize)),
	}
	if len(expand) > 0 {
		opts.Expand = optional.NewInterface(expand)
	}
	for {
		resp, err := c.GetContractsByAccount(ctx, address, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get contracts by account page: %w", err)
		}
		all = append(all, resp.Contracts...)
		if resp.NextCursor == "" {
			break
		}
		opts.Cursor = optional.NewInterface(resp.NextCursor)
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
