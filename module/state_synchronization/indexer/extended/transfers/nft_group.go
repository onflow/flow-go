package transfers

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
)

// nftDecodedWithdrawal wraps a decoded NFT withdrawal event with its source [flow.Event] metadata.
type nftDecodedWithdrawal struct {
	source         flow.Event
	decoded        events.NFTWithdrawnEvent
	ancestorEvents []flow.Event // events from parent withdrawals in the ProviderUUID chain
}

// nftDecodedDeposit wraps a decoded NFT deposit event with its source [flow.Event] metadata.
type nftDecodedDeposit struct {
	source  flow.Event
	decoded events.NFTDepositedEvent
}

// nftWithdrawalRun groups consecutive Withdrawn events with the same source address.
// Consecutive same-address withdrawals for a single NFT represent multiple collection layers
// owned by the same account.
type nftWithdrawalRun struct {
	address flow.Address
	events  []*nftDecodedWithdrawal
}

// nftTxEventGroup holds decoded withdrawal and deposit events for a single transaction.
//
// pendingWithdrawals maps an NFT UUID to the ordered list of withdrawals not yet paired with a
// deposit. After a deposit is matched the entry is removed, so the same UUID may be withdrawn
// again within the same transaction (multi-hop transfers: A → B → C).
type nftTxEventGroup struct {
	pendingWithdrawals map[uint64][]*nftDecodedWithdrawal
	pairedResults      []nftPairedResult
}

func newNFTTxEventGroup() *nftTxEventGroup {
	return &nftTxEventGroup{
		pendingWithdrawals: make(map[uint64][]*nftDecodedWithdrawal),
	}
}

// addWithdrawal adds a withdrawal event to the event group.
//
// No error returns are expected during normal operation.
func (g *nftTxEventGroup) addWithdrawal(event flow.Event, decoded *events.NFTWithdrawnEvent) error {
	w := &nftDecodedWithdrawal{source: event, decoded: *decoded}

	existing := g.pendingWithdrawals[decoded.UUID]
	if len(existing) == 0 {
		// First withdrawal for this UUID. Check if the NFT's provider collection was itself
		// withdrawn in this transaction and propagate the source address if missing.
		parentPending := g.pendingWithdrawals[w.decoded.ProviderUUID]
		if len(parentPending) > 0 {
			parent := parentPending[len(parentPending)-1]

			// Build event ancestor chain: parent's ancestors + parent itself.
			chain := make([]flow.Event, len(parent.ancestorEvents)+1)
			copy(chain, parent.ancestorEvents)
			chain[len(chain)-1] = parent.source
			w.ancestorEvents = chain

			// Propagate source address from parent if this withdrawal has none.
			if w.decoded.From == flow.EmptyAddress && parent.decoded.From != flow.EmptyAddress {
				w.decoded.From = parent.decoded.From
			}
		}
	}

	g.pendingWithdrawals[decoded.UUID] = append(existing, w)
	return nil
}

// addDeposit adds a deposit event to the event group.
//
// No error returns are expected during normal operation.
func (g *nftTxEventGroup) addDeposit(event flow.Event, decoded *events.NFTDepositedEvent) error {
	d := &nftDecodedDeposit{source: event, decoded: *decoded}
	uuid := decoded.UUID

	pending := g.pendingWithdrawals[uuid]
	if len(pending) == 0 {
		// No corresponding withdrawal - treat as mint.
		g.pairedResults = append(g.pairedResults, resolveNFTTransfers(nil, d)...)
		return nil
	}

	// Partition pending withdrawals by NFT ID. Some collections (e.g. TopShot) reuse Cadence
	// resource UUIDs across distinct NFTs, so multiple NFTs with different IDs can share a UUID.
	// Only withdrawals with a matching ID belong to this deposit; the rest stay pending.
	var matching []*nftDecodedWithdrawal
	var remaining []*nftDecodedWithdrawal
	for _, w := range pending {
		if w.decoded.ID == decoded.ID {
			matching = append(matching, w)
		} else {
			remaining = append(remaining, w)
		}
	}

	if len(matching) == 0 {
		// No withdrawal with a matching NFT ID - treat as mint.
		g.pairedResults = append(g.pairedResults, resolveNFTTransfers(nil, d)...)
		return nil
	}

	// Validate token type against the matched withdrawals.
	lastW := matching[len(matching)-1]
	if lastW.decoded.Type != decoded.Type {
		return fmt.Errorf("withdrawal token type %s (eventIdx=%d) is not equal to the deposit token type %s (eventIdx=%d) in transaction %d",
			lastW.decoded.Type, lastW.source.EventIndex, decoded.Type, event.EventIndex, event.TransactionIndex)
	}

	g.pairedResults = append(g.pairedResults, resolveNFTTransfers(matching, d)...)

	// Keep only non-matching withdrawals pending. If none remain, clean up the map entry
	// so the same UUID can be withdrawn again within this transaction (multi-hop: A → B → C).
	if len(remaining) == 0 {
		delete(g.pendingWithdrawals, uuid)
	} else {
		g.pendingWithdrawals[uuid] = remaining
	}
	return nil
}

