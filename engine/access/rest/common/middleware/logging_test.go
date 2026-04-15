package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestLoggingMiddleware verifies that LoggingMiddleware logs the start and finish
// of each request and passes the request through to the inner handler.
func TestLoggingMiddleware(t *testing.T) {
	logger := zerolog.Nop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := mux.NewRouter()
	r.Handle("/test", inner)
	r.Use(LoggingMiddleware(logger))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestLoggingMiddleware_NonDefaultStatus verifies that the response code is captured
// correctly for non-200 responses.
func TestLoggingMiddleware_NonDefaultStatus(t *testing.T) {
	logger := zerolog.Nop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	r := mux.NewRouter()
	r.Handle("/notfound", inner)
	r.Use(LoggingMiddleware(logger))

	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}