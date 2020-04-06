package cmd

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/crypto"
)

func generateRandomSeeds(n int) [][]byte {
	seeds := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		seeds = append(seeds, generateRandomSeed())
	}
	return seeds
}

func generateRandomSeed() []byte {
	seed := make([]byte, randomSeedBytes)
	if n, err := rand.Read(seed); err != nil || n != randomSeedBytes {
		log.Fatal().Err(err).Msg("cannot generate random seeds")
	}
	return seed
}

func readJSON(filename string, target interface{}) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read json")
	}
	err = json.Unmarshal(dat, target)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot unmarshal json in file")
	}
}

func writeJSON(filename string, data interface{}) {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot marshal json")
	}

	path := filepath.Join(flagOutdir, filename)

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create output dir")
	}

	err = ioutil.WriteFile(path, bz, 0644)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write file")
	}

	log.Info().Msgf("wrote file %v", path)
}

func pubKeyToBytes(key crypto.PublicKey) []byte {
	enc, err := key.Encode()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot encode public key")
	}
	return enc
}

func pubKeyToString(key crypto.PublicKey) string {
	return fmt.Sprintf("%x", pubKeyToBytes(key))
}

func filesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
