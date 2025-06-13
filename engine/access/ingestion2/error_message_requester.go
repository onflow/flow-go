package ingestion2

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ErrorMessageRequester interface {
	Notify(blockID flow.Identifier)
	StartRequesting(ctx irrecoverable.SignalerContext, ready component.ReadyFunc)
}

type ErrorMessageRequesterImpl struct {
	log          zerolog.Logger
	blockIDsChan chan flow.Identifier
	core         *tx_error_messages.TxErrorMessagesCore
}

func NewErrorMessageRequester(log zerolog.Logger, core *tx_error_messages.TxErrorMessagesCore) *ErrorMessageRequesterImpl {
	return &ErrorMessageRequesterImpl{
		log:          log,
		blockIDsChan: make(chan flow.Identifier, 1),
		core:         core,
	}
}

func (p *ErrorMessageRequesterImpl) Notify(blockID flow.Identifier) {
	p.blockIDsChan <- blockID
}

func (p *ErrorMessageRequesterImpl) StartRequesting(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	defer close(p.blockIDsChan)

	for {
		select {
		case <-ctx.Done():
			return
		case blockID := <-p.blockIDsChan:
			err := p.core.FetchErrorMessages(ctx, blockID)
			if err != nil {
				// TODO: we should revisit error handling here.
				// Errors that come from querying the EN and possibly ExecutionNodesForBlockID should be logged and
				// retried later, while others should cause an exception.
				p.log.Error().
					Err(err).
					Msg("error encountered while processing transaction result error messages by receipts")
			}
		}
	}
}

type NoopErrorMessageRequester struct{}

func NewNoopErrorMessageRequester() *NoopErrorMessageRequester {
	return &NoopErrorMessageRequester{}
}

func (*NoopErrorMessageRequester) Notify(flow.Identifier) {}

func (*NoopErrorMessageRequester) StartRequesting(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
}
