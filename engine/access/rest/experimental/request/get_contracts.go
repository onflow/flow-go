package request

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// GetContracts holds parsed request params for GET /contracts.
type GetContracts struct {
	Limit  uint32
	Cursor *accessmodel.ContractDeploymentCursor
	Filter extended.ContractDeploymentFilter
}

// NewGetContracts parses and validates the HTTP request for GET /contracts.
//
// All errors indicate an invalid request.
func NewGetContracts(r *common.Request) (GetContracts, error) {
	var req GetContracts

	if raw := r.GetQueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return req, fmt.Errorf("invalid limit: %w", err)
		}
		req.Limit = uint32(parsed)
	}

	if raw := r.GetQueryParam("cursor"); raw != "" {
		c, err := DecodeContractDeploymentCursor(raw)
		if err != nil {
			return req, err
		}
		req.Cursor = c
	}

	if err := parseContractFilter(r, &req.Filter); err != nil {
		return req, err
	}

	return req, nil
}

// GetContract holds parsed request params for GET /contracts/{identifier}.
type GetContract struct {
	ID     string
	Filter extended.ContractDeploymentFilter
}

// NewGetContract parses and validates the HTTP request for GET /contracts/{identifier}.
//
// All errors indicate an invalid request.
func NewGetContract(r *common.Request) (GetContract, error) {
	var req GetContract

	req.ID = r.GetVar("identifier")

	if err := parseContractFilter(r, &req.Filter); err != nil {
		return req, err
	}

	return req, nil
}

// GetContractDeployments holds parsed request params for GET /contracts/{identifier}/deployments.
type GetContractDeployments struct {
	ID     string
	Limit  uint32
	Cursor *accessmodel.ContractDeploymentCursor
	Filter extended.ContractDeploymentFilter
}

// NewGetContractDeployments parses and validates the HTTP request for
// GET /contracts/{identifier}/deployments.
//
// All errors indicate an invalid request.
func NewGetContractDeployments(r *common.Request) (GetContractDeployments, error) {
	var req GetContractDeployments

	req.ID = r.GetVar("identifier")

	if raw := r.GetQueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return req, fmt.Errorf("invalid limit: %w", err)
		}
		req.Limit = uint32(parsed)
	}

	if raw := r.GetQueryParam("cursor"); raw != "" {
		c, err := DecodeContractDeploymentCursor(raw)
		if err != nil {
			return req, err
		}
		req.Cursor = c
	}

	if err := parseContractFilter(r, &req.Filter); err != nil {
		return req, err
	}

	return req, nil
}

// GetContractsByAddress holds parsed request params for GET /contracts/account/{address}.
type GetContractsByAddress struct {
	Address flow.Address
	Limit   uint32
	Cursor  *accessmodel.ContractDeploymentCursor
	Filter  extended.ContractDeploymentFilter
}

// NewGetContractsByAddress parses and validates the HTTP request for
// GET /contracts/account/{address}.
//
// All errors indicate an invalid request.
func NewGetContractsByAddress(r *common.Request) (GetContractsByAddress, error) {
	var req GetContractsByAddress

	address, err := parser.ParseAddress(r.GetVar("address"), r.Chain)
	if err != nil {
		return req, fmt.Errorf("invalid address: %w", err)
	}
	req.Address = address

	if raw := r.GetQueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return req, fmt.Errorf("invalid limit: %w", err)
		}
		req.Limit = uint32(parsed)
	}

	if raw := r.GetQueryParam("cursor"); raw != "" {
		c, err := DecodeContractDeploymentCursor(raw)
		if err != nil {
			return req, err
		}
		req.Cursor = c
	}

	if err := parseContractFilter(r, &req.Filter); err != nil {
		return req, err
	}

	return req, nil
}

// parseContractFilter parses all optional filter query params from r into filter.
//
// All errors indicate an invalid request.
func parseContractFilter(r *common.Request, filter *extended.ContractDeploymentFilter) error {
	if raw := r.GetQueryParam("contract_name"); raw != "" {
		filter.ContractName = raw
	}

	if raw := r.GetQueryParam("start_block"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid start_block: %w", err)
		}
		filter.StartBlock = &v
	}

	if raw := r.GetQueryParam("end_block"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid end_block: %w", err)
		}
		filter.EndBlock = &v
	}

	return nil
}
