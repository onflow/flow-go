package collection

import (
	"math"

	"github.com/onflow/flow-go/model/flow"
)

// rateLimiter implements payer-based rate limiting. See Config for details.
type rateLimiter struct {

	// maximum rate of transactions/payer/collection (from Config)
	rate float64
	// Derived fields based on the rate. Only one will be used:
	//  - if rate >= 1, then txPerBlock is set to the number of transactions allowed per payer per block
	//  - if rate < 1, then blocksPerTx is set to the number of consecutive blocks in which one transaction per payer is allowed
	txPerBlock  uint
	blocksPerTx uint64

	// set of unlimited payer address (from Config)
	unlimited map[flow.Address]struct{}
	// height of the collection we are building
	height uint64

	// for each payer, height of latest collection in which a transaction for
	// which they were payer was included
	latestCollectionHeight map[flow.Address]uint64
	// number of transactions included in the currently built block per payer
	txIncludedCount map[flow.Address]uint
}

func newRateLimiter(conf Config, height uint64) *rateLimiter {
	limiter := &rateLimiter{
		rate:                   conf.MaxPayerTransactionRate,
		unlimited:              conf.UnlimitedPayers,
		height:                 height,
		latestCollectionHeight: make(map[flow.Address]uint64),
		txIncludedCount:        make(map[flow.Address]uint),
	}
	if limiter.rate >= 1 {
		limiter.txPerBlock = uint(math.Floor(limiter.rate))
	} else {
		limiter.blocksPerTx = uint64(math.Ceil(1 / limiter.rate))
	}
	return limiter
}

// note the existence and height of a transaction in an ancestor collection.
func (limiter *rateLimiter) addAncestor(height uint64, tx *flow.TransactionBody) {

	// skip tracking payers if we aren't rate-limiting or are configured
	// to allow multiple transactions per payer per collection
	if limiter.rate >= 1 || limiter.rate <= 0 {
		return
	}

	latest := limiter.latestCollectionHeight[tx.Payer]
	if height >= latest {
		limiter.latestCollectionHeight[tx.Payer] = height
	}
}

// note that we have added a transaction to the collection under construction.
func (limiter *rateLimiter) transactionIncluded(tx *flow.TransactionBody) {
	limiter.txIncludedCount[tx.Payer]++
}

// applies the rate limiting rules, returning whether the transaction should be
// omitted from the collection under construction.
func (limiter *rateLimiter) shouldRateLimit(tx *flow.TransactionBody) bool {

	payer := tx.Payer

	// skip rate limiting if it is turned off or the payer is unlimited
	_, isUnlimited := limiter.unlimited[payer]
	if limiter.rate <= 0 || isUnlimited {
		return false
	}

	// if rate >=1, we only consider the current collection and rate limit once
	// the number of transactions for the payer exceeds rate
	if limiter.rate >= 1 {
		if limiter.txIncludedCount[payer] >= limiter.txPerBlock {
			return true
		}
	}

	// if rate < 1, we need to look back to see when a transaction by this payer
	// was most recently included - we rate limit if the # of collections since
	// the payer's last transaction is less than ceil(1/rate)
	if limiter.rate < 1 {

		// rate limit if we've already include a transaction for this payer, we allow
		// AT MOST one transaction per payer in a given collection
		if limiter.txIncludedCount[payer] > 0 {
			return true
		}

		// otherwise, check whether sufficiently many empty collection
		// have been built since the last transaction from the payer

		latestHeight, hasLatest := limiter.latestCollectionHeight[payer]
		// if there is no recent transaction, don't rate limit
		if !hasLatest {
			return false
		}

		if limiter.height-latestHeight < limiter.blocksPerTx {
			return true
		}
	}

	return false
}
