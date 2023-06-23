package apiproxy

import (
	"context"

	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

// RestRouter is a structure that represents the routing proxy algorithm for observer node.
// It splits requests between a local requests and forward requests which can't be handled locally to an upstream access node.
type RestRouter struct {
	Logger   zerolog.Logger
	Metrics  metrics.ObserverMetrics
	Upstream api.RestServerApi
	Observer api.RestServerApi
}

func (r *RestRouter) log(handler, rpc string, err error) {
	code := status.Code(err)
	r.Metrics.RecordRPC(handler, rpc, code)

	logger := r.Logger.With().
		Str("handler", handler).
		Str("rest_method", rpc).
		Str("rest_code", code.String()).
		Logger()

	if err != nil {
		logger.Error().Err(err).Msg("request failed")
		return
	}

	logger.Info().Msg("request succeeded")
}

func (r *RestRouter) GetTransactionByID(req request.GetTransaction, context context.Context, link models.LinkGenerator, chain flow.Chain) (models.Transaction, error) {
	res, err := r.Upstream.GetTransactionByID(req, context, link, chain)
	r.log("upstream", "GetNodeVersionInfo", err)
	return res, err
}

func (r *RestRouter) CreateTransaction(req request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error) {
	res, err := r.Upstream.CreateTransaction(req, context, link)
	r.log("upstream", "CreateTransaction", err)
	return res, err
}

func (r *RestRouter) GetTransactionResultByID(req request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error) {
	res, err := r.Upstream.GetTransactionResultByID(req, context, link)
	r.log("upstream", "GetTransactionResultByID", err)
	return res, err
}

func (r *RestRouter) GetBlocksByIDs(req request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error) {
	res, err := r.Observer.GetBlocksByIDs(req, context, expandFields, link)
	r.log("observer", "GetBlocksByIDs", err)
	return res, err
}

func (r *RestRouter) GetBlocksByHeight(req *request.Request, link models.LinkGenerator) ([]*models.Block, error) {
	res, err := r.Observer.GetBlocksByHeight(req, link)
	r.log("observer", "GetBlocksByHeight", err)
	return res, err
}

func (r *RestRouter) GetBlockPayloadByID(req request.GetBlockPayload, context context.Context, link models.LinkGenerator) (models.BlockPayload, error) {
	res, err := r.Observer.GetBlockPayloadByID(req, context, link)
	r.log("observer", "GetBlockPayloadByID", err)
	return res, err
}

func (r *RestRouter) GetExecutionResultByID(req request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error) {
	res, err := r.Upstream.GetExecutionResultByID(req, context, link)
	r.log("upstream", "GetExecutionResultByID", err)
	return res, err
}

func (r *RestRouter) GetExecutionResultsByBlockIDs(req request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error) {
	res, err := r.Upstream.GetExecutionResultsByBlockIDs(req, context, link)
	r.log("upstream", "GetExecutionResultsByBlockIDs", err)
	return res, err
}

func (r *RestRouter) GetCollectionByID(req request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, chain flow.Chain) (models.Collection, error) {
	res, err := r.Upstream.GetCollectionByID(req, context, expandFields, link, chain)
	r.log("upstream", "GetCollectionByID", err)
	return res, err
}

func (r *RestRouter) ExecuteScript(req request.GetScript, context context.Context, link models.LinkGenerator) ([]byte, error) {
	res, err := r.Upstream.ExecuteScript(req, context, link)
	r.log("upstream", "ExecuteScript", err)
	return res, err
}

func (r *RestRouter) GetAccount(req request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error) {
	res, err := r.Upstream.GetAccount(req, context, expandFields, link)
	r.log("upstream", "GetAccount", err)
	return res, err
}

func (r *RestRouter) GetEvents(req request.GetEvents, context context.Context) (models.BlocksEvents, error) {
	res, err := r.Upstream.GetEvents(req, context)
	r.log("upstream", "GetEvents", err)
	return res, err
}

func (r *RestRouter) GetNetworkParameters(req *request.Request) (models.NetworkParameters, error) {
	res, err := r.Observer.GetNetworkParameters(req)
	r.log("observer", "GetNetworkParameters", err)
	return res, err
}

func (r *RestRouter) GetNodeVersionInfo(req *request.Request) (models.NodeVersionInfo, error) {
	res, err := r.Observer.GetNodeVersionInfo(req)
	r.log("observer", "GetNodeVersionInfo", err)
	return res, err
}
