package ingestion2

import (
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ErrorMessageRequester struct {
	log          zerolog.Logger
	blockIDsChan chan flow.Identifier
	core         *tx_error_messages.TxErrorMessagesCore
	exists       *atomic.Bool
}

func NewErrorMessageRequester(log zerolog.Logger, core *tx_error_messages.TxErrorMessagesCore) *ErrorMessageRequester {
	if core == nil {
		return &ErrorMessageRequester{
			log:          log,
			blockIDsChan: nil,
			core:         core,
		}
	}

	return &ErrorMessageRequester{
		log:          log,
		blockIDsChan: make(chan flow.Identifier, 1),
		core:         core,
		exists:       atomic.NewBool(true),
	}
}

func (p *ErrorMessageRequester) Notify(blockID flow.Identifier) {
	if p.exists.Load() {
		p.blockIDsChan <- blockID
	}
}

func (p *ErrorMessageRequester) RequestErrorMessages(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	if !p.exists.Load() {
		close(p.blockIDsChan)
		return
	}

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
