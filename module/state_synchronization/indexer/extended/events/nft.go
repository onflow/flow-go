package events

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// NFTWithdrawnEvent represents a decoded NonFungibleToken.Withdrawn event.
type NFTWithdrawnEvent struct {
	Type         string
	ID           uint64 // NFT ID
	UUID         uint64
	From         flow.Address
	ProviderUUID uint64
}

// NFTDepositedEvent represents a decoded NonFungibleToken.Deposited event.
type NFTDepositedEvent struct {
	Type           string
	ID             uint64 // NFT ID
	UUID           uint64
	To             flow.Address
	CollectionUUID uint64
}

// DecodeNFTDeposited extracts fields from a NonFungibleToken.Deposited event.
//
// Any error indicates that the event is malformed.
func DecodeNFTDeposited(event cadence.Event) (*NFTDepositedEvent, error) {
	type nftDepositedEventRaw struct {
		Type           string           `cadence:"type"`
		ID             uint64           `cadence:"id"`
		UUID           uint64           `cadence:"uuid"`
		To             cadence.Optional `cadence:"to"`
		CollectionUUID uint64           `cadence:"collectionUUID"`
	}

	var raw nftDepositedEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode NFT deposited event: %w", err)
	}

	to, err := AddressFromOptional(raw.To)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NFT deposited 'to' field: %w", err)
	}

	return &NFTDepositedEvent{
		Type:           raw.Type,
		ID:             raw.ID,
		UUID:           raw.UUID,
		To:             to,
		CollectionUUID: raw.CollectionUUID,
	}, nil
}

// DecodeNFTWithdrawn extracts fields from a NonFungibleToken.Withdrawn event.
//
// Any error indicates that the event is malformed.
func DecodeNFTWithdrawn(event cadence.Event) (*NFTWithdrawnEvent, error) {
	type nftWithdrawnEventRaw struct {
		Type         string           `cadence:"type"`
		ID           uint64           `cadence:"id"`
		UUID         uint64           `cadence:"uuid"`
		From         cadence.Optional `cadence:"from"`
		ProviderUUID uint64           `cadence:"providerUUID"`
	}

	var raw nftWithdrawnEventRaw
	if err := cadence.DecodeFields(event, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode NFT withdrawn event: %w", err)
	}

	from, err := AddressFromOptional(raw.From)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NFT withdrawn 'from' field: %w", err)
	}

	return &NFTWithdrawnEvent{
		Type:         raw.Type,
		ID:           raw.ID,
		UUID:         raw.UUID,
		From:         from,
		ProviderUUID: raw.ProviderUUID,
	}, nil
}
