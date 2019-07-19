// Package processor is in charge of the ExecutionReceipt processing flow.
// It decides whether an receipt gets discarded/slashed/approved/cached, while relying on external side effects functions to trigger these actions.
// The package holds a queue of receipts and processes them in FIFO to utilise caching.
// Note a sun currency optimisation is possible by having a queue-per-block-height without losing on any caching potential.
package processor

import (
	"fmt"

	"github.com/bluele/gcache"

	// "github.com/dapperlabs/bamboo-node/internal/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/config"
)

// ReceiptProcessorConfig holds the configuration for receipt processor.
type ReceiptProcessorConfig struct {
	QueueBuffer int
	CacheBuffer int
}

// todo: use config values
func NewReceiptProcessorConfig(c *config.Config) *ReceiptProcessorConfig {
	return &ReceiptProcessorConfig{
		QueueBuffer: 3,
		CacheBuffer: 4,
	}
}

type receiptProcessor struct {
	q       chan *receiptAndDoneChan
	effects processorEffects
	cache   gcache.Cache
}

type receiptAndDoneChan struct {
	receipt *types.ExecutionReceipt
	done    chan bool
}

// NewReceiptProcessor returns a new processor instance.
// A go routine is initialised and waiting to process new items.
func NewReceiptProcessor(effects processorEffects, rc *ReceiptProcessorConfig) *receiptProcessor {
	p := &receiptProcessor{
		q:       make(chan *receiptAndDoneChan, rc.QueueBuffer),
		effects: effects,
		cache:   gcache.New(rc.CacheBuffer).LRU().Build(),
	}

	go p.run()
	return p
}

// Submit takes in an ExecutionReceipt to be process async.
// The done chan is optional. If caller is not interested to be notified when processing has been completed, nil should be passed.
func (p *receiptProcessor) Submit(receipt *types.ExecutionReceipt, done chan bool) {
	// todo: if not a valid signature, then discard

	if ok, err := p.effects.hasMinStake(receipt); err != nil {
		p.effects.handleError(err)
	} else if !ok {
		p.effects.handleError(fmt.Errorf("receipt does not have minimum stake: %v", receipt))
		return
	}

	rdc := &receiptAndDoneChan{
		receipt: receipt,
		done:    done,
	}
	p.q <- rdc
}

func (p *receiptProcessor) run() {
	for {
		rdc := <-p.q
		receipt := rdc.receipt
		done := rdc.done

		// receiptHash := crypto.NewHash(receipt)
		receiptHash := "TODO"

		// If cached result exists (err == nil), reuse it
		if v, err := p.cache.Get(receiptHash); err == nil {
			validationResult := v.(ValidationResult)
			p.sendApprovalOrSlash(receipt, validationResult)
			notifyDone(done)
			return
		}

		// Else, err!=nil, meaning not in cache, continue processing.
		// If block is already sealed with different receipt, slash it
		// TODO: discuss the feasibility of slashing request without proof?
		if shouldSlash, err := p.effects.isSealedWithDifferentReceipt(receipt); err != nil {
			p.effects.handleError(err)
			notifyDone(done)
			return
		} else if shouldSlash {
			p.effects.slashExpiredReceipt(receipt)
			notifyDone(done)
			return
		}

		// Validate receipt (chunk assignment logic is encapsulated away).
		validationResult, err := p.effects.isValidExecutionReceipt(receipt)
		if err != nil {
			p.effects.handleError(err)
			notifyDone(done)
			return
		}
		p.sendApprovalOrSlash(receipt, validationResult)

		// Cache the result.
		if err := p.cache.Set(receiptHash, validationResult); err != nil {
			p.effects.handleError(err)
		}
		notifyDone(done)
	}
}

// dd success
func (p *receiptProcessor) sendApprovalOrSlash(receipt *types.ExecutionReceipt, validationResult ValidationResult) {
	switch vr := validationResult.(type) {
	case *ValidationResultSuccess:
		p.effects.send(receipt, vr.proof)
	case *ValidationResultFail:
		p.effects.slashInvalidReceipt(receipt, vr.blockPartResult)
	default:
		panic(fmt.Sprintf("unreachable code with unexpected type %T", vr))
	}
}

func notifyDone(c chan bool) {
	if c != nil {
		c <- true
		close(c)
	}
}
