package transfers

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// nftDecodedWithdrawal wraps a decoded NFT withdrawal event with its source [flow.Event] metadata.
type nftDecodedWithdrawal struct {
	source         flow.Event
	decoded        nftWithdrawnEvent
	ancestorEvents []flow.Event // events from removed parent withdrawals in the event chain
}

// nftDecodedDeposit wraps a decoded NFT deposit event with its source [flow.Event] metadata.
type nftDecodedDeposit struct {
	source  flow.Event
	decoded nftDepositedEvent
}

// nftTxEventGroup holds decoded withdrawal and deposit events for a single transaction.
type nftTxEventGroup struct {
	withdrawals []*nftDecodedWithdrawal
	deposits    []*nftDecodedDeposit

	withdrawalByUUID map[uint64]int
	matchedDeposits  map[uint64]struct{}

	pairedResults []nftPairedResult
}

func newNFTTxEventGroup() *nftTxEventGroup {
	return &nftTxEventGroup{
		withdrawalByUUID: make(map[uint64]int),
		matchedDeposits:  make(map[uint64]struct{}),
	}
}

// addWithdrawal adds a withdrawal event to the event group.
//
// No error returns are expected during normal operation.
func (g *nftTxEventGroup) addWithdrawal(event flow.Event, decoded *nftWithdrawnEvent) error {
	w := &nftDecodedWithdrawal{source: event, decoded: *decoded}
	g.withdrawals = append(g.withdrawals, w)

	// 1. build a mapping of withdrawn NFT UUID to the withdrawal index in the `withdrawals` slice.
	// this is used to identify the parent when there is a chain of withdrawals.
	if _, exists := g.withdrawalByUUID[decoded.UUID]; exists {
		return fmt.Errorf("duplicate withdrawal resource UUID %d in transaction %d", decoded.UUID, event.TransactionIndex)
	}
	g.withdrawalByUUID[decoded.UUID] = len(g.withdrawals) - 1

	// 2. check if withdrawal is from a stored collection or another withdrawn collection
	parentIdx, ok := g.withdrawalByUUID[w.decoded.ProviderUUID]
	if !ok {
		return nil // withdrew from stored collection (or mint)
	}
	parent := g.withdrawals[parentIdx]

	// 3. build event ancestor chain: parent's ancestors + parent itself.
	chain := make([]flow.Event, len(parent.ancestorEvents)+1)
	copy(chain, parent.ancestorEvents)
	chain[len(chain)-1] = parent.source
	w.ancestorEvents = chain

	// 4. propagate the source address from parent to track sender
	if w.decoded.From == flow.EmptyAddress && parent.decoded.From != flow.EmptyAddress {
		w.decoded.From = parent.decoded.From
	}

	return nil
}

// addDeposit adds a deposit event to the event group.
//
// No error returns are expected during normal operation.
func (g *nftTxEventGroup) addDeposit(event flow.Event, decoded *nftDepositedEvent) error {
	d := &nftDecodedDeposit{source: event, decoded: *decoded}
	g.deposits = append(g.deposits, d)

	uuid := decoded.UUID

	// 1. check if the deposit is an NFT withdrawn in this transaction.
	wIdx, ok := g.withdrawalByUUID[uuid]
	if !ok {
		// No corresponding withdrawal - treat as mint.
		g.pairedResults = append(g.pairedResults, nftPairedResult{
			sourceEvents: nftMergeEvents(nil, d),
			deposit:      &d.decoded,
		})
		return nil
	}
	w := g.withdrawals[wIdx]

	// 2. make sure the deposit and withdrawal match.
	// if not, then there is likely a bug in the event parsing logic.
	if w.decoded.Type != d.decoded.Type {
		return fmt.Errorf("withdrawal token type %s (eventIdx=%d) is not equal to the deposit token type %s (eventIdx=%d) in transaction %d",
			w.decoded.Type, w.source.EventIndex, d.decoded.Type, d.source.EventIndex, event.TransactionIndex)
	}

	if w.decoded.ID != d.decoded.ID {
		return fmt.Errorf("withdrawal NFT ID %d (eventIdx=%d) is not equal to the deposit NFT ID %d (eventIdx=%d) in transaction %d",
			w.decoded.ID, w.source.EventIndex, d.decoded.ID, d.source.EventIndex, event.TransactionIndex)
	}

	// 3. pair the deposit with the withdrawal.
	g.matchedDeposits[uuid] = struct{}{}
	g.pairedResults = append(g.pairedResults, nftPairedResult{
		sourceEvents: nftMergeEvents(w, d),
		withdrawal:   &w.decoded,
		deposit:      &d.decoded,
	})
	return nil
}

// ResolvePairs returns all paired results including unmatched withdrawals (burns).
func (g *nftTxEventGroup) ResolvePairs() []nftPairedResult {
	// find all unmatched withdrawals
	for uuid, wIdx := range g.withdrawalByUUID {
		if _, ok := g.matchedDeposits[uuid]; ok {
			continue
		}
		// Unmatched withdrawal -- treat as burn.
		g.pairedResults = append(g.pairedResults, nftPairedResult{
			sourceEvents: nftMergeEvents(g.withdrawals[wIdx], nil),
			withdrawal:   &g.withdrawals[wIdx].decoded,
		})
	}
	return g.pairedResults
}

func nftMergeEvents(w *nftDecodedWithdrawal, d *nftDecodedDeposit) []flow.Event {
	var events []flow.Event
	if w != nil {
		events = append(events, w.ancestorEvents...)
		events = append(events, w.source)
	}
	if d != nil {
		events = append(events, d.source)
	}
	return events
}
