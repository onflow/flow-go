package common

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
)

// PrettyPrintEntity pretty print a flow entity
func PrettyPrintEntity(entity flow.Hashable) {
	log.Info().Msgf("entity id: %v", entity.Hash())
	PrettyPrint(entity)
}

// PrettyPrint an interface
func PrettyPrint(entity interface{}) {
	bytes, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("could not marshal interface into json")
	}

	fmt.Println(string(bytes))
}
