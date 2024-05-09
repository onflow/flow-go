package cmd

import (
	"crypto/rand"
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
