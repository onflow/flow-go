package models

import (
	"strconv"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const (
	expandableTransaction   = "transaction"
	expandableResult        = "result"
	expandableHandlerContract = "handler_contract"
)

// Build populates a [ScheduledTransaction] from a domain model.
func (t *ScheduledTransaction) Build(
	tx *accessmodel.ScheduledTransaction,
	link commonmodels.LinkGenerator,
	expand map[string]bool,
) {
	t.Id = strconv.FormatUint(tx.ID, 10)
	var priority ScheduledTransactionPriority
	priority.Build(tx.Priority)
	t.Priority = &priority
	var status ScheduledTransactionStatus
	status.Build(tx.Status)
	t.Status = &status
	t.Timestamp = strconv.FormatUint(tx.Timestamp, 10)
	t.ExecutionEffort = strconv.FormatUint(tx.ExecutionEffort, 10)
	t.Fees = strconv.FormatUint(tx.Fees, 10)
	t.TransactionHandlerOwner = tx.TransactionHandlerOwner.String()
	t.TransactionHandlerTypeIdentifier = tx.TransactionHandlerTypeIdentifier
	t.TransactionHandlerUuid = strconv.FormatUint(tx.TransactionHandlerUUID, 10)
	t.TransactionHandlerPublicPath = tx.TransactionHandlerPublicPath

	if tx.FeesReturned > 0 {
		t.FeesReturned = strconv.FormatUint(tx.FeesReturned, 10)
	}
	if tx.FeesDeducted > 0 {
		t.FeesDeducted = strconv.FormatUint(tx.FeesDeducted, 10)
	}
	if tx.CreatedTransactionID != flow.ZeroID {
		t.CreatedTransactionId = tx.CreatedTransactionID.String()
	}
	if tx.ExecutedTransactionID != flow.ZeroID {
		t.ExecutedTransactionId = tx.ExecutedTransactionID.String()
	}
	if tx.CancelledTransactionID != flow.ZeroID {
		t.CancelledTransactionId = tx.CancelledTransactionID.String()
	}

	t.Expandable = new(ScheduledTransactionExpandable)

	if expand[expandableTransaction] && tx.Transaction != nil {
		t.Transaction = new(commonmodels.Transaction)
		t.Transaction.Build(tx.Transaction, nil, link)
	} else {
		t.Expandable.Transaction = expandableTransaction
	}

	if expand[expandableResult] && tx.Result != nil {
		t.Result = new(commonmodels.TransactionResult)
		t.Result.Build(tx.Result, tx.ExecutedTransactionID, link)
	} else {
		t.Expandable.Result = expandableResult
	}

	if expand[expandableHandlerContract] && tx.HandlerContract != nil {
		t.HandlerContract = new(Contract)
		t.HandlerContract.Build(tx.HandlerContract)
	} else {
		t.Expandable.HandlerContract = expandableHandlerContract
	}
}

// Build sets the [ScheduledTransactionStatus] from a domain status value.
func (s *ScheduledTransactionStatus) Build(status accessmodel.ScheduledTransactionStatus) {
	switch status {
	case accessmodel.ScheduledTxStatusScheduled:
		*s = SCHEDULED_ScheduledTransactionStatus
	case accessmodel.ScheduledTxStatusExecuted:
		*s = EXECUTED_ScheduledTransactionStatus
	case accessmodel.ScheduledTxStatusCancelled:
		*s = CANCELLED_ScheduledTransactionStatus
	case accessmodel.ScheduledTxStatusFailed:
		*s = FAILED_ScheduledTransactionStatus
	default:
		*s = ""
	}
}

// Build sets the [ScheduledTransactionPriority] from a domain priority value.
// The contract encodes priority as: 0 = high, 1 = medium, 2 = low.
func (p *ScheduledTransactionPriority) Build(priority accessmodel.ScheduledTransactionPriority) {
	switch priority {
	case accessmodel.ScheduledTxPriorityHigh:
		*p = HIGH_ScheduledTransactionPriority
	case accessmodel.ScheduledTxPriorityMedium:
		*p = MEDIUM_ScheduledTransactionPriority
	default:
		*p = LOW_ScheduledTransactionPriority
	}
}
