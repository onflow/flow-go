# AGENTS.md

## Localnet (Local Network Testing)

A Docker-based local Flow network for manual end-to-end testing. Located in `integration/localnet/`. Particularly useful for API development and testing against a fully running network.

### Quick Start

All commands run from `integration/localnet/`:

- `make bootstrap` - Generate network config and bootstrap data
- `make start` - Build images and start all nodes + metrics stack
- `make start-cached` - Start without rebuilding (faster iteration)
- `make stop` - Stop all services
- `make clean-data` - Remove all generated data and bootstrap filest

### Network Configuration

Node counts are configured via env vars during bootstrap:

- `COLLECTION`, `CONSENSUS`, `EXECUTION`, `VERIFICATION`, `ACCESS`, `OBSERVER`
- Example: `make -e COLLECTION=2 CONSENSUS=3 ACCESS=2 bootstrap`
- Pre-configured variants:
  - `make bootstrap-light` - Minimal network
  - `make bootstrap-short-epochs` - Short epochs for epoch testing

### Accessing the Access Node API

Default ports for `access_1`:

- gRPC: `localhost:4001`
- HTTP: `localhost:4003`
- REST API: `localhost:4004`
- Admin tool: `localhost:4000`
- Full port mappings: `integration/localnet/ports.nodes.json`

### Testing Endpoints

REST API:
```sh
curl -s http://localhost:4004/v1/blocks?height=latest
```

gRPC (via grpcurl):
```sh
grpcurl -plaintext localhost:4001 list
```

### Flow CLI

Connect using network name `localnet` (config at `integration/localnet/client/flow-localnet.json`):

- Service account address: `f8d6e0586b0a20c7`
- Example: `flow -n localnet accounts get f8d6e0586b0a20c7`

### Observability

- Grafana: `http://localhost:3000/` (no login required)
- Prometheus: `http://localhost:9090`

### Development Iteration Workflow

1. Make code changes
2. `make stop`
3. `make start` (or `make start-cached` if only config changed)

For single-node rebuild:
```
docker-compose -f docker-compose.nodes.yml build access_1 && docker-compose -f docker-compose.nodes.yml up -d access_1
```

### Viewing Logs

- All nodes: `make logs`
- Specific node: `docker-compose -f docker-compose.nodes.yml logs -f access_1`
