package main

import (
	"testing"
	"time"

	ztsd "github.com/DataDog/zstd"
	csc "github.com/onflow/flow-go/cadence_script_compression"
)

const contracsDir = "../../contracts/mainnet"

func TestCompressLargeSize(t *testing.T) {
	// get a sample of contracts with large size compared to rest of the contracts
	contractNames := []string{
		"f61e40c19db2a9e2.Model.cdc",              // 703397
		"5f43c2fba744163c.SpaceModel.cdc",         // 602177
		"3885d9d426d2ef5c.EndModel.cdc",           // 405918
		"8624b52f9ddcd04a.FlowIDTableStaking.cdc", // 67533
		"0b2a3299cc857e29.TopShot.cdc",            // 44575
	}

	contracts := csc.ReadContracts(contracsDir)

	sumMbPerSec := 0.00
	sumRatio := 0.00
	for _, name := range contractNames {
		c := contracts[name]
		start := time.Now()

		dst := make([]byte, 0)
		dst, err := ztsd.Compress(dst, c.Data)
		if err != nil {
			t.Fatal(err)
		}

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
		dst, err := ztsd.Compress(dst, c.Data)
		if err != nil {
			t.Fatal(err)
		}

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
