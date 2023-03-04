package util

import (
	"github.com/rs/zerolog"
)

func LogProgress(msg string, total int, logger *zerolog.Logger) func(currentIndex int) {
	return LogProgressWithThreshold(10, msg, total, logger)
}

func LogProgressWithThreshold(logThreshold int, msg string, total int, logger *zerolog.Logger) func(currentIndex int) {
	threshold := float64(logThreshold)
	nextThreshold := threshold
	return func(currentIndex int) {
		percentage := float64(100)
		if total > 0 {
			percentage = (float64(currentIndex+1) / float64(total)) * 100. // currentIndex+1 assuming zero based indexing
		}

		// report every 10 percent
		if percentage >= nextThreshold {
			logger.Info().Msgf("%s completion percentage: %v percent", msg, int(percentage))
			nextThreshold += threshold
		}
	}
}