// ResolvePairs returns all paired results including unmatched withdrawals (burns).
func (g *nftTxEventGroup) ResolvePairs() []nftPairedResult {
	for _, pending := range g.pendingWithdrawals {
		g.pairedResults = append(g.pairedResults, resolveNFTTransfers(pending, nil)...)
	}
	return g.pairedResults
}

// resolveNFTTransfers converts a sequence of pending Withdrawn events (and an optional Deposited
// event) into one or more [nftPairedResult] objects.
//
// Consecutive withdrawals with the same source address are grouped into runs. Each run represents
// one or more collection layers owned by the same account. A change in source address between
// consecutive withdrawals indicates an ownership boundary and produces an intermediate transfer.
//
// Given runs [G0, G1, ..., GN] (G0 is innermost / earliest events, GN is outermost):
//   - One final transfer: G0.address → deposit.To (or burn if deposit is nil),
//     with all G0 events plus the deposit as source events.
//   - One intermediate transfer per adjacent run pair (Gi → Gi-1 for i = 1..N):
//     Gi.address → Gi-1.address, with [last event of Gi-1, first event of Gi] as source events.
func resolveNFTTransfers(pending []*nftDecodedWithdrawal, deposit *nftDecodedDeposit) []nftPairedResult {
	if len(pending) == 0 {
		if deposit == nil {
			return nil
		}
		return []nftPairedResult{{
			sourceEvents: []flow.Event{deposit.source},
			deposit:      &deposit.decoded,
		}}
	}

	runs := groupNFTIntoRuns(pending)

	var results []nftPairedResult

	// Final transfer: innermost run → deposit (or burn if deposit is nil).
	innerRun := runs[0]
	var finalSourceEvents []flow.Event
	for _, w := range innerRun.events {
		finalSourceEvents = append(finalSourceEvents, w.ancestorEvents...)
		finalSourceEvents = append(finalSourceEvents, w.source)
	}
	if deposit != nil {
		finalSourceEvents = append(finalSourceEvents, deposit.source)
		results = append(results, nftPairedResult{
			sourceEvents: finalSourceEvents,
			withdrawal:   &innerRun.events[0].decoded,
			deposit:      &deposit.decoded,
		})
	} else {
		results = append(results, nftPairedResult{
			sourceEvents: finalSourceEvents,
			withdrawal:   &innerRun.events[0].decoded,
			// recipientAddress is zero = burn
		})
	}

	// Intermediate transfers: runs[i] → runs[i-1] for each adjacent run pair.
	for i := 1; i < len(runs); i++ {
		outerRun := runs[i]
		prevInnerRun := runs[i-1]

		lastInner := prevInnerRun.events[len(prevInnerRun.events)-1]
		firstOuter := outerRun.events[0]

		var sourceEvents []flow.Event
		sourceEvents = append(sourceEvents, lastInner.ancestorEvents...)
		sourceEvents = append(sourceEvents, lastInner.source)
		sourceEvents = append(sourceEvents, firstOuter.ancestorEvents...)
		sourceEvents = append(sourceEvents, firstOuter.source)

		results = append(results, nftPairedResult{
			sourceEvents:     sourceEvents,
			withdrawal:       &firstOuter.decoded,
			recipientAddress: prevInnerRun.address,
		})
	}

	return results
}

// groupNFTIntoRuns groups a sequence of withdrawals into runs of consecutive same-address events.
func groupNFTIntoRuns(pending []*nftDecodedWithdrawal) []nftWithdrawalRun {
	var runs []nftWithdrawalRun
	for _, w := range pending {
		if len(runs) == 0 || runs[len(runs)-1].address != w.decoded.From {
			runs = append(runs, nftWithdrawalRun{address: w.decoded.From})
		}
		runs[len(runs)-1].events = append(runs[len(runs)-1].events, w)
	}
	return runs
}
