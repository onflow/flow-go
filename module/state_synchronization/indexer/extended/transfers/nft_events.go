package transfers

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// nftWithdrawnEvent represents a decoded NonFungibleToken.Withdrawn event.
type nftWithdrawnEvent struct {
	Type         string
	ID           uint64 // NFT ID
	UUID         uint64
	From         flow.Address
	ProviderUUID uint64
}

// nftDepositedEvent represents a decoded NonFungibleToken.Deposited event.
type nftDepositedEvent struct {
	Type           string
	ID             uint64 // NFT ID
	UUID           uint64
	To             flow.Address
	CollectionUUID uint64
}

// decodeNFTDeposited extracts fields from a NonFungibleToken.Deposited event.
//
// Any error indicates that the event is malformed.
func decodeNFTDeposited(event cadence.Event) (*nftDepositedEvent, error) {
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

	to, err := addressFromOptional(raw.To)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NFT deposited 'to' field: %w", err)
	}

	return &nftDepositedEvent{
		Type:           raw.Type,
		ID:             raw.ID,
		UUID:           raw.UUID,
		To:             to,
		CollectionUUID: raw.CollectionUUID,
	}, nil
}

// decodeNFTWithdrawn extracts fields from a NonFungibleToken.Withdrawn event.
//
// Any error indicates that the event is malformed.
func decodeNFTWithdrawn(event cadence.Event) (*nftWithdrawnEvent, error) {
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

	from, err := addressFromOptional(raw.From)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NFT withdrawn 'from' field: %w", err)
	}

	return &nftWithdrawnEvent{
		Type:         raw.Type,
		ID:           raw.ID,
		UUID:         raw.UUID,
		From:         from,
		ProviderUUID: raw.ProviderUUID,
	}, nil
}
