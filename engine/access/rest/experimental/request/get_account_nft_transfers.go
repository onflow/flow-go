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

// GetAccountNFTTransfers holds the parsed request parameters for the GetAccountNonFungibleTokenTransfers endpoint.
type GetAccountNFTTransfers struct {
	Address flow.Address
	Limit   uint32
	Cursor  *accessmodel.TransferCursor
	Filter  extended.AccountNFTTransferFilter
}

// NewGetAccountNFTTransfers parses and validates the HTTP request for the
// GetAccountNonFungibleTokenTransfers endpoint.
//
// All errors indicate the request is invalid.
func NewGetAccountNFTTransfers(r *common.Request) (GetAccountNFTTransfers, error) {
	var req GetAccountNFTTransfers

	address, err := parser.ParseAddress(r.GetVar("address"), r.Chain)
	if err != nil {
		return req, err
	}
	req.Address = address
	req.Filter.AccountAddress = address

	if raw := r.GetQueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return req, fmt.Errorf("invalid limit: %w", err)
		}
		req.Limit = uint32(parsed)
	}

	if raw := r.GetQueryParam("cursor"); raw != "" {
		c, err := parseTransferCursor(raw)
		if err != nil {
			return req, err
		}
		req.Cursor = c
	}

	if raw := r.GetQueryParam("token_type"); raw != "" {
		req.Filter.TokenType = raw
	}

	if raw := r.GetQueryParam("source_address"); raw != "" {
		addr, err := parser.ParseAddress(raw, r.Chain)
		if err != nil {
			return req, fmt.Errorf("invalid source_address: %w", err)
		}
		req.Filter.SourceAddress = addr
	}

	if raw := r.GetQueryParam("recipient_address"); raw != "" {
		addr, err := parser.ParseAddress(raw, r.Chain)
		if err != nil {
			return req, fmt.Errorf("invalid recipient_address: %w", err)
		}
		req.Filter.RecipientAddress = addr
	}

	if raw := r.GetQueryParam("role"); raw != "" {
		role, err := ParseTransferRole(raw)
		if err != nil {
			return req, err
		}
		req.Filter.TransferRole = role
	}

	return req, nil
}
