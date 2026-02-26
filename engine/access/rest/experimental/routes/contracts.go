package routes

import (
	"net/http"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// GetContracts handles GET /experimental/v1/contracts.
func GetContracts(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
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

	return buildContractDeploymentsResponse(page)
}

// GetContract handles GET /experimental/v1/contracts/{identifier}.
func GetContract(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
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
	m.Build(deployment)
	return m, nil
}

// GetContractDeployments handles GET /experimental/v1/contracts/{identifier}/deployments.
func GetContractDeployments(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
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

	return buildContractDeploymentsResponse(page)
}

// GetContractsByAddress handles GET /experimental/v1/contracts/account/{address}.
func GetContractsByAddress(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
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

	return buildContractDeploymentsResponse(page)
}

// buildContractDeploymentsResponse converts a [accessmodel.ContractDeploymentPage] to a REST
// response, encoding the next cursor if present.
func buildContractDeploymentsResponse(
	page *accessmodel.ContractDeploymentPage,
) (models.ContractDeploymentsResponse, error) {
	contracts := make([]models.ContractDeployment, len(page.Deployments))
	for i := range page.Deployments {
		contracts[i].Build(&page.Deployments[i])
	}

	var nextCursor string
	if page.NextCursor != nil {
		var err error
		nextCursor, err = request.EncodeContractDeploymentCursor(page.NextCursor)
		if err != nil {
			return models.ContractDeploymentsResponse{}, common.NewRestError(http.StatusInternalServerError, "failed to encode next cursor", err)
		}
	}

	return models.ContractDeploymentsResponse{
		Contracts:  contracts,
		NextCursor: nextCursor,
	}, nil
}
