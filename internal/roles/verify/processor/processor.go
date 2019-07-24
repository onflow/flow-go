// Package processor is in charge of the ExecutionReceipt processing flow.
// It decides whether a receipt gets discarded/slashed/approved/cached, while relying on external side effects functions to trigger these actions (template pattern).
// The package holds a queue of receipts and processes them in FIFO to utilise caching result and not re-validate a validated receipt submitted by another node.
// Note that some concurrency optimisation is possible by having a queue-per-block-height without sacrificing any caching potential.
package processor

import (
	"fmt"

	"github.com/bluele/gcache"

	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type ReceiptProcessor struct {
	q       chan *receiptAndDoneChan
	effects Effects
	cache   gcache.Cache
	hasher  crypto.Hasher
}

type receiptAndDoneChan struct {
	receipt *types.ExecutionReceipt
	done    chan bool
}

// NewReceiptProcessor returns a new processor instance.
// A go routine is initialised and waiting to process new items.
func NewReceiptProcessor(effects Effects, rc *ReceiptProcessorConfig, hasher crypto.Hasher) *ReceiptProcessor {
	p := &ReceiptProcessor{
		q:       make(chan *receiptAndDoneChan, rc.QueueBuffer),
		effects: effects,
		cache:   gcache.New(rc.CacheBuffer).LRU().Build(),
		hasher:  hasher,
	}

	go p.run()
	return p
}

// Submit takes in an ExecutionReceipt to be process async.
// The done chan is optional. If caller is not interested to be notified when processing has been completed, nil value should be used for it.
func (p *ReceiptProcessor) Submit(receipt *types.ExecutionReceipt, done chan bool) {
	// TODO: if ER does not have a valid signature, then this needs to be discard. Deal with it here are at upper layer before submit?

	if ok, err := p.effects.HasMinStake(receipt); err != nil {
		p.effects.HandleError(err)
		notifyDone(done)
		return
	} else if !ok {
		p.effects.HandleError(fmt.Errorf("receipt does not have minimum stake: %v", receipt))
		notifyDone(done)
		return
	}

	rdc := &receiptAndDoneChan{
		receipt: receipt,
		done:    done,
	}
	p.q <- rdc
}

func (p *ReceiptProcessor) run() {
	for {
		rdc := <-p.q
		receipt := rdc.receipt
		done := rdc.done

		receiptHash := p.hasher.ComputeStructHash(receipt)

		// If cached result exists (err == nil), reuse it
		if v, err := p.cache.Get(receiptHash); err == nil {
			validationResult := v.(compute.ValidationResult)
			p.sendApprovalOrSlash(receipt, validationResult)
			notifyDone(done)
			return
		}

		// Else, err!=nil, meaning not in cache, continue processing.
		// If block is already sealed with different receipt, slash it
		// TODO: discuss the feasibility of slashing request without proof?
		if shouldSlash, err := p.effects.IsSealedWithDifferentReceipt(receipt); err != nil {
			p.effects.HandleError(err)
			notifyDone(done)
			return
		} else if shouldSlash {
			p.effects.SlashExpiredReceipt(receipt)
			notifyDone(done)
			return
		}

		// Validate receipt (chunk assignment logic is encapsulated away).
		validationResult, err := p.effects.IsValidExecutionReceipt(receipt)
		if err != nil {
			p.effects.HandleError(err)
			notifyDone(done)
			return
		}
		p.sendApprovalOrSlash(receipt, validationResult)

		// Cache the result.
		if err := p.cache.Set(receiptHash, validationResult); err != nil {
			p.effects.HandleError(err)
		}
		notifyDone(done)
	}
}

func (p *ReceiptProcessor) sendApprovalOrSlash(receipt *types.ExecutionReceipt, validationResult compute.ValidationResult) {
	switch vr := validationResult.(type) {
	case *compute.ValidationResultSuccess:
		p.effects.Send(receipt, vr.Proof)
	case *compute.ValidationResultFail:
		p.effects.SlashInvalidReceipt(receipt, vr.BlockPartResult)
	default:
		panic(fmt.Sprintf("unreachable code with unexpected type %T", vr))
	}
}

// ReceiptProcessorConfig holds the configuration for receipt processor.
type ReceiptProcessorConfig struct {
	QueueBuffer int
	CacheBuffer int
}

func notifyDone(c chan bool) {
	if c == nil {
		return
	}

	c <- true
	close(c)
}
