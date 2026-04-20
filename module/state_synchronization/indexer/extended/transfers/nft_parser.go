package transfers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
)

const (
	nftWithdrawnFormat = "A.%s.NonFungibleToken.Withdrawn"
	nftDepositedFormat = "A.%s.NonFungibleToken.Deposited"
)

// nftPairedResult holds a matched withdrawal/deposit pair, or an unpaired event.
// For paired events, both withdrawal and deposit are set.
// For mints (deposit-only), withdrawal is nil.
// For burns (withdrawal-only), deposit is nil and recipientAddress is zero.
// For intermediate transfers (multi-layer collections), deposit is nil and recipientAddress is set.
type nftPairedResult struct {
	sourceEvents     []flow.Event              // the flow.Event(s) that produced this result
	withdrawal       *events.NFTWithdrawnEvent // nil for deposit-only (mint)
	deposit          *events.NFTDepositedEvent // nil for withdrawal-only (burn or intermediate transfer)
	recipientAddress flow.Address              // used for intermediate transfers when deposit is nil
}

// NFTParser decodes NonFungibleToken transfer events from CCF-encoded payloads and converts them
// into the model types used by the storage index.
//
// All methods are safe for concurrent access.
type NFTParser struct {
	withdrawnEventType flow.EventType
	depositedEventType flow.EventType
}

// NewNFTParser creates a new non-fungible token transfer event parser.
func NewNFTParser(chainID flow.ChainID) *NFTParser {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return &NFTParser{
		withdrawnEventType: flow.EventType(fmt.Sprintf(nftWithdrawnFormat, sc.NonFungibleToken.Address)),
		depositedEventType: flow.EventType(fmt.Sprintf(nftDepositedFormat, sc.NonFungibleToken.Address)),
	}
}

// Parse extracts non-fungible token transfer events from the given events, pairs
// Withdrawn/Deposited events within each transaction, and returns fully-formed
// [access.NonFungibleTokenTransfer] objects.
//
// Events are paired by matching the `uuid` field between Withdrawn and Deposited events within
// the same transaction. Each paired result uses the Deposited event's [flow.Event.EventIndex].
// Unpaired events produce records with a zero address for the missing side.
//
// No error returns are expected during normal operation.
func (p *NFTParser) Parse(evts []flow.Event, blockHeight uint64) ([]access.NonFungibleTokenTransfer, error) {
	groups, err := p.filterAndDecodeNFT(evts)
	if err != nil {
		return nil, err
	}

	paired := make([]nftPairedResult, 0)
	for _, group := range groups {
		paired = append(paired, group.ResolvePairs()...)
	}

	return p.buildTransfers(paired, blockHeight)
}

// filterAndDecodeNFT filters events by type, decodes CCF payloads into typed domain events,
// and groups the results by transaction index.
//
// No error returns are expected during normal operation.
func (p *NFTParser) filterAndDecodeNFT(evts []flow.Event) (map[uint32]*nftTxEventGroup, error) {
	txEventGroups := make(map[uint32]*nftTxEventGroup)

	ensureGroup := func(txIndex uint32) *nftTxEventGroup {
		g, ok := txEventGroups[txIndex]
		if !ok {
			g = newNFTTxEventGroup()
			txEventGroups[txIndex] = g
		}
		return g
	}

	for _, event := range evts {
		switch event.Type {
		case p.withdrawnEventType:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			decoded, err := events.DecodeNFTWithdrawn(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode withdrawn event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			g := ensureGroup(event.TransactionIndex)
			err = g.addWithdrawal(event, decoded)
			if err != nil {
				return nil, fmt.Errorf("failed to add withdrawal event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}

		case p.depositedEventType:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event %d in transaction %d: %w", event.EventIndex, event.TransactionIndex, err)
			}
			decoded, err := events.DecodeNFTDeposited(cadenceEvent)
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

// buildTransfers converts paired results into [access.NonFungibleTokenTransfer] model objects.
//
// No error returns are expected during normal operation.
func (p *NFTParser) buildTransfers(paired []nftPairedResult, blockHeight uint64) ([]access.NonFungibleTokenTransfer, error) {
	transfers := make([]access.NonFungibleTokenTransfer, 0, len(paired))
	for i, pair := range paired {
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

		if len(eventIndices) == 0 {
			return nil, fmt.Errorf("no event indices for source events (tx=%s, pairIdx=%d)", txID, i)
		}

		transfer := access.NonFungibleTokenTransfer{
			BlockHeight:      blockHeight,
			TransactionID:    txID,
			TransactionIndex: txIndex,
			EventIndices:     eventIndices,
		}

		if pair.withdrawal != nil {
			transfer.TokenType = pair.withdrawal.Type
			transfer.SourceAddress = pair.withdrawal.From
			transfer.ID = pair.withdrawal.ID
		}

		if pair.deposit != nil {
			transfer.TokenType = pair.deposit.Type
			transfer.ID = pair.deposit.ID
			transfer.RecipientAddress = pair.deposit.To
		} else {
			transfer.RecipientAddress = pair.recipientAddress
		}

		if transfer.TokenType == "" {
			return nil, fmt.Errorf("token type is empty for NFT transfer (tx=%s, evtIdx=%s)",
				txID, strings.Join(eventIndicesStr, ","))
		}

		transfers = append(transfers, transfer)
	}
	// sort ascending. for transfers in the same transaction, sort ascending by the last event index,
	// which is either the deposit event, or the withdrawal placeholder event.
	sort.Slice(transfers, func(i, j int) bool {
		if transfers[i].TransactionIndex == transfers[j].TransactionIndex {
			return transfers[i].EventIndices[len(transfers[i].EventIndices)-1] < transfers[j].EventIndices[len(transfers[j].EventIndices)-1]
		}
		return transfers[i].TransactionIndex < transfers[j].TransactionIndex
	})
	return transfers, nil
}
