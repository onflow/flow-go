package transfers

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const (
	ftDepositedSuffix = "FungibleToken.Deposited"
	ftWithdrawnSuffix = "FungibleToken.Withdrawn"
)

// ftPairedResult holds a matched withdrawal/deposit pair, or an unpaired event.
// For paired events, both withdrawal and deposit are set.
// For mints (deposit-only), withdrawal is nil.
// For burns (withdrawal-only), deposit is nil.
type ftPairedResult struct {
	sourceEvents []flow.Event      // the flow.Event(s) that produced this result
	withdrawal   *ftWithdrawnEvent // nil for deposit-only (mint)
	deposit      *ftDepositedEvent // nil for withdrawal-only (burn)
}

// FTParser decodes FungibleToken transfer events from CCF-encoded payloads and converts them
// into the model types used by the storage index.
//
// All methods are safe for concurrent access.
type FTParser struct {
	withdrawnEventType flow.EventType
	depositedEventType flow.EventType
}

// NewFTParser creates a new fungible token transfer event parser.
func NewFTParser(chainID flow.ChainID) *FTParser {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return &FTParser{
		withdrawnEventType: flow.EventType(fmt.Sprintf("A.%s.%s", sc.FungibleToken.Address, ftWithdrawnSuffix)),
		depositedEventType: flow.EventType(fmt.Sprintf("A.%s.%s", sc.FungibleToken.Address, ftDepositedSuffix)),
	}
}

// Parse extracts fungible token transfer events from the given events, pairs
// Withdrawn/Deposited events within each transaction, and returns fully-formed
// [access.FungibleTokenTransfer] objects.
//
// Events are paired by matching the `withdrawnUUID` field from Withdrawn events with the
// `depositedUUID` field from Deposited events within the same transaction. A single
// withdrawal may pair with multiple deposits (e.g. when a vault is split and deposited
// into several recipients). Each paired result uses the Deposited event's
// [flow.Event.EventIndex] and amount. Unpaired events produce records with a zero address
// for the missing side.
//
// No error returns are expected during normal operation.
func (p *FTParser) Parse(events []flow.Event, blockHeight uint64) ([]access.FungibleTokenTransfer, error) {
	groups, err := p.filterAndDecodeFT(events)
	if err != nil {
		return nil, err
	}

	paired := make([]ftPairedResult, 0)
	for _, group := range groups {
		paired = append(paired, group.ResolvePairs()...)
	}

	return p.buildTransfers(paired, blockHeight)
}

// filterAndDecodeFT filters events by type, decodes CCF payloads into typed domain events,
// and groups the results by transaction index.
//
// No error returns are expected during normal operation.
func (p *FTParser) filterAndDecodeFT(events []flow.Event) (map[uint32]*ftTxEventGroup, error) {
	txEventGroups := make(map[uint32]*ftTxEventGroup)

	ensureGroup := func(txIndex uint32) *ftTxEventGroup {
		g, ok := txEventGroups[txIndex]
		if !ok {
			g = newFTTxEventGroup()
			txEventGroups[txIndex] = g
		}
		return g
	}

	for _, event := range events {
		switch event.Type {
		case p.withdrawnEventType:
			cadenceEvent, err := decodeEvent(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			decoded, err := decodeFTWithdrawn(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode withdrawn event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			g := ensureGroup(event.TransactionIndex)
			err = g.addWithdrawal(event, decoded)
			if err != nil {
				return nil, fmt.Errorf("failed to add withdrawal event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}

		case p.depositedEventType:
			cadenceEvent, err := decodeEvent(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			decoded, err := decodeFTDeposited(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode deposited event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			g := ensureGroup(event.TransactionIndex)
			err = g.addDeposit(event, decoded)
			if err != nil {
				return nil, fmt.Errorf("failed to add deposit event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
		}
	}

	return txEventGroups, nil
}

// buildTransfers converts paired results into [access.FungibleTokenTransfer] model objects.
//
// No error returns are expected during normal operation.
func (p *FTParser) buildTransfers(paired []ftPairedResult, blockHeight uint64) ([]access.FungibleTokenTransfer, error) {
	transfers := make([]access.FungibleTokenTransfer, 0, len(paired))
	for _, pair := range paired {
		if len(pair.sourceEvents) == 0 {
			return nil, fmt.Errorf("paired result has no source events")
		}
		if pair.withdrawal == nil && pair.deposit == nil {
			return nil, fmt.Errorf("paired result has neither withdrawal nor deposit (events=%v)", pair.sourceEvents)
		}

		txID := pair.sourceEvents[0].TransactionID
		txIndex := pair.sourceEvents[0].TransactionIndex
		eventIndices := make([]uint32, len(pair.sourceEvents))
		eventIndicesStr := make([]string, len(pair.sourceEvents))
		for i, event := range pair.sourceEvents {
			eventIndices[i] = event.EventIndex
			eventIndicesStr[i] = strconv.Itoa(int(event.EventIndex))

			if txID != event.TransactionID {
				return nil, fmt.Errorf("transaction ID mismatch for source event: %s != %s (tx=%d, evtIdx=%s)",
					txID, event.TransactionID, txIndex, strings.Join(eventIndicesStr, ","))
			}
			if txIndex != event.TransactionIndex {
				return nil, fmt.Errorf("transaction index mismatch for source event: %d != %d (tx=%d, evtIdx=%s)",
					txIndex, event.TransactionIndex, txIndex, strings.Join(eventIndicesStr, ","))
			}
		}

		transfer := access.FungibleTokenTransfer{
			BlockHeight:      blockHeight,
			TransactionID:    txID,
			TransactionIndex: txIndex,
			EventIndices:     eventIndices,
		}

		if pair.withdrawal != nil {
			transfer.TokenType = pair.withdrawal.Type
			transfer.Amount = new(big.Int).SetUint64(uint64(pair.withdrawal.Amount))
			transfer.SourceAddress = pair.withdrawal.From
		}

		// Deposit amount takes precedence since a single withdrawal may be split across multiple deposits
		if pair.deposit != nil {
			transfer.TokenType = pair.deposit.Type
			transfer.Amount = new(big.Int).SetUint64(uint64(pair.deposit.Amount))
			transfer.RecipientAddress = pair.deposit.To
		}

		if transfer.TokenType == "" {
			return nil, fmt.Errorf("token type is empty for transfer (tx=%s, evtIdx=%s)",
				txID, strings.Join(eventIndicesStr, ","))
		}

		if transfer.Amount == nil {
			return nil, fmt.Errorf("amount is empty for transfer (tx=%s, evtIdx=%s)",
				txID, strings.Join(eventIndicesStr, ","))
		}

		transfers = append(transfers, transfer)
	}
	return transfers, nil
}
