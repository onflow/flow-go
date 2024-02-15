package rpc

import (
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
)

type HTTPHeader struct {
	Key   string
	Value string
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

// newHTTPProxyServer creates a new HTTP GRPC proxy server.
func newHTTPProxyServer(grpcServer *grpc.Server) *http.Server {
	wrappedServer := grpcweb.WrapServer(
		grpcServer,
		grpcweb.WithOriginFunc(func(origin string) bool { return true }),
	)

	// register gRPC HTTP proxy
	mux := http.NewServeMux()
	mux.Handle("/", wrappedHandler(wrappedServer, defaultHTTPHeaders))

	httpServer := &http.Server{
		Handler: mux,
	}

	return httpServer
}

func wrappedHandler(wrappedServer *grpcweb.WrappedGrpcServer, headers []HTTPHeader) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		setResponseHeaders(res, headers)

		if req.Method == "OPTIONS" {
			return
		}

		wrappedServer.ServeHTTP(res, req)
	}
}

func setResponseHeaders(w http.ResponseWriter, headers []HTTPHeader) {
	for _, header := range headers {
		w.Header().Set(header.Key, header.Value)
	}
}
