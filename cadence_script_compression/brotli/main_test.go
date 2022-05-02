package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	csc "github.com/onflow/flow-go/cadence_script_compression"
)

const (
	contracsDir = "../contracts/mainnet"
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
		dst := make([]byte, 0)

		r := bytes.NewReader(c.Data)
		w := bytes.NewBuffer(dst)
		bw := brotli.NewWriter(w)

		start := time.Now()
		_, _ = io.Copy(bw, r)

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec

		_ = bw.Close() // Make sure the writer is closed
		dst = w.Bytes()

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
		dst := make([]byte, 0)

		r := bytes.NewReader(c.Data)
		w := bytes.NewBuffer(dst)
		bw := brotli.NewWriter(w)

		start := time.Now()
		_, _ = io.Copy(bw, r)

		mbpersec := csc.CompressionSpeed(float64(len(c.Data)), start)
		sumMbPerSec = sumMbPerSec + mbpersec
:
		_ = bw.Close() // Make sure the writer is closed
		dst = w.Bytes()

		ratio := csc.CompressionRatio(float64(len(c.Data)), float64(len(dst)))
		sumRatio = sumRatio + ratio

		t.Logf("Name: %s | UnCompressed Size: %d | Compressed Size: %d | Speed: %.2f MB/s | Ratio: %.2f", c.Name, c.Size, len(dst), mbpersec, ratio)
	}

	avgRatio := sumRatio / float64(len(contracts))
	avgSpeed := sumMbPerSec / float64(len(contracts))

	t.Logf("Average Compression Ratio: %.2f , Average Compression Speed: %.2f MB/s", avgRatio, avgSpeed)
}
