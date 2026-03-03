// Package testutil provides shared event-building helpers for FT transfer tests.
package testutil

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// FTTokenType is the Cadence type string used for FlowToken in test events.
	FTTokenType = "A.0000000000000001.FlowToken.Vault"

	// NFTTokenType is the Cadence type string used for a test NFT in test events.
	NFTTokenType = "A.0000000000000002.TopShot.NFT"

	ftWithdrawnFormat  = "A.%s.FungibleToken.Withdrawn"
	ftDepositedFormat  = "A.%s.FungibleToken.Deposited"
	flowFeesFormat     = "A.%s.FlowFees.FeesDeducted"
	nftWithdrawnFormat = "A.%s.NonFungibleToken.Withdrawn"
	nftDepositedFormat = "A.%s.NonFungibleToken.Deposited"
)

// FTWithdrawnEventType returns the FungibleToken.Withdrawn event type for the given chain.
func FTWithdrawnEventType(chainID flow.ChainID) flow.EventType {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return flow.EventType(fmt.Sprintf(ftWithdrawnFormat, sc.FungibleToken.Address))
}

// FTDepositedEventType returns the FungibleToken.Deposited event type for the given chain.
func FTDepositedEventType(chainID flow.ChainID) flow.EventType {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return flow.EventType(fmt.Sprintf(ftDepositedFormat, sc.FungibleToken.Address))
}

// FlowFeesEventType returns the FlowFees.FeesDeducted event type for the given chain.
func FlowFeesEventType(chainID flow.ChainID) flow.EventType {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return flow.EventType(fmt.Sprintf(flowFeesFormat, sc.FlowFees.Address))
}

// FlowFeesAddress returns the FlowFees contract address for the given chain.
func FlowFeesAddress(chainID flow.ChainID) flow.Address {
	return systemcontracts.SystemContractsForChain(chainID).FlowFees.Address
}

// MakeFTTransfer builds an [access.FungibleTokenTransfer] for use in test assertions.
func MakeFTTransfer(
	blockHeight uint64,
	txID flow.Identifier,
	txIndex uint32,
	source, recipient flow.Address,
	amount cadence.UFix64,
	eventIndices ...uint32,
) access.FungibleTokenTransfer {
	return access.FungibleTokenTransfer{
		TransactionID:    txID,
		BlockHeight:      blockHeight,
		TransactionIndex: txIndex,
		EventIndices:     eventIndices,
		TokenType:        FTTokenType,
		Amount:           new(big.Int).SetUint64(uint64(amount)),
		SourceAddress:    source,
		RecipientAddress: recipient,
	}
}

// MakeFTWithdrawnEvent creates a CCF-encoded FungibleToken.Withdrawn event.
func MakeFTWithdrawnEvent(
	t testing.TB,
	chainID flow.ChainID,
	fromAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	fromUUID, withdrawnUUID uint64,
	amount cadence.UFix64,
) flow.Event {
	sc := systemcontracts.SystemContractsForChain(chainID)
	eventType := flow.EventType(fmt.Sprintf(ftWithdrawnFormat, sc.FungibleToken.Address))

	var fromValue cadence.Value
	if fromAddr != nil {
		fromValue = cadence.NewOptional(cadence.NewAddress(*fromAddr))
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	loc := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	cadenceType := cadence.NewEventType(loc, "FungibleToken.Withdrawn", []cadence.Field{
		{Identifier: "type", Type: cadence.StringType},
		{Identifier: "amount", Type: cadence.UFix64Type},
		{Identifier: "from", Type: cadence.NewOptionalType(cadence.AddressType)},
		{Identifier: "fromUUID", Type: cadence.UInt64Type},
		{Identifier: "withdrawnUUID", Type: cadence.UInt64Type},
		{Identifier: "balanceAfter", Type: cadence.UFix64Type},
	}, nil)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String(FTTokenType),
		amount,
		fromValue,
		cadence.UInt64(fromUUID),
		cadence.UInt64(withdrawnUUID),
		cadence.UFix64(150_00000000),
	}).WithType(cadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	flowEvent, err := flow.NewEvent(flow.UntrustedEvent{
		Type:             eventType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	})
	require.NoError(t, err)
	return *flowEvent
}

// MakeFTDepositedEvent creates a CCF-encoded FungibleToken.Deposited event.
func MakeFTDepositedEvent(
	t testing.TB,
	chainID flow.ChainID,
	toAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	toUUID uint64,
	depositedUUID uint64,
	amount cadence.UFix64,
) flow.Event {
	sc := systemcontracts.SystemContractsForChain(chainID)
	eventType := flow.EventType(fmt.Sprintf(ftDepositedFormat, sc.FungibleToken.Address))

	var toValue cadence.Value
	if toAddr != nil {
		toValue = cadence.NewOptional(cadence.NewAddress(*toAddr))
	} else {
		toValue = cadence.NewOptional(nil)
	}

	loc := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	cadenceType := cadence.NewEventType(loc, "FungibleToken.Deposited", []cadence.Field{
		{Identifier: "type", Type: cadence.StringType},
		{Identifier: "amount", Type: cadence.UFix64Type},
		{Identifier: "to", Type: cadence.NewOptionalType(cadence.AddressType)},
		{Identifier: "toUUID", Type: cadence.UInt64Type},
		{Identifier: "depositedUUID", Type: cadence.UInt64Type},
		{Identifier: "balanceAfter", Type: cadence.UFix64Type},
	}, nil)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String(FTTokenType),
		amount,
		toValue,
		cadence.UInt64(toUUID),
		cadence.UInt64(depositedUUID),
		cadence.UFix64(200_00000000),
	}).WithType(cadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	flowEvent, err := flow.NewEvent(flow.UntrustedEvent{
		Type:             eventType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	})
	require.NoError(t, err)
	return *flowEvent
}

