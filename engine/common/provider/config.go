package provider

import (
	"time"
)

type Config struct {
	BatchThreshold uint
	BatchInterval  time.Duration
}
