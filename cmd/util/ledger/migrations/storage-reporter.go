package migrations

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"math"
	"strings"
)

// iterates through registers keeping a map of register sizes
// reports on storage metrics
type StorageReporter struct {
	Log zerolog.Logger
}

func (r StorageReporter) Report(payload []ledger.Payload) error {
	r.Log.Info().Msg("Running Storage Reporter")
	storageUsed := make(map[string]uint64)
	average := 0.0
	max := uint64(0)

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
		if u > max {
			max = u
		}
	}
	r.Log.Info().
		Msgf("Average storage used %g", average)
	r.Log.Info().
		Msgf("Max storage used %d", max)

	bins := 150
	binHeight := 40
	w := float64(max) / float64(bins)
	distribution := make([]float64, bins)

	var maxBin = float64(0)

	for s, u := range storageUsed {
		var i = int(float64(u) / w)
		if i >= bins {
			i = bins - 1
		}
		distribution[i] = distribution[i] + 1

		if maxBin < distribution[i] {
			maxBin = distribution[i]
		}
		if i == bins-1 {
			r.Log.Info().
				Msgf("High storage usage address: %s, used: %d", flow.BytesToAddress([]byte(s)), u)
		}
	}
    maxBinAccounts := maxBin
	maxBin = math.Log10(maxBin)

	graph := make([]int, bins)

	for i, d := range distribution {
		var v float64
		if d <= 0 {
			v = 0
		} else {
			v = math.Log10(d)
		}

		graph[i] = int(math.Ceil((v / maxBin) * float64(binHeight)))
	}

	r.Log.Info().
		Msgf("Logarithmic account storage distribution x[storage used] / y[log number of accounts (max = %d accounts)]:\n", maxBinAccounts)

	var sb strings.Builder
	for j := binHeight; j >= 0; j-- {
		for i := 0; i < bins; i++ {
			if graph[i] >= j {
				sb.WriteRune('#')
			} else {
				sb.WriteRune('.')
			}
		}
		sb.WriteRune('\n')
	}

	r.Log.Info().
		Msg(sb.String())

	return nil
}
