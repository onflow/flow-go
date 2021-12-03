# Flow Access Node REST API Server

This package and subpackages implement the REST API Server for the [Flow OpenAPI definition](https://github.com/onflow/flow/blob/master/openapi/access.yaml)

## Packages:

`rest`: The HTTP handlers for all the request, server generator and the select filter.

`middleware`: The common [middlewares](https://github.com/gorilla/mux#middleware) that all request pass through.

`generate`: The generated models from https://app.swaggerhub.com/ (note: only the generated models are included and not the api implementation since that part us custom)


## Request lifecycle

1. Each HTTP request gets directed to the appropriate `ApiHandlerFunc` function as defined in `server.go` at server startup e.g. Get Blocks by ID or heights go to `blocks.go`, Get Collections go to `collections.go`.
2. Before reaching the handler function, each request goes through a common set of middlewares.
3. Within the handler function, the request is first validated, then a database lookup is performed and finally the appropriate response function is called. e.g. `blockResponse` for Block response.
4. After the response is generated, the select filter is applied if a `select` query param has been specified.

