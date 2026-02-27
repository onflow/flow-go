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

// GetScheduledTransactions holds parsed request params for the list endpoints.
type GetScheduledTransactions struct {
	Address       *flow.Address
	Limit         uint32
	Cursor        *accessmodel.ScheduledTransactionCursor
	Filter        extended.ScheduledTransactionFilter
	ExpandOptions extended.ScheduledTransactionExpandOptions
}

// NewGetScheduledTransactions parses and validates the HTTP request for GET /scheduled.
//
// All errors indicate an invalid request.
func NewGetScheduledTransactions(r *common.Request) (GetScheduledTransactions, error) {
	var req GetScheduledTransactions

	if raw := r.GetQueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return req, fmt.Errorf("invalid limit: %w", err)
		}
		req.Limit = uint32(parsed)
	}

	if raw := r.GetQueryParam("cursor"); raw != "" {
		c, err := parseScheduledTxCursor(raw)
		if err != nil {
			return req, err
		}
		req.Cursor = c
	}

	if err := parseScheduledTxFilter(r, &req.Filter); err != nil {
		return req, err
	}

	req.ExpandOptions = parseExpandOptions(r)

	return req, nil
}

// GetScheduledTransactions holds parsed request params for the list endpoints.
type GetAccountScheduledTransactions struct {
	GetScheduledTransactions
	Address flow.Address
}

// NewGetScheduledTransactionsByAddress parses GET /accounts/{address}/scheduled.
//
// All errors indicate an invalid request.
func NewGetScheduledTransactionsByAddress(r *common.Request) (GetAccountScheduledTransactions, error) {
	req, err := NewGetScheduledTransactions(r)
	if err != nil {
		return GetAccountScheduledTransactions{}, err
	}

	address, err := parser.ParseAddress(r.GetVar("address"), r.Chain)
	if err != nil {
		return GetAccountScheduledTransactions{}, err
	}

	return GetAccountScheduledTransactions{
		GetScheduledTransactions: req,
		Address:                  address,
	}, nil
}

// GetScheduledTransaction holds parsed request params for the single-transaction endpoint.
type GetScheduledTransaction struct {
	ID            uint64
	ExpandOptions extended.ScheduledTransactionExpandOptions
}

// NewGetScheduledTransaction parses GET /scheduled/transaction/{id}.
//
// All errors indicate an invalid request.
func NewGetScheduledTransaction(r *common.Request) (GetScheduledTransaction, error) {
	var req GetScheduledTransaction

	id, err := strconv.ParseUint(r.GetVar("id"), 10, 64)
	if err != nil {
		return req, fmt.Errorf("invalid scheduled transaction ID: %w", err)
	}
	req.ID = id
	req.ExpandOptions = parseExpandOptions(r)

	return req, nil
}

// parseScheduledTxFilter parses all optional filter query params from r into filter.
//
// All errors indicate an invalid request.
func parseScheduledTxFilter(r *common.Request, filter *extended.ScheduledTransactionFilter) error {
	if raw := r.GetQueryParam("status"); raw != "" {
		rawStatuses := strings.Split(raw, ",")
		for _, rawStatus := range rawStatuses {
			s, err := accessmodel.ParseScheduledTransactionStatus(rawStatus)
			if err != nil {
				return fmt.Errorf("invalid status: %w", err)
			}
			filter.Statuses = append(filter.Statuses, s)
		}
	}

	if raw := r.GetQueryParam("priority"); raw != "" {
		p, err := accessmodel.ParseScheduledTransactionPriority(raw)
		if err != nil {
			return fmt.Errorf("invalid priority: %w", err)
		}
		filter.Priority = &p
	}

	if raw := r.GetQueryParam("start_time"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid start_time: %w", err)
		}
		filter.StartTime = &v
	}

	if raw := r.GetQueryParam("end_time"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid end_time: %w", err)
		}
		filter.EndTime = &v
	}

	if raw := r.GetQueryParam("handler_owner"); raw != "" {
		addr, err := parser.ParseAddress(raw, r.Chain)
		if err != nil {
			return fmt.Errorf("invalid handler_owner: %w", err)
		}
		filter.TransactionHandlerOwner = &addr
	}

	if raw := r.GetQueryParam("handler_type_id"); raw != "" {
		filter.TransactionHandlerTypeID = &raw
	}

	if raw := r.GetQueryParam("handler_uuid"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid handler_uuid: %w", err)
		}
		filter.TransactionHandlerUUID = &v
	}

	return nil
}

// parseExpandOptions parses the expand options from the request.
func parseExpandOptions(r *common.Request) extended.ScheduledTransactionExpandOptions {
	return extended.ScheduledTransactionExpandOptions{
		Transaction:     r.Expands("transaction"),
		Result:          r.Expands("result"),
		HandlerContract: r.Expands("handler_contract"),
	}
}
