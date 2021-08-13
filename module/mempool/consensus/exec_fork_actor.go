package consensus

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

type ExecForkActor func([]*flow.IncorporatedResultSeal)

func LogForkAndCrash(log zerolog.Logger) ExecForkActor {
	return func(conflictingSeals []*flow.IncorporatedResultSeal) {
		l := log.Fatal().Int("number conflicting seals", len(conflictingSeals))
		for i, s := range conflictingSeals {
			sealAsJson, err := json.Marshal(s)
			if err != nil {
				err = fmt.Errorf("failed to marshal candidate seal to json: %w", err)
				l.Str(fmt.Sprintf("seal_%d", i), err.Error())
				continue
			}
			l = l.Str(fmt.Sprintf("seal_%d", i), string(sealAsJson))
		}
		l.Msg("inconsistent seals for the same block")
	}
}
