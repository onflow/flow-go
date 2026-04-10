package models

import (
	"fmt"
	"strconv"
	"time"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// Build populates a [ScheduledTransaction] from a domain model.
func (t *ScheduledTransaction) Build(
	tx *accessmodel.ScheduledTransaction,
	link LinkGenerator,
	expand map[string]bool,
) error {
	t.Id = strconv.FormatUint(tx.ID, 10)

	var priority ScheduledTransactionPriority
	if err := priority.Build(tx.Priority); err != nil {
		return fmt.Errorf("failed to build scheduled transaction priority: %w", err)
	}
	t.Priority = &priority

	var status ScheduledTransactionStatus
	if err := status.Build(tx.Status); err != nil {
		return fmt.Errorf("failed to build scheduled transaction status: %w", err)
	}
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

	t.IsPlaceholder = tx.IsPlaceholder

	if tx.CreatedAt != 0 {
		t.CreatedAt = time.UnixMilli(int64(tx.CreatedAt)).UTC().Format(time.RFC3339Nano)
	}
	if tx.CompletedAt != 0 {
		t.CompletedAt = time.UnixMilli(int64(tx.CompletedAt)).UTC().Format(time.RFC3339Nano)
	}

	t.Expandable = new(ScheduledTransactionExpandable)

	if tx.Transaction != nil {
		t.Transaction = new(commonmodels.Transaction)
		t.Transaction.Build(tx.Transaction, nil, link)
	} else if tx.ExecutedTransactionID != flow.ZeroID {
		transactionLink, err := link.TransactionLink(tx.ExecutedTransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate transaction link: %w", err)
		}
		t.Expandable.Transaction = transactionLink
	}

	if tx.Result != nil {
		t.Result = new(commonmodels.TransactionResult)
		t.Result.Build(tx.Result, tx.ExecutedTransactionID, link)
	} else if tx.ExecutedTransactionID != flow.ZeroID {
		resultLink, err := link.TransactionResultLink(tx.ExecutedTransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate result link: %w", err)
		}
		t.Expandable.Result = resultLink
	}

	if tx.HandlerContract != nil {
		t.HandlerContract = new(ContractDeployment)
		expandWithCode := map[string]bool{"code": true}
		if err := t.HandlerContract.Build(tx.HandlerContract, link, expandWithCode); err != nil {
			return err
		}
	} else {
		contractID, err := tx.HandlerContractID()
		if err != nil {
			return fmt.Errorf("failed to get handler contract ID: %w", err)
		}

		handlerContractLink, err := link.ContractCodeLink(contractID)
		if err != nil {
			return fmt.Errorf("failed to generate handler contract link: %w", err)
		}
		t.Expandable.HandlerContract = handlerContractLink
	}

	return nil
}

// Build sets the [ScheduledTransactionStatus] from a domain status value.
func (s *ScheduledTransactionStatus) Build(status accessmodel.ScheduledTransactionStatus) error {
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
		return fmt.Errorf("unknown scheduled transaction status: %d", status)
	}
	return nil
}

// Build sets the [ScheduledTransactionPriority] from a domain priority value.
// The contract encodes priority as: 0 = high, 1 = medium, 2 = low.
func (p *ScheduledTransactionPriority) Build(priority accessmodel.ScheduledTransactionPriority) error {
	switch priority {
	case accessmodel.ScheduledTxPriorityHigh:
		*p = HIGH_ScheduledTransactionPriority
	case accessmodel.ScheduledTxPriorityMedium:
		*p = MEDIUM_ScheduledTransactionPriority
	case accessmodel.ScheduledTxPriorityLow:
		*p = LOW_ScheduledTransactionPriority
	default:
		return fmt.Errorf("unknown scheduled transaction priority: %d", priority)
	}
	return nil
}
