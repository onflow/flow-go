package rpc

import (
	"context"
	"path/filepath"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO: read this map from an input file
// TODO: fine-tune these numbers
// APILimit is the map of an API and it's rate limit (request per second)
var APILimit = map[string]uint{
	"Ping":                       300,
	"GetLatestBlockHeader":       300,
	"GetBlockHeaderByID":         300,
	"GetBlockHeaderByHeight":     300,
	"GetLatestBlock":             300,
	"GetBlockByID":               300,
	"GetBlockByHeight":           300,
	"GetCollectionByID":          300,
	"SendTransaction":            300,
	"GetTransaction":             300,
	"GetTransactionResult":       300,
	"GetAccount":                 300,
	"GetAccountAtLatestBlock":    300,
	"GetAccountAtBlockHeight":    300,
	"ExecuteScriptAtLatestBlock": 300,
	"ExecuteScriptAtBlockID":     300,
	"ExecuteScriptAtBlockHeight": 300,
	"GetEventsForHeightRange":    300,
	"GetEventsForBlockIDs":       300,
	"GetNetworkParameters":       300,
}

var defaultRateLimit = 10 // 10 calls per second
var DefaultBurst = 3      // at most 3 call that are made at the same time

// rateLimiterInterceptor rate limits the
type rateLimiterInterceptor struct {
	log zerolog.Logger

	// default rate limiter for APIs whose rate limit is not explicitly defined
	defaultLimiter *rate.Limiter

	// a map of api and its limiter
	methodLimiterMap map[string]*rate.Limiter
}

func NewRateLimiterInterceptor(log zerolog.Logger) *rateLimiterInterceptor {

	defaultLimiter := rate.NewLimiter(rate.Limit(defaultRateLimit), DefaultBurst)
	methodLimiterMap := make(map[string]*rate.Limiter, len(APILimit))

	// read rate limit values for each API and create a limiter for each
	for api, limit := range APILimit {
		methodLimiterMap[api] = rate.NewLimiter(rate.Limit(limit), DefaultBurst)
	}

	return &rateLimiterInterceptor{
		defaultLimiter:   defaultLimiter,
		methodLimiterMap: methodLimiterMap,
		log:              log,
	}
}

func (interceptor *rateLimiterInterceptor) unaryServerInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (resp interface{}, err error) {

	// remove the package name (e.g. "/flow.access.AccessAPI/Ping" to "Ping")
	methodName := filepath.Base(info.FullMethod)

	// look up the limiter
	limiter := interceptor.methodLimiterMap[methodName]

	// if not found, use the default limiter
	if limiter == nil {

		// log error since each API should typically have a defined rate limit
		interceptor.log.Error().Str("method", methodName).Msg("rate limit not defined, using default limit")

		limiter = interceptor.defaultLimiter
	}

	// check if request within limit
	if !limiter.Allow() {

		// log the limit violation
		interceptor.log.Info().
			Str("method", methodName).
			Interface("request", req).
			Float64("limit", float64(limiter.Limit())).
			Msg("rate limit exceeded")

		// reject the request
		return nil, status.Errorf(codes.ResourceExhausted, "%s rate limit reached, please retry later.",
			info.FullMethod)
	}

	// call the handler
	h, err := handler(ctx, req)

	return h, err
}
