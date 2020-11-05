package cmd

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/io"
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

func readJSON(path string, target interface{}) {
	dat, err := io.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read json")
	}
	err = json.Unmarshal(dat, target)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot unmarshal json in file")
	}
}

func writeJSON(path string, data interface{}) {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot marshal json")
	}

	writeText(path, bz)
}

func writeText(path string, data []byte) {
	path = filepath.Join(flagOutdir, path)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create output dir")
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		log.Fatal().Err(err).Msg("could not write file")
	}

	log.Info().Msgf("wrote file %v", path)
}

func pubKeyToString(key crypto.PublicKey) string {
	return fmt.Sprintf("%x", key.Encode())
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

// pathExists
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func nodeCountByRole(nodes []model.NodeInfo) map[flow.Role]uint16 {
	roleCounts := map[flow.Role]uint16{
		flow.RoleCollection:   0,
		flow.RoleConsensus:    0,
		flow.RoleExecution:    0,
		flow.RoleVerification: 0,
		flow.RoleAccess:       0,
	}
	for _, node := range nodes {
		roleCounts[node.Role] = roleCounts[node.Role] + 1
	}

	return roleCounts
}
