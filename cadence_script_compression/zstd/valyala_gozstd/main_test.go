package main

import (
	"io/ioutil"
	"testing"
	"time"

	csc "github.com/onflow/flow-go/cadence_script_compression"
	zstd "github.com/valyala/gozstd"
)

const (
	contracsDir = "../../contracts/mainnet"
	dictionary  = "../MAINNET_DICT"
)

func TestCompressLargeSize(t *testing.T) {
	// get a sample of contracts with large size compared to rest of the contracts
	contractNames := []string{
		"3885d9d426d2ef5c.Model.cdc",         // 703397
		"3885d9d426d2ef5c.SpaceModel.cdc",    // 602177
		"3885d9d426d2ef5c.EndModel.cdc",      // 405918
		"2817a7313c740232.TenantService.cdc", // 60130
		"0b2a3299cc857e29.TopShot.cdc",       // 44575
	}

	contracts := csc.ReadContracts(contracsDir)

	sumMbPerSec := 0.00
	sumRatio := 0.00
	for _, name := range contractNames {
		c := contracts[name]
		start := time.Now()

		dst := make([]byte, 0)
		dst = zstd.Compress(dst, c.Data)

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec

		ratio := csc.CompressionRatio(float64(len(c.Data)), float64(len(dst)))
		sumRatio = sumRatio + ratio

		t.Logf("Name: %s | UnCompressed Size: %d | Compressed Size: %d | Speed: %.2f MB/s | Ratio: %.2f", c.Name, c.Size, len(dst), mbpersec, ratio)
	}

	avgRatio := sumRatio / float64(len(contractNames))
	avgSpeed := sumMbPerSec / float64(len(contractNames))

	t.Logf("Average Compression Ratio: %.2f , Average Compression Speed: %.2f MB/s", avgRatio, avgSpeed)
}

func TestCompressAllContracts(t *testing.T) {
	contracts := csc.ReadContracts(contracsDir)

	sumMbPerSec := 0.00
	sumRatio := 0.00
	for _, c := range contracts {
		start := time.Now()

		dst := make([]byte, 0)
		dst = zstd.Compress(dst, c.Data)

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec

		ratio := csc.CompressionRatio(float64(len(c.Data)), float64(len(dst)))
		sumRatio = sumRatio + ratio

		t.Logf("Name: %s | UnCompressed Size: %d | Compressed Size: %d | Speed: %.2f MB/s | Ratio: %.2f", c.Name, c.Size, len(dst), mbpersec, ratio)
	}

	avgRatio := sumRatio / float64(len(contracts))
	avgSpeed := sumMbPerSec / float64(len(contracts))

	t.Logf("Average Compression Ratio: %.2f , Average Compression Speed: %.2f MB/s", avgRatio, avgSpeed)
}

func TestCompressAllCDictBestCompression(t *testing.T) {
	contracts := csc.ReadContracts(contracsDir)

	sumMbPerSec := 0.00
	sumRatio := 0.00

	dictFile, err := ioutil.ReadFile(dictionary)
	if err != nil {
		t.Fatal(err)
	}

	dict, err := zstd.NewCDictLevel(dictFile, 22)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range contracts {
		start := time.Now()

		dst := make([]byte, 0)

		dst = zstd.CompressDict(dst, c.Data, dict)

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec

		ratio := csc.CompressionRatio(float64(len(c.Data)), float64(len(dst)))
		sumRatio = sumRatio + ratio

		t.Logf("Name: %s | UnCompressed Size: %d | Compressed Size: %d | Speed: %.2f MB/s | Ratio: %.2f", c.Name, c.Size, len(dst), mbpersec, ratio)
	}

	avgRatio := sumRatio / float64(len(contracts))
	avgSpeed := sumMbPerSec / float64(len(contracts))

	t.Logf("Average Compression Ratio: %.2f , Average Compression Speed: %.2f MB/s", avgRatio, avgSpeed)
}

func TestCompressAllCDictBestSpeed(t *testing.T) {
	contracts := csc.ReadContracts(contracsDir)

	sumMbPerSec := 0.00
	sumRatio := 0.00

	dictFile, err := ioutil.ReadFile(dictionary)
	if err != nil {
		t.Fatal(err)
	}

	dict, err := zstd.NewCDictLevel(dictFile, 1)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range contracts {
		start := time.Now()

		dst := make([]byte, 0)

		dst = zstd.CompressDict(dst, c.Data, dict)

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec

		ratio := csc.CompressionRatio(float64(len(c.Data)), float64(len(dst)))
		sumRatio = sumRatio + ratio

		t.Logf("Name: %s | UnCompressed Size: %d | Compressed Size: %d | Speed: %.2f MB/s | Ratio: %.2f", c.Name, c.Size, len(dst), mbpersec, ratio)
	}

	avgRatio := sumRatio / float64(len(contracts))
	avgSpeed := sumMbPerSec / float64(len(contracts))

	t.Logf("Average Compression Ratio: %.2f , Average Compression Speed: %.2f MB/s", avgRatio, avgSpeed)
}

func TestDecompressAllContracts(t *testing.T) {
	contracts := csc.ReadContracts(contracsDir)

	sumMbPerSec := 0.00
	for _, c := range contracts {
		compressed := zstd.Compress(nil, c.Data)

		start := time.Now()
		dst, err := zstd.Decompress(nil, compressed)
		if err != nil {
			t.Fatal(err)
		}

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec

		t.Logf("Name: %s | Orig Size: %d | Compressed Size: %d | UnCompressed Size: %d | Speed: %.2f MB/s", c.Name, c.Size, len(compressed), len(dst), mbpersec)
	}

	avgSpeed := sumMbPerSec / float64(len(contracts))

	t.Logf("Average DeCompression Speed: %.2f MB/s", avgSpeed)
}
