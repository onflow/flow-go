package main

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/grpcutils"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// config:
// * dbdir
// * start height
// * end height
func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := startBackend(ctx); err != nil {
		fmt.Println(err)
	}

	<-ctx.Done()
}

func startBackend(ctx context.Context) error {
	scriptExecMode := backend.IndexQueryModeLocalOnly
	logger := zerolog.New(os.Stdout)

	loggedScripts, err := lru.New[[md5.Size]byte, time.Time](backend.DefaultLoggedScriptsCacheSize)
	if err != nil {
		return fmt.Errorf("failed to initialize script logging cache: %w", err)
	}

	var headers storage.Headers
	var executionReceipts storage.ExecutionReceipts
	var state protocol.State
	var scriptExecutor execution.ScriptExecutor

	be := backend.NewBackendScripts(
		logger,
		headers,
		executionReceipts,
		state,
		nil, // local only
		metrics.NewNoopCollector(),
		loggedScripts,
		nil,
		scriptExecutor,
		scriptExecMode,
	)

	rpcHandler := &handler{api: be}

	unsecureGrpcServer := grpcserver.NewGrpcServerBuilder(
		logger,
		"0.0.0.0:9999",
		grpcutils.DefaultMaxMsgSize,
		false,
		nil,
		nil,
	).Build()

	accessproto.RegisterAccessAPIServer(unsecureGrpcServer.Server, rpcHandler)

	signalCtx, errCh := irrecoverable.WithSignaler(ctx)
	go unsecureGrpcServer.Start(signalCtx)
	go func() {
		select {
		case <-signalCtx.Done():
		case err := <-errCh:
			log.Fatalf("irrecoverable error: %v", err)
		}
	}()

	select {
	case <-signalCtx.Done():
	case <-unsecureGrpcServer.Ready():
	}

	return nil
}

var _ accessproto.AccessAPIServer = (*handler)(nil)

type handler struct {
	*access.UnimplementedAccessAPIServer

	api *backend.BackendScripts
}

func (h *handler) ExecuteScriptAtHeight(ctx context.Context, req *accessproto.ExecuteScriptAtBlockHeightRequest) (*accessproto.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()
	height := req.GetBlockHeight()

	value, err := h.api.ExecuteScript(ctx, backend.NewScriptExecutionRequest(flow.ZeroID, height, script, arguments))
	if err != nil {
		return nil, err
	}

	return &accessproto.ExecuteScriptResponse{
		Value: value,
	}, nil
}
