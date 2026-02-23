package models

import (
	"strconv"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const expandableHandlerContract = "handler_contract"

// Build populates a [ScheduledTransaction] from a domain model.
func (t *ScheduledTransaction) Build(
	tx *accessmodel.ScheduledTransaction,
	link commonmodels.LinkGenerator,
	expand map[string]bool,
) {
	t.Id = strconv.FormatUint(tx.ID, 10)
	priority := scheduledTxPriority(tx.Priority)
	t.Priority = &priority
	status := scheduledTxStatus(tx.Status)
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
	if tx.ScheduledTransactionID != flow.ZeroID {
		t.ScheduledTransactionId = tx.ScheduledTransactionID.String()
	}
	if tx.ExecutedTransactionID != flow.ZeroID {
		t.ExecutedTransactionId = tx.ExecutedTransactionID.String()
	}
	if tx.CancelledTransactionID != flow.ZeroID {
		t.CancelledTransactionId = tx.CancelledTransactionID.String()
	}
	if tx.FailedTransactionID != flow.ZeroID {
		t.FailedTransactionId = tx.FailedTransactionID.String()
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

	// HandlerContract expansion is not yet supported by the domain model.
	t.Expandable.HandlerContract = expandableHandlerContract
}

// scheduledTxStatus returns the [ScheduledTransactionStatus] for a domain status value.
func scheduledTxStatus(s accessmodel.ScheduledTxStatus) ScheduledTransactionStatus {
	switch s {
	case accessmodel.ScheduledTxStatusScheduled:
		return SCHEDULED_ScheduledTransactionStatus
	case accessmodel.ScheduledTxStatusExecuted:
		return EXECUTED_ScheduledTransactionStatus
	case accessmodel.ScheduledTxStatusCancelled:
		return CANCELLED_ScheduledTransactionStatus
	case accessmodel.ScheduledTxStatusFailed:
		return FAILED_ScheduledTransactionStatus
	default:
		return "unknown"
	}
}

// scheduledTxPriority returns the [ScheduledTransactionPriority] for a domain priority value.
// The contract encodes priority as: 0 = high, 1 = medium, 2 = low.
func scheduledTxPriority(p uint8) ScheduledTransactionPriority {
	switch p {
	case 0:
		return HIGH_ScheduledTransactionPriority
	case 1:
		return MEDIUM_ScheduledTransactionPriority
	default:
		return LOW_ScheduledTransactionPriority
	}
}
