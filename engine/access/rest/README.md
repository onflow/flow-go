# Flow Access Node HTTP API Server

This package and subpackages implement the HTTP API Server for
the [Flow OpenAPI definition](https://github.com/onflow/flow/blob/master/openapi/access.yaml). The API documentation is
available on our [docs site](https://docs.onflow.org/http-api/).

## Packages

- `rest`: The HTTP handlers for the server generator and the select filter, implementation of handling local requests.
- `middleware`: The common [middlewares](https://github.com/gorilla/mux#middleware) that all request pass through.
- `models`: The generated models using openapi generators and implementation of model builders.
- `request`: Implementation of API requests that provide validation for input data and build request models.
- `routes`: The common HTTP handlers for all the requests, tests for each request.
- `apiproxy`: Implementation of proxy backend handler which includes the local backend and forwards the methods which 
can't be handled locally to an upstream using gRPC API. This is used by observers that don't have all data in their
local db.

## Request lifecycle

1. Every incoming request passes through a common set of middlewares - logging middleware, query expandable and query
   select middleware defined in the middleware package.
2. Each request is then wrapped by our handler (`rest/handler.go`) and request input data is used to build the request
   models defined in request package.
3. The request is then sent to the corresponding API handler based on the configuration in the router.
4. Each handler implements actions to perform the request (database lookups etc) and after the response is built using
   the model builders defined in models package.
5. Returned value is then again handled by our wrapped handler making sure to correctly handle successful and failure
   responses.

## Maintaining

### Updating OpenAPI Schema

Make sure the OpenAPI schema if first updated and merged into
master [on the hosted repository](https://github.com/onflow/flow/tree/master/openapi). After you can use the make
command to generated updated models:

```makefile
make generate-openapi
```

### Adding New API Endpoints

A new endpoint can be added by first implementing a new request handler, a request handle is a function in the routes
package that complies with function interfaced defined as:

```go
type ApiHandlerFunc func (
r *request.Request,
backend access.API,
generator models.LinkGenerator,
) (interface{}, error)
```

That handler implementation needs to be added to the `router.go` with corresponding API endpoint and method. If the data
is not available on observers, an override the method is needed in the backend handler `RestProxyHandler` for request 
forwarding. Adding a new API endpoint also requires for a new request builder to be implemented and added in request 
package. Make sure to not forget about adding tests for each of the API handler.
