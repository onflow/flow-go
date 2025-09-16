package migrations

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"strconv"
)

type accountMigrationResult struct {
	address      string
	beforeCount  int
	beforeSize   int
	afterCount   int
	afterSize    int
	deduplicated bool
}

type migrationResult struct {
	TotalDeduplicatedAccountCount   int `json:"deduplicated_account"`
	TotalUndeduplicatedAccountCount int `json:"undeduplicated_account"`
	TotalCountDelta                 int `json:"register_count_delta"`
	TotalSizeDelta                  int `json:"register_size_delta"`
}

func writeAccountMigrationResults(
	fileName string,
	migrationResults []accountMigrationResult,
) error {
	slices.SortFunc(migrationResults, func(a, b accountMigrationResult) int {
		r := cmp.Compare(a.beforeCount, b.beforeCount)
		if r != 0 {
			return r
		}
		return cmp.Compare(a.address, b.address)
	})

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create account migration result file %s: %w", fileName, err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	header := []string{
		"address",
		"before_count",
		"before_size",
		"after_count",
		"after_size",
		"deduplicated",
	}

	// Write header
	err = w.Write(header)
	if err != nil {
		return fmt.Errorf("failed to write header to %s: %w", fileName, err)
	}

	for _, migrationStats := range migrationResults {
		data := []string{
			migrationStats.address,
			strconv.Itoa(migrationStats.beforeCount),
			strconv.Itoa(migrationStats.beforeSize),
			strconv.Itoa(migrationStats.afterCount),
			strconv.Itoa(migrationStats.afterSize),
			strconv.FormatBool(migrationStats.deduplicated),
		}
		if err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write migration result for %s: %w", migrationStats.address, err)
		}
	}
	return nil
}
