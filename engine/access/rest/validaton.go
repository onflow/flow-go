package rest

import (
	"context"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"net/http"
	"net/url"
)

// SchemaValidation implements validation against open API schema
type SchemaValidation struct {
	router routers.Router
	schema *openapi3.T
}

func newSchemaValidation() (*SchemaValidation, error) {
	// initialize open API schema
	loader := openapi3.NewLoader()
	u, err := url.Parse("https://raw.githubusercontent.com/onflow/flow/master/openapi/access.yaml") // todo(sideninja) we should tag and version schema
	if err != nil {
		return nil, err
	}

	// load open API schema
	schema, err := loader.LoadFromURI(u)
	if err != nil {
		return nil, err
	}

	// validate open API schema
	err = schema.Validate(context.Background())
	if err != nil {
		return nil, err
	}

	router, err := gorillamux.NewRouter(schema)
	if err != nil {
		return nil, err
	}

	return &SchemaValidation{
		router: router,
		schema: schema,
	}, nil
}

// Validate a http request against a schema
func (v *SchemaValidation) Validate(r *http.Request) error {
	route, path, err := v.router.FindRoute(r)
	if err != nil {
		return err
	}

	validation := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: path,
		Route:      route,
	}

	return openapi3filter.ValidateRequest(r.Context(), validation)
}
