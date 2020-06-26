package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
)

type HTTPHeader struct {
	Key   string
	Value string
}

// HTTPServer is the HTTP GRPC proxy server
type HTTPServer struct {
	httpServer *http.Server
}

var defaultHTTPHeaders = []HTTPHeader{
		{
			Key:   "Access-Control-Allow-Origin",
			Value: "*",
		},
		{
			Key:   "Access-Control-Allow-Methods",
			Value: "POST, GET, OPTIONS, PUT, DELETE",
		},
		{
			Key:   "Access-Control-Allow-Headers",
			Value: "*",
		},
	}

func NewHTTPServer(
	grpcServer *grpc.Server,
	port int,
) *HTTPServer {
	wrappedServer := grpcweb.WrapServer(
		grpcServer,
		grpcweb.WithOriginFunc(func(origin string) bool { return true }),
	)

	mux := http.NewServeMux()

	// register gRPC HTTP proxy
	mux.Handle("/", wrappedHandler(wrappedServer, defaultHTTPHeaders))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &HTTPServer{
		httpServer: httpServer,
	}
}

func (h *HTTPServer) Start() error {
	err := h.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (h *HTTPServer) Stop() {
	_ = h.httpServer.Shutdown(context.Background())
}

func wrappedHandler(wrappedServer *grpcweb.WrappedGrpcServer, headers []HTTPHeader) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		setResponseHeaders(res, headers)

		if (*req).Method == "OPTIONS" {
			return
		}

		wrappedServer.ServeHTTP(res, req)
	}
}

func setResponseHeaders(w *http.ResponseWriter, headers []HTTPHeader) {
	for _, header := range headers {
		(*w).Header().Set(header.Key, header.Value)
	}
}
