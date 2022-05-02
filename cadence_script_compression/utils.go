package cadence_script_compression

import (
	"os"
	"path/filepath"
	"time"
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

func ReadContracts(dir string) map[string]*Contract {
	contracts := make(map[string]*Contract, 0)

	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	dirPath, _ := filepath.Abs(dir)
	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		contracts[file.Name()] = ReadContract(filePath)
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

func (c *CompressionComparison) CompressedSize() int {
	return len(c.CompressedData)
}

func (c *CompressionComparison) UnCompressedSize() int {
	return len(c.UncompressedData)
}

// CompressionRatio is the uncompressed size / compressed size
func CompressionRatio(uncompressed, compressed float64) float64 {
	return uncompressed / compressed
}

func CompressionSpeed(size float64, start time.Time) float64 {
	return (size / (1 << 20)) / (float64(time.Since(start)) / (float64(time.Second)))
}
