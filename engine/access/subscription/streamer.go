package subscription

import (
	"context"
)

type Streamer interface {
	Stream(ctx context.Context)
}
