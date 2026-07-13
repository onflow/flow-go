package websockets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/limiters"
)

func TestConnectionLimitedHandler_RejectsWhenLimitReached(t *testing.T) {
	limiter, err := limiters.NewConcurrencyLimiter(1)
	require.NoError(t, err)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	httpHandler := common.NewHttpHandler(zerolog.Nop(), flow.Localnet.Chain(), 1024, 1024)
	handler := NewConnectionLimitedHandler(zerolog.Nop(), httpHandler, inner, limiter)

	// Saturate the limiter by blocking inside Allow.
	started := make(chan struct{})
	unblock := make(chan struct{})
	go func() {
		limiter.Allow(func() {
			close(started)
			<-unblock
		})
	}()
	<-started

	// A request while the limiter is full must return 429.
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	close(unblock)
}
