package util

import "github.com/rs/zerolog"

func LogProgress(msg string, total int, logger *zerolog.Logger) func(current int) {
	lookup := make(map[int]int)
	// report every 10 percent
	for i := 1; i < 10; i++ { // [1...9]
		lookup[total/10*i] = i * 10
	}
	if total > 0 {
		lookup[total-1] = 100
	}
	return func(current int) {
		percentage, ok := lookup[current]
		if ok {
			logger.Info().Msgf("%s completion percentage: %v percent", msg, percentage)
		}
	}
}
