package transfers

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ftDecodedWithdrawal wraps a decoded FT withdrawal event with its source [flow.Event] metadata.
type ftDecodedWithdrawal struct {
	source         flow.Event
	decoded        ftWithdrawnEvent
	ancestorEvents []flow.Event // events from removed parent withdrawals in the event chain
}

// ftDecodedDeposit wraps a decoded FT deposit event with its source [flow.Event] metadata.
type ftDecodedDeposit struct {
	source  flow.Event
	decoded ftDepositedEvent
}

// ftTxEventGroup holds decoded withdrawal and deposit events for a single transaction.
type ftTxEventGroup struct {
	withdrawals []*ftDecodedWithdrawal
	deposits    []*ftDecodedDeposit

	withdrawalByUUID map[uint64]int
	matchedDeposits  map[uint64]struct{}

	pairedResults []ftPairedResult
}

func newFTTxEventGroup() *ftTxEventGroup {
	return &ftTxEventGroup{
		withdrawalByUUID: make(map[uint64]int),
		matchedDeposits:  make(map[uint64]struct{}),
	}
}

// addWithdrawal adds a withdrawal event to the event group.
//
// No error returns are expected during normal operation.
func (g *ftTxEventGroup) addWithdrawal(event flow.Event, decoded *ftWithdrawnEvent) error {
	w := &ftDecodedWithdrawal{source: event, decoded: *decoded}
	g.withdrawals = append(g.withdrawals, w)

	// 1. build a mapping of withdrawn vault UUID to the withdrawal index in the `withdrawals` slice.
	// this is used to identify the parent when there is a chain of withdrawals.
	if _, exists := g.withdrawalByUUID[decoded.WithdrawnUUID]; exists {
		return fmt.Errorf("duplicate withdrawal resource UUID %d in transaction %d", decoded.WithdrawnUUID, event.TransactionIndex)
	}
	g.withdrawalByUUID[decoded.WithdrawnUUID] = len(g.withdrawals) - 1

	// 2. check if withdrawal is from a stored vault or another withdrawn vault
	parentIdx, ok := g.withdrawalByUUID[w.decoded.FromUUID]
	if !ok {
		return nil // withdrew from stored vault (or mint)
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

	// 5. Subtract child withdrawal amount from parent. this ensures the final amounts used in
	// the deposit/withdraw pairing are correct.
	if w.decoded.Amount <= parent.decoded.Amount {
		parent.decoded.Amount -= w.decoded.Amount
	} else {
		return fmt.Errorf("child withdrawal amount %s (eventIdx=%d) is greater than the remaining parent withdrawal amount %s (eventIdx=%d) in transaction %d",
			w.decoded.Amount.String(), w.source.EventIndex, parent.decoded.Amount.String(), parent.source.EventIndex, event.TransactionIndex)
	}

	return nil
}

// addDeposit adds a deposit event to the event group.
//
// No error returns are expected during normal operation.
func (g *ftTxEventGroup) addDeposit(event flow.Event, decoded *ftDepositedEvent) error {
	d := &ftDecodedDeposit{source: event, decoded: *decoded}
	g.deposits = append(g.deposits, d)

	uuid := decoded.DepositedUUID

	// 1. check if the deposit is a vault withdrawn in this transaction.
	wIdx, ok := g.withdrawalByUUID[uuid]
	if !ok {
		// No corresponding withdrawal - treat as mint.
		g.pairedResults = append(g.pairedResults, ftPairedResult{
			sourceEvents: mergeEvents(nil, d),
			deposit:      &d.decoded,
		})
		return nil
	}
	w := g.withdrawals[wIdx]

	// 2. make sure the deposit and withdrawal match.
	// if not, then there is likely a bug in the event parsing logic.
	if w.decoded.Amount != d.decoded.Amount {
		return fmt.Errorf("withdrawal amount %s (eventIdx=%d) is not equal to the deposit amount %s (eventIdx=%d) in transaction %d",
			d.decoded.Amount.String(), d.source.EventIndex, w.decoded.Amount.String(), w.source.EventIndex, event.TransactionIndex)
	}

	if w.decoded.Type != d.decoded.Type {
		return fmt.Errorf("withdrawal token type %s (eventIdx=%d) is not equal to the deposit token type %s (eventIdx=%d) in transaction %d",
			w.decoded.Type, w.source.EventIndex, d.decoded.Type, d.source.EventIndex, event.TransactionIndex)
	}

	// 3. pair the deposit with the withdrawal.
	g.matchedDeposits[uuid] = struct{}{}
	g.pairedResults = append(g.pairedResults, ftPairedResult{
		sourceEvents: mergeEvents(w, d),
		withdrawal:   &w.decoded,
		deposit:      &d.decoded,
	})
	return nil
}

func (g *ftTxEventGroup) ResolvePairs() []ftPairedResult {
	// find all unmatched withdrawals
	for uuid, wIdx := range g.withdrawalByUUID {
		if _, ok := g.matchedDeposits[uuid]; ok {
			continue
		}
		// Unmatched withdrawal -- treat as burn.
		g.pairedResults = append(g.pairedResults, ftPairedResult{
			sourceEvents: mergeEvents(g.withdrawals[wIdx], nil),
			withdrawal:   &g.withdrawals[wIdx].decoded,
		})
	}
	return g.pairedResults
}

func mergeEvents(w *ftDecodedWithdrawal, d *ftDecodedDeposit) []flow.Event {
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
