package common

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	FailureThresholdPercent float64 `json:"failures_threshold_percent"`
	FailuresSliceMax        int     `json:"failures_slice_max"`

	DurationThresholdSeconds float64 `json:"duration_threshold_seconds"`
	DurationSliceMax         int     `json:"duration_slice_max"`
}

func ReadProperties(directory string) Config {
	propertyFile := filepath.Join(directory, "flaky-test-monitor.json")

	var config Config
	configBytes, err := os.ReadFile(propertyFile)
	AssertNoError(err, "error reading property file")
	err = json.Unmarshal(configBytes, &config)
	AssertNoError(err, "error unmarshalling property file")

	return config
}
