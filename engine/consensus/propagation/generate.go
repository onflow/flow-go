// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"math/rand"
	"time"

	"github.com/dapperlabs/flow-go/model/collection"
)

// Generate will generate random fingerprints and inject them into the
// engine.
func Generate(e *Engine, interval time.Duration) {

	// we randomize interval from 50% to 150%
	min := uint64(interval) * 5 / 10
	max := uint64(interval) * 15 / 10

	// first sleep is between zero and possible range between min and max
	dur := (rand.Uint64() % (max - min))
	time.Sleep(time.Duration(dur))

	for {

		// create random hash for fingerprint
		hash := make([]byte, 32)
		_, _ = rand.Read(hash)
		fp := &collection.GuaranteedCollection{
			Hash: hash,
		}

		// submit to engine
		err := e.SubmitGuaranteedCollection(fp)
		if err != nil {
			e.log.Error().
				Err(err).
				Hex("collection_hash", hash).
				Msg("could not submit fingerprint")
			continue
		}

		e.log.Info().
			Hex("collection_hash", hash).
			Msg("generated fingerprint submitted")

		// subsequent sleeps are from min to max
		dur = (rand.Uint64() % (max - min)) + min
		time.Sleep(time.Duration(dur))
	}
}
