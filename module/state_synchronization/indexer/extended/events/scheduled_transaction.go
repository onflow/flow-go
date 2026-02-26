package events

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionSchedulerScheduledEvent represents a decoded FlowTransactionScheduler.Scheduled event,
// emitted when a new scheduled transaction is registered.
type TransactionSchedulerScheduledEvent struct {
	ID                               uint64
	Priority                         uint8
	Timestamp                        cadence.UFix64
	ExecutionEffort                  uint64
	Fees                             cadence.UFix64
	TransactionHandlerOwner          flow.Address
	TransactionHandlerTypeIdentifier string
	TransactionHandlerUUID           uint64
	TransactionHandlerPublicPath     string
}

// TransactionSchedulerPendingExecutionEvent represents a decoded FlowTransactionScheduler.PendingExecution event,
// emitted when a scheduled transaction's timestamp is reached and it is ready for execution.
type TransactionSchedulerPendingExecutionEvent struct {
	ID                               uint64
	Priority                         uint8
	ExecutionEffort                  uint64
	Fees                             cadence.UFix64
	TransactionHandlerOwner          flow.Address
	TransactionHandlerTypeIdentifier string
}

// TransactionSchedulerExecutedEvent represents a decoded FlowTransactionScheduler.Executed event,
// emitted when a scheduled transaction has been successfully executed.
type TransactionSchedulerExecutedEvent struct {
	ID                               uint64
	Priority                         uint8
	ExecutionEffort                  uint64
	TransactionHandlerOwner          flow.Address
	TransactionHandlerTypeIdentifier string
	TransactionHandlerUUID           uint64
	TransactionHandlerPublicPath     string
}

// TransactionSchedulerCanceledEvent represents a decoded FlowTransactionScheduler.Canceled event,
// emitted when a scheduled transaction is cancelled by its creator.
type TransactionSchedulerCanceledEvent struct {
	ID                               uint64
	Priority                         uint8
	FeesReturned                     cadence.UFix64
	FeesDeducted                     cadence.UFix64
	TransactionHandlerOwner          flow.Address
	TransactionHandlerTypeIdentifier string
}

// DecodeTransactionSchedulerScheduled extracts fields from a FlowTransactionScheduler.Scheduled event.
//
// Any error indicates that the event is malformed.
func DecodeTransactionSchedulerScheduled(event cadence.Event) (*TransactionSchedulerScheduledEvent, error) {
	type scheduledEventRaw struct {
		ID                               uint64           `cadence:"id"`
		Priority                         uint8            `cadence:"priority"`
		Timestamp                        cadence.UFix64   `cadence:"timestamp"`
		ExecutionEffort                  uint64           `cadence:"executionEffort"`
		Fees                             cadence.UFix64   `cadence:"fees"`
		TransactionHandlerOwner          cadence.Address  `cadence:"transactionHandlerOwner"`
		TransactionHandlerTypeIdentifier string           `cadence:"transactionHandlerTypeIdentifier"`
		TransactionHandlerUUID           uint64           `cadence:"transactionHandlerUUID"`
		TransactionHandlerPublicPath     cadence.Optional `cadence:"transactionHandlerPublicPath"`
	}

	var raw scheduledEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode Scheduled event: %w", err)
	}

	publicPath, err := PathFromOptional(raw.TransactionHandlerPublicPath)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Scheduled 'transactionHandlerPublicPath' field: %w", err)
	}

	return &TransactionSchedulerScheduledEvent{
		ID:                               raw.ID,
		Priority:                         raw.Priority,
		Timestamp:                        raw.Timestamp,
		ExecutionEffort:                  raw.ExecutionEffort,
		Fees:                             raw.Fees,
		TransactionHandlerOwner:          flow.Address(raw.TransactionHandlerOwner),
		TransactionHandlerTypeIdentifier: raw.TransactionHandlerTypeIdentifier,
		TransactionHandlerUUID:           raw.TransactionHandlerUUID,
		TransactionHandlerPublicPath:     publicPath,
	}, nil
}

