package checkpoint_collect_stats

import (
	"cmp"
	"slices"

	statslib "github.com/montanaflynn/stats"
	"github.com/rs/zerolog/log"
)

type percentileValue struct {
	Percentile float64 `json:"percentile"`
	Value      float64 `json:"value"`
}

type stats struct {
	Count       uint64  `json:"count"`
	Sum         float64 `json:"sum"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Percentiles []percentileValue
}

func getValueStats(values []float64, percentiles []float64) stats {
	if len(values) == 0 {
		return stats{}
	}

	describe, err := statslib.Describe(values, true, &percentiles)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot describe values")
	}

	sum, err := statslib.Sum(values)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot compute sum of values")
	}

	percentileValues := make([]percentileValue, len(describe.DescriptionPercentiles))
	for i, pv := range describe.DescriptionPercentiles {
		percentileValues[i] = percentileValue{
			Percentile: pv.Percentile,
			Value:      pv.Value,
		}
	}

	slices.SortFunc(percentileValues, func(a, b percentileValue) int {
		return cmp.Compare(a.Percentile, b.Percentile)
	})

	return stats{
		Count:       uint64(len(values)),
		Sum:         sum,
		Min:         describe.Min,
		Max:         describe.Max,
		Percentiles: percentileValues,
	}
}
