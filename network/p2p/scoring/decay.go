package scoring

import "time"

func DecayScore(score float64, decay float64, lastUpdated time.Time) float64 {
	return score * decay * time.Since(lastUpdated).Seconds()
}
