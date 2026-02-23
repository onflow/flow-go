package request

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// GetAccountTransactions holds the parsed request parameters for the GetAccountTransactions endpoint.
type GetAccountTransactions struct {
	Address flow.Address
	Limit   uint32
	Cursor  *accessmodel.AccountTransactionCursor
	Filter  extended.AccountTransactionFilter
}

// NewGetAccountTransactions parses and validates the HTTP request for the GetAccountTransactions endpoint.
//
// All errors indicate the request is invalid.
func NewGetAccountTransactions(r *common.Request) (GetAccountTransactions, error) {
	var req GetAccountTransactions

	address, err := parser.ParseAddress(r.GetVar("address"), r.Chain)
	if err != nil {
		return req, err
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
		c, err := parseAccountTransactionCursor(raw)
		if err != nil {
			return req, err
		}
		req.Cursor = c
	}

	if raw := r.GetQueryParam("roles"); raw != "" {
		for role := range strings.SplitSeq(raw, ",") {
			parsed, err := accessmodel.ParseTransactionRole(strings.TrimSpace(role))
			if err != nil {
				return req, fmt.Errorf("invalid role: %w", err)
			}
			req.Filter.Roles = append(req.Filter.Roles, parsed)
		}
	}

	return req, nil
}