// NFTWithdrawnEventType returns the NonFungibleToken.Withdrawn event type for the given chain.
func NFTWithdrawnEventType(chainID flow.ChainID) flow.EventType {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return flow.EventType(fmt.Sprintf(nftWithdrawnFormat, sc.NonFungibleToken.Address))
}

// NFTDepositedEventType returns the NonFungibleToken.Deposited event type for the given chain.
func NFTDepositedEventType(chainID flow.ChainID) flow.EventType {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return flow.EventType(fmt.Sprintf(nftDepositedFormat, sc.NonFungibleToken.Address))
}

// MakeNFTTransfer builds an [access.NonFungibleTokenTransfer] for use in test assertions.
func MakeNFTTransfer(
	blockHeight uint64,
	txID flow.Identifier,
	txIndex uint32,
	source, recipient flow.Address,
	nftID uint64,
	eventIndices ...uint32,
) access.NonFungibleTokenTransfer {
	return access.NonFungibleTokenTransfer{
		TransactionID:    txID,
		BlockHeight:      blockHeight,
		TransactionIndex: txIndex,
		EventIndices:     eventIndices,
		TokenType:        NFTTokenType,
		ID:               nftID,
		SourceAddress:    source,
		RecipientAddress: recipient,
	}
}

// MakeNFTWithdrawnEvent creates a CCF-encoded NonFungibleToken.Withdrawn event.
func MakeNFTWithdrawnEvent(
	t testing.TB,
	chainID flow.ChainID,
	fromAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	uuid, nftID uint64,
) flow.Event {
	sc := systemcontracts.SystemContractsForChain(chainID)
	eventType := flow.EventType(fmt.Sprintf(nftWithdrawnFormat, sc.NonFungibleToken.Address))

	var fromValue cadence.Value
	if fromAddr != nil {
		fromValue = cadence.NewOptional(cadence.NewAddress(*fromAddr))
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	loc := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	cadenceType := cadence.NewEventType(loc, "NonFungibleToken.Withdrawn", []cadence.Field{
		{Identifier: "type", Type: cadence.StringType},
		{Identifier: "id", Type: cadence.UInt64Type},
		{Identifier: "uuid", Type: cadence.UInt64Type},
		{Identifier: "from", Type: cadence.NewOptionalType(cadence.AddressType)},
		{Identifier: "providerUUID", Type: cadence.UInt64Type},
	}, nil)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String(NFTTokenType),
		cadence.UInt64(nftID),
		cadence.UInt64(uuid),
		fromValue,
		cadence.UInt64(5),
	}).WithType(cadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	flowEvent, err := flow.NewEvent(flow.UntrustedEvent{
		Type:             eventType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	})
	require.NoError(t, err)
	return *flowEvent
}

// MakeNFTDepositedEvent creates a CCF-encoded NonFungibleToken.Deposited event.
func MakeNFTDepositedEvent(
	t testing.TB,
	chainID flow.ChainID,
	toAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	uuid, nftID uint64,
) flow.Event {
	sc := systemcontracts.SystemContractsForChain(chainID)
	eventType := flow.EventType(fmt.Sprintf(nftDepositedFormat, sc.NonFungibleToken.Address))

	var toValue cadence.Value
	if toAddr != nil {
		toValue = cadence.NewOptional(cadence.NewAddress(*toAddr))
	} else {
		toValue = cadence.NewOptional(nil)
	}

	loc := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	cadenceType := cadence.NewEventType(loc, "NonFungibleToken.Deposited", []cadence.Field{
		{Identifier: "type", Type: cadence.StringType},
		{Identifier: "id", Type: cadence.UInt64Type},
		{Identifier: "uuid", Type: cadence.UInt64Type},
		{Identifier: "to", Type: cadence.NewOptionalType(cadence.AddressType)},
		{Identifier: "collectionUUID", Type: cadence.UInt64Type},
	}, nil)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String(NFTTokenType),
		cadence.UInt64(nftID),
		cadence.UInt64(uuid),
		toValue,
		cadence.UInt64(5),
	}).WithType(cadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	flowEvent, err := flow.NewEvent(flow.UntrustedEvent{
		Type:             eventType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	})
	require.NoError(t, err)
	return *flowEvent
}

// MakeFlowFeesEvent creates a CCF-encoded FlowFees.FeesDeducted event.
func MakeFlowFeesEvent(
	t testing.TB,
	chainID flow.ChainID,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	amount cadence.UFix64,
) flow.Event {
	sc := systemcontracts.SystemContractsForChain(chainID)
	eventType := flow.EventType(fmt.Sprintf(flowFeesFormat, sc.FlowFees.Address))

	loc := common.NewAddressLocation(nil, common.Address{}, "FlowFees")
	cadenceType := cadence.NewEventType(loc, "FlowFees.FeesDeducted", []cadence.Field{
		{Identifier: "amount", Type: cadence.UFix64Type},
		{Identifier: "executionEffort", Type: cadence.UInt64Type},
		{Identifier: "inclusionEffort", Type: cadence.UInt64Type},
	}, nil)

	event := cadence.NewEvent([]cadence.Value{
		amount,
		cadence.UInt64(500),
		cadence.UInt64(1000),
	}).WithType(cadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	flowEvent, err := flow.NewEvent(flow.UntrustedEvent{
		Type:             eventType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	})
	require.NoError(t, err)
	return *flowEvent
}