// DecodeTransactionSchedulerPendingExecution extracts fields from a FlowTransactionScheduler.PendingExecution event.
//
// Any error indicates that the event is malformed.
func DecodeTransactionSchedulerPendingExecution(event cadence.Event) (*TransactionSchedulerPendingExecutionEvent, error) {
	type pendingExecutionEventRaw struct {
		ID                               uint64          `cadence:"id"`
		Priority                         uint8           `cadence:"priority"`
		ExecutionEffort                  uint64          `cadence:"executionEffort"`
		Fees                             cadence.UFix64  `cadence:"fees"`
		TransactionHandlerOwner          cadence.Address `cadence:"transactionHandlerOwner"`
		TransactionHandlerTypeIdentifier string          `cadence:"transactionHandlerTypeIdentifier"`
	}

	var raw pendingExecutionEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode PendingExecution event: %w", err)
	}

	return &TransactionSchedulerPendingExecutionEvent{
		ID:                               raw.ID,
		Priority:                         raw.Priority,
		ExecutionEffort:                  raw.ExecutionEffort,
		Fees:                             raw.Fees,
		TransactionHandlerOwner:          flow.Address(raw.TransactionHandlerOwner),
		TransactionHandlerTypeIdentifier: raw.TransactionHandlerTypeIdentifier,
	}, nil
}

// DecodeTransactionSchedulerExecuted extracts fields from a FlowTransactionScheduler.Executed event.
//
// Any error indicates that the event is malformed.
func DecodeTransactionSchedulerExecuted(event cadence.Event) (*TransactionSchedulerExecutedEvent, error) {
	type executedEventRaw struct {
		ID                               uint64           `cadence:"id"`
		Priority                         uint8            `cadence:"priority"`
		ExecutionEffort                  uint64           `cadence:"executionEffort"`
		TransactionHandlerOwner          cadence.Address  `cadence:"transactionHandlerOwner"`
		TransactionHandlerTypeIdentifier string           `cadence:"transactionHandlerTypeIdentifier"`
		TransactionHandlerUUID           uint64           `cadence:"transactionHandlerUUID"`
		TransactionHandlerPublicPath     cadence.Optional `cadence:"transactionHandlerPublicPath"`
	}

	var raw executedEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode Executed event: %w", err)
	}

	publicPath, err := PathFromOptional(raw.TransactionHandlerPublicPath)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Executed 'transactionHandlerPublicPath' field: %w", err)
	}

	return &TransactionSchedulerExecutedEvent{
		ID:                               raw.ID,
		Priority:                         raw.Priority,
		ExecutionEffort:                  raw.ExecutionEffort,
		TransactionHandlerOwner:          flow.Address(raw.TransactionHandlerOwner),
		TransactionHandlerTypeIdentifier: raw.TransactionHandlerTypeIdentifier,
		TransactionHandlerUUID:           raw.TransactionHandlerUUID,
		TransactionHandlerPublicPath:     publicPath,
	}, nil
}

// DecodeTransactionSchedulerCanceled extracts fields from a FlowTransactionScheduler.Canceled event.
//
// Any error indicates that the event is malformed.
func DecodeTransactionSchedulerCanceled(event cadence.Event) (*TransactionSchedulerCanceledEvent, error) {
	type canceledEventRaw struct {
		ID                               uint64          `cadence:"id"`
		Priority                         uint8           `cadence:"priority"`
		FeesReturned                     cadence.UFix64  `cadence:"feesReturned"`
		FeesDeducted                     cadence.UFix64  `cadence:"feesDeducted"`
		TransactionHandlerOwner          cadence.Address `cadence:"transactionHandlerOwner"`
		TransactionHandlerTypeIdentifier string          `cadence:"transactionHandlerTypeIdentifier"`
	}

	var raw canceledEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode Canceled event: %w", err)
	}

	return &TransactionSchedulerCanceledEvent{
		ID:                               raw.ID,
		Priority:                         raw.Priority,
		FeesReturned:                     raw.FeesReturned,
		FeesDeducted:                     raw.FeesDeducted,
		TransactionHandlerOwner:          flow.Address(raw.TransactionHandlerOwner),
		TransactionHandlerTypeIdentifier: raw.TransactionHandlerTypeIdentifier,
	}, nil
}
