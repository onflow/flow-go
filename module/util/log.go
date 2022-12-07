package util

import (
	"github.com/rs/zerolog"
)

func LogProgress(msg string, total int, logger *zerolog.Logger) func(currentIndex int) {
	logThreshold := float64(0)
	return func(currentIndex int) {
		percentage := float64(100)
		if total > 0 {
			percentage = (float64(currentIndex+1) / float64(total)) * 100. // currentIndex+1 assuming zero based indexing
		}

		// report every 10 percent
		if percentage >= logThreshold+10 {
			logger.Info().Msgf("%s completion percentage: %v percent", msg, int(percentage))
			logThreshold += 10
		}
	}
}
