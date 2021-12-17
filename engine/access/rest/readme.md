# Flow Access Node REST API Server

This package and subpackages implement the REST API Server for the [Flow OpenAPI definition](https://github.com/onflow/flow/blob/master/openapi/access.yaml)

## Packages:

`rest`: The HTTP handlers for all the request, server generator and the select filter.

`middleware`: The common [middlewares](https://github.com/gorilla/mux#middleware) that all request pass through.

`generate`: The generated models from https://app.swaggerhub.com/ (note: only the generated models are included and not the api implementation since that part us custom)


## Request lifecycle

1. Every incoming request passes through a common set of middlewares - logging middleware, query expandable and query select middleware defined in the middleware package.
2. A request is then sent to the `handler.ServeHTTP` function.
3. The `handler.ServeHTTP` function calls the appropriate `ApiHandlerFunc` handler function as defined in `server.go` e.g. `getBlocksByIDs` in `blocks.go` for a get blocks by IDs request or `getBlocksByHeight` for a get blocks by heights request etc.
4. Within the handler function, the request is first validated, then the necessary database lookups are performed and finally the appropriate response function is called. e.g. `blockResponse` for Block response.
5. After the response is generated, the select filter is applied if a `select` query param has been specified.
6. The Response is then sent to the client

