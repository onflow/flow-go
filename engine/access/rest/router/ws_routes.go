package router

import (
	"net/http"

	"github.com/onflow/flow-go/engine/access/rest/websockets/legacy"
	"github.com/onflow/flow-go/engine/access/rest/websockets/legacy/routes"
)

type wsLegacyRoute struct {
	Name    string
	Method  string
	Pattern string
	Handler legacy.SubscribeHandlerFunc
}

var WSLegacyRoutes = []wsLegacyRoute{{
	Method:  http.MethodGet,
	Pattern: "/subscribe_events",
	Name:    "subscribeEvents",
	Handler: routes.SubscribeEvents,
}}
