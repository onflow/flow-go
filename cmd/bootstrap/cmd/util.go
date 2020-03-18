package cmd

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

func generateRandomSeeds(n int) [][]byte {
	seeds := make([][]byte, 0, n)

	for i := 0; i < n; i++ {
		seeds = append(seeds, make([]byte, 64))
		if _, err := rand.Read(seeds[i]); err != nil {
			log.Fatal().Err(err).Msg("cannot generate random seeds")
		}
	}

	return seeds
}

func readYaml(filename string, target interface{}) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read file")
	}
	err = yaml.Unmarshal(dat, target)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot unmarshal yaml in file")
	}
}

func writeYaml(filename string, data interface{}) {
	bz, err := yaml.Marshal(data)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot marshal yaml")
	}

	err = ioutil.WriteFile(filename, bz, 0644)

	log.Info().Msgf("wrote yaml to file %v", filename)
}

func pubKeyToBytes(key crypto.PublicKey) []byte {
	enc, err := key.Encode()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot encode public key")
	}
	return enc
}

func pubKeyToString(key crypto.PublicKey) string {
	return fmt.Sprintf("%#x", pubKeyToBytes(key))
}

func privKeyToBytes(key crypto.PrivateKey) []byte {
	enc, err := key.Encode()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot encode private key")
	}
	return enc
}

func privKeyToString(key crypto.PrivateKey) string {
	return fmt.Sprintf("%#x", privKeyToBytes(key))
}
