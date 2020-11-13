package common

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
)

func PrettyPrintEntity(entity flow.Entity) {
	bytes, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("could not marshal entity into json")
	}

	log.Info().Msgf("entity id: %v", entity.ID())
	fmt.Println(string(bytes))
}

func PrettyPrint(entity interface{}) {
	bytes, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("could not marshal interface into json")
	}

	fmt.Println(string(bytes))
}
