package events

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// FTWithdrawnEvent represents a decoded FungibleToken.Withdrawn event.
type FTWithdrawnEvent struct {
	Type          string         // Token type identifier (e.g., "A.f233dcee88fe0abe.FlowToken.Vault")
	Amount        cadence.UFix64 // Amount in UFix64 (Cadence-side denomination)
	From          flow.Address   // Address the tokens were withdrawn from
	FromUUID      uint64         // UUID of the source vault
	WithdrawnUUID uint64         // UUID of the withdrawn vault
	BalanceAfter  cadence.UFix64 // Balance after the tokens were withdrawn (in UFix64)
}

// FTDepositedEvent represents a decoded FungibleToken.Deposited event.
type FTDepositedEvent struct {
	Type          string         // Token type identifier (e.g., "A.f233dcee88fe0abe.FlowToken.Vault")
	Amount        cadence.UFix64 // Amount in UFix64 (Cadence-side denomination)
	To            flow.Address   // Address the tokens were deposited to
	ToUUID        uint64         // UUID of the destination vault
	DepositedUUID uint64         // UUID of the deposited vault
	BalanceAfter  cadence.UFix64 // Balance after the tokens were deposited (in UFix64)
}

// DecodeFTDeposited extracts fields from a FungibleToken.Deposited event.
//
// Any error indicates that the event is malformed.
func DecodeFTDeposited(event cadence.Event) (*FTDepositedEvent, error) {
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

	to, err := AddressFromOptional(raw.To)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FT deposited 'to' field: %w", err)
	}

	return &FTDepositedEvent{
		Type:          raw.Type,
		Amount:        raw.Amount,
		To:            to,
		ToUUID:        raw.ToUUID,
		DepositedUUID: raw.DepositedUUID,
		BalanceAfter:  raw.BalanceAfter,
	}, nil
}

// DecodeFTWithdrawn extracts fields from a FungibleToken.Withdrawn event.
//
// Any error indicates that the event is malformed.
func DecodeFTWithdrawn(event cadence.Event) (*FTWithdrawnEvent, error) {
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

	from, err := AddressFromOptional(raw.From)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FT withdrawn 'from' field: %w", err)
	}

	return &FTWithdrawnEvent{
		Type:          raw.Type,
		Amount:        raw.Amount,
		From:          from,
		FromUUID:      raw.FromUUID,
		WithdrawnUUID: raw.WithdrawnUUID,
		BalanceAfter:  raw.BalanceAfter,
	}, nil
}

// FlowFeesEvent represents a decoded FlowFees.FeesDeducted event.
type FlowFeesEvent struct {
	Amount          cadence.UFix64
	ExecutionEffort uint64
	InclusionEffort uint64
}

// DecodeFlowFees extracts fields from a FlowFees.FeesDeducted event.
//
// Any error indicates that the event is malformed.
func DecodeFlowFees(event cadence.Event) (*FlowFeesEvent, error) {
	type flowFeesEventRaw struct {
		Amount          cadence.UFix64 `cadence:"amount"`
		ExecutionEffort uint64         `cadence:"executionEffort"`
		InclusionEffort uint64         `cadence:"inclusionEffort"`
	}

	var raw flowFeesEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode FlowFees.FeesDeducted event: %w", err)
	}

	return &FlowFeesEvent{
		Amount:          raw.Amount,
		ExecutionEffort: raw.ExecutionEffort,
		InclusionEffort: raw.InclusionEffort,
	}, nil
}
