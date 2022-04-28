package cadence_script_compression

import (
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/model/flow"
)

const (
	mainnetContractsDir = "./contracts/mainnet"
)

type Contract struct {
	Data []byte
	Name string
	Size int64
}

type CompressionComparison struct {
	CompressedData   []byte
	UncompressedData []byte
}

func ReadContracts(chainID flow.ChainID) []*Contract {
	contracts := make([]*Contract, 0)

	dir := getDir(chainID)

	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	dirPath, _ := filepath.Abs(dir)
	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		contracts = append(contracts, ReadContract(filePath))
	}

	return contracts
}

func ReadContract(path string) *Contract {
	stat, err := os.Stat(path)
	if err != nil {
		panic(err)
	}

	c := readContract(path)
	c.Size = stat.Size()
	c.Name = stat.Name()

	return c
}

func readContract(path string) *Contract {
	f, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return &Contract{
		Data: f,
	}
}

func getDir(chainID flow.ChainID) string {
	if chainID == flow.Mainnet {
		return mainnetContractsDir
	}

	return ""
}

func (c *CompressionComparison) CompressedSize() int {
	return len(c.CompressedData)
}

func (c *CompressionComparison) UnCompressedSize() int {
	return len(c.UncompressedData)
}

// CompressionRatio is the uncompressed size / compressed size
func (c *CompressionComparison) CompressionRatio() float64 {
	return float64(len(c.UncompressedData)) / float64(len(c.CompressedData))
}
