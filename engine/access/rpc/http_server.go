package rpc

import (
	"fmt"
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

// NewHTTPServer creates and intializes a new HTTP GRPC proxy server
func NewHTTPServer(
	grpcServer *grpc.Server,
	port int,
) *http.Server {
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
