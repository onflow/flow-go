package transfers

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// ftWithdrawnEvent represents a decoded FungibleToken.Withdrawn event.
type ftWithdrawnEvent struct {
	Type          string // Token type identifier (e.g., "A.f233dcee88fe0abe.FlowToken.Vault")
	Amount        cadence.UFix64
	From          flow.Address
	FromUUID      uint64
	WithdrawnUUID uint64
	BalanceAfter  cadence.UFix64
}

// ftDepositedEvent represents a decoded FungibleToken.Deposited event.
type ftDepositedEvent struct {
	Type          string
	Amount        cadence.UFix64
	To            flow.Address
	ToUUID        uint64
	DepositedUUID uint64
	BalanceAfter  cadence.UFix64
}

// decodeFTDeposited extracts fields from a FungibleToken.Deposited event.
//
// Any error indicates that the event is malformed.
func decodeFTDeposited(event cadence.Event) (*ftDepositedEvent, error) {
	type ftDepositedEventRaw struct {
		Type          string           `cadence:"type"`
		Amount        cadence.UFix64   `cadence:"amount"`
		To            cadence.Optional `cadence:"to"`
		ToUUID        uint64           `cadence:"toUUID"`
		DepositedUUID uint64           `cadence:"depositedUUID"`
		BalanceAfter  cadence.UFix64   `cadence:"balanceAfter"`
	}

	var raw ftDepositedEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode FT deposited event: %w", err)
	}

	to, err := addressFromOptional(raw.To)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FT deposited 'to' field: %w", err)
	}

	return &ftDepositedEvent{
		Type:          raw.Type,
		Amount:        raw.Amount,
		To:            to,
		ToUUID:        raw.ToUUID,
		DepositedUUID: raw.DepositedUUID,
		BalanceAfter:  raw.BalanceAfter,
	}, nil
}

// decodeFTWithdrawn extracts fields from a FungibleToken.Withdrawn event.
//
// Any error indicates that the event is malformed.
func decodeFTWithdrawn(event cadence.Event) (*ftWithdrawnEvent, error) {
	type ftWithdrawnEventRaw struct {
		Type          string           `cadence:"type"`
		Amount        cadence.UFix64   `cadence:"amount"`
		From          cadence.Optional `cadence:"from"`
		FromUUID      uint64           `cadence:"fromUUID"`
		WithdrawnUUID uint64           `cadence:"withdrawnUUID"`
		BalanceAfter  cadence.UFix64   `cadence:"balanceAfter"`
	}

	var raw ftWithdrawnEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode FT withdrawn event: %w", err)
	}

	from, err := addressFromOptional(raw.From)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FT withdrawn 'from' field: %w", err)
	}

	return &ftWithdrawnEvent{
		Type:          raw.Type,
		Amount:        raw.Amount,
		From:          from,
		FromUUID:      raw.FromUUID,
		WithdrawnUUID: raw.WithdrawnUUID,
		BalanceAfter:  raw.BalanceAfter,
	}, nil
}

// flowFeesEvent represents a decoded FlowFees.FeesDeducted event.
type flowFeesEvent struct {
	Amount          cadence.UFix64
	ExecutionEffort uint64
	InclusionEffort uint64
}

// decodeFlowFees extracts fields from a FlowFees.FeesDeducted event.
//
// Any error indicates that the event is malformed.
func decodeFlowFees(event cadence.Event) (*flowFeesEvent, error) {
	type flowFeesEventRaw struct {
		Amount          cadence.UFix64 `cadence:"amount"`
		ExecutionEffort uint64         `cadence:"executionEffort"`
		InclusionEffort uint64         `cadence:"inclusionEffort"`
	}

	var raw flowFeesEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode FlowFees.FeesDeducted event: %w", err)
	}

	return &flowFeesEvent{
		Amount:          raw.Amount,
		ExecutionEffort: raw.ExecutionEffort,
		InclusionEffort: raw.InclusionEffort,
	}, nil
}
