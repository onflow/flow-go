package cmd

import (
	"crypto/rand"
	"strings"

	"github.com/rs/zerolog"
)

func GenerateRandomSeeds(n int, seedLen int) [][]byte {
	seeds := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		seeds = append(seeds, GenerateRandomSeed(seedLen))
	}
	return seeds
}

func GenerateRandomSeed(seedLen int) []byte {
	seed := make([]byte, seedLen)
	if _, err := rand.Read(seed); err != nil {
		log.Fatal().Err(err).Msg("cannot generate random seed")
	}
	return seed
}

// zeroLoggerHook is a simple logger hook used for capturing log output
// from zerolog during tests. It writes all log messages to a provided
// strings.Builder so they can be inspected later.
type zeroLoggerHook struct {
	logs *strings.Builder
}

func (h zeroLoggerHook) Run(_ *zerolog.Event, _ zerolog.Level, msg string) {
	h.logs.WriteString(msg)
}
