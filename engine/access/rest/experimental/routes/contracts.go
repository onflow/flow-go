package routes

import (
	"net/http"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/experimental/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// GetContracts handles GET /experimental/v1/contracts.
func GetContracts(r *common.Request, backend extended.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetContracts(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	page, err := backend.GetContracts(
		r.Context(),
		req.Limit,
		req.Cursor,
		req.Filter,
	)
	if err != nil {
		return nil, err
	}

	return buildContractsResponse(page, link)
}

// GetContract handles GET /experimental/v1/contracts/{identifier}.
func GetContract(r *common.Request, backend extended.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetContract(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	deployment, err := backend.GetContract(
		r.Context(),
		req.ID,
		req.Filter,
	)
	if err != nil {
		return nil, err
	}

	var m models.ContractDeployment
	m.Build(deployment, link)
	return m, nil
}

// GetContractDeployments handles GET /experimental/v1/contracts/{identifier}/deployments.
func GetContractDeployments(r *common.Request, backend extended.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetContractDeployments(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	page, err := backend.GetContractDeployments(
		r.Context(),
		req.ID,
		req.Limit,
		req.Cursor,
		req.Filter,
	)
	if err != nil {
		return nil, err
	}

	return buildContractDeploymentsResponse(page, link)
}

// GetContractsByAddress handles GET /experimental/v1/contracts/account/{address}.
func GetContractsByAddress(r *common.Request, backend extended.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetContractsByAddress(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	page, err := backend.GetContractsByAddress(
		r.Context(),
		req.Address,
		req.Limit,
		req.Cursor,
		req.Filter,
	)
	if err != nil {
		return nil, err
	}

	return buildContractsResponse(page, link)
}

// buildContractDeploymentsResponse converts a [accessmodel.ContractDeploymentPage] to a
// [models.ContractDeploymentsResponse] for the deployment history endpoint.
func buildContractDeploymentsResponse(
	page *accessmodel.ContractDeploymentPage,
	link models.LinkGenerator,
) (models.ContractDeploymentsResponse, error) {
	deployments := make([]models.ContractDeployment, len(page.Deployments))
	for i := range page.Deployments {
		deployments[i].Build(&page.Deployments[i], link)
	}

	nextCursor, err := encodeNextCursor(page)
	if err != nil {
		return models.ContractDeploymentsResponse{}, err
	}

	return models.ContractDeploymentsResponse{
		Deployments: deployments,
		NextCursor:  nextCursor,
	}, nil
}

// buildContractsResponse converts a [accessmodel.ContractDeploymentPage] to a
// [models.ContractsResponse] for the list and by-address endpoints.
func buildContractsResponse(
	page *accessmodel.ContractDeploymentPage,
	link models.LinkGenerator,
) (models.ContractsResponse, error) {
	contracts := make([]models.ContractDeployment, len(page.Deployments))
	for i := range page.Deployments {
		contracts[i].Build(&page.Deployments[i], link)
	}

	nextCursor, err := encodeNextCursor(page)
	if err != nil {
		return models.ContractsResponse{}, err
	}

	return models.ContractsResponse{
		Contracts:  contracts,
		NextCursor: nextCursor,
	}, nil
}

// encodeNextCursor encodes the next cursor for a contract deployment page, returning an
// empty string if there is no next page.
func encodeNextCursor(page *accessmodel.ContractDeploymentPage) (string, error) {
	if page.NextCursor == nil {
		return "", nil
	}
	cursor, err := request.EncodeContractDeploymentCursor(page.NextCursor)
	if err != nil {
		return "", common.NewRestError(http.StatusInternalServerError, "failed to encode next cursor", err)
	}
	return cursor, nil
}
