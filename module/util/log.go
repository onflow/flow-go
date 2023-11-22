package util

import (
	"github.com/rs/zerolog"
)

// LogProgress takes a total and return function such that when called with a 0-based index
// it prints the progress from 0% to 100% to indicate the index from 0 to (total - 1) has been
// processed.
// useful to report the progress of processing the index from 0 to (total - 1)
func LogProgress(msg string, total int, logger zerolog.Logger) func(currentIndex int) {
	logThreshold := float64(0)
	return func(currentIndex int) {
		percentage := float64(100)
		if total > 0 {
			percentage = (float64(currentIndex+1) / float64(total)) * 100. // currentIndex+1 assuming zero based indexing
		}

		// report every 10 percent
		if percentage >= logThreshold {
			logger.Info().Msgf("%s progress: %v percent", msg, logThreshold)
			logThreshold += 10
		}
	}
}
