package migrations

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

// iterates through registers keeping a map of register sizes
// reports on storage metrics
type StorageReporter struct {
	Log zerolog.Logger
}

func (r StorageReporter) Report(payload []ledger.Payload) error {
	r.Log.Info().Msg("Running Storage Reporter")
	storageUsed := make(map[string]uint64)
	var average = 0.0
	// assuming storage used is an exponential distribution.
	// only 1% of accounts will be use then exponentialPercentile99 times average storage
	var exponentialPercentile99 = 4.605170185988091368036

	for i, p := range payload {
		id, err := keyToRegisterId(p.Key)
		if err != nil {
			return err
		}
		if len([]byte(id.Owner)) != flow.AddressLength {
			// not an address
			continue
		}
		if id.Key != "storage_used" {
			continue
		}
		u, _, err := utils.ReadUint64(p.Value)
		if err != nil {
			return err
		}
		storageUsed[id.Owner] = u
		average = average + (float64(u)-average)/(float64(i)+1.0)
	}
	r.Log.Info().
		Msgf("Average storage used %g", average)
	r.Log.Info().
		Msg("99th percentile accounts (assuming exponential distribution):")

	for s, u := range storageUsed {
		if float64(u) > exponentialPercentile99*average {
			r.Log.Info().
				Msgf("Address: %s Used: %d", s, u)
		}
	}
	return nil
}
