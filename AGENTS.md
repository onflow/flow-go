# AGENTS.md

This file provides guidance to AI Agents when working with code in this repository.

## Behavior Guidance

### Git branches names
By default, use origin/master as the base for new branches unless the user specifies otherwise.
Use the following format
```
[user]/[github issue number]-[brief description in kebab-case]
```

### Git commit messages
Git commit messages should be a brief single line message.

### Github PRs
- if there is an associated issue AND the PR will complete the issue, add a "Closes" statement as the first line in the description
    e.g. `Closes: #[Issue Number]`
- Include a high level overview of the problem and changes in the PR. Be concise. DO NOT add unnecessary detail or boilerplate.
- Check available labels and suggest any that seem appropriate. Ask user to confirm.
- If the branch was created from a branch other than master, update the base branch field for the PR to the correct branch.

### Github Issues
- Include a note that the message was produced in collaboration with [your agent name - e.g. claude, gemini, cursor, etc].

### Answering Questions
- When asked a question, consider the answer and perform any exploration of the codebase required to provide quality answer.
- DO NOT attempt to write or modify code. Simply answer the question.

### Communication
- Be direct and straight forward.
- DO NO be overly dramatic or jump to conclusions. e.g. don't say "Critical Memory Safety Issue Found" unless you are certain that is true. If you are not certain, then frame it "Potential Memory Issue Found".
- DO NOT be sycophantic or use unnecessary flattery. Avoid phrases like "You're absolutely right".

### 3rd Party Module Code

On standard installs of go, 3rd party modules can be found at $GOPATH/pkg/src

## Development Commands

### Building and Testing
- `make test` - Run unit test suite
- `make integration-test` - Run integration test suite (requires Docker)
- `make docker-native-build-flow` - Build Docker image for all node types
- `make docker-native-build-$ROLE` - Build Docker image for specific node role (collection, consensus, access, execution, verification)
- `make docker-native-build-access-binary` - Build native access node binary

### Code Quality and Generation
- `make lint` - Run linter (includes tidy and custom checks)
- `make lint -e LINT_PATH=./path/to/lint/...` - Run linter for a specific module
- `make fix-lint` - Automatically fix linting issues
- `make generate` - Run all code generators (proto, mocks, fvm wrappers)
- `make generate-mocks` - Generate mocks for unit tests
- `make generate-proto` - Generate protobuf stubs
- `make tidy` - Run go mod tidy

### Dependency Management
- `make install-tools` - Install all required development tools
- `make install-mock-generators` - Install mock generation tools

## Architecture Overview

Flow is a multi-node blockchain protocol implementing a byzantine fault-tolerant consensus mechanism. The architecture follows a data flow graph pattern where components are processing vertices connected by message-passing edges.

### Node Types
- **Access Node** (`/cmd/access/`) - Public API gateway, transaction submission and execution
- **Collection Node** (`/cmd/collection/`) - Transaction batching into collections
- **Consensus Node** (`/cmd/consensus/`) - Block production and consensus using Jolteon (HotStuff derivative)
- **Execution Node** (`/cmd/execution/`) - Transaction execution and state management
- **Verification Node** (`/cmd/verification/`) - Execution result verification
- **Observer Node** (`/cmd/observer/`) - Read-only network participant

### Core Components

#### Consensus (HotStuff/Jolteon)
- **Location**: `/consensus/hotstuff/`
- **Algorithm**: Jolteon protocol (HotStuff derivative with 2-chain finality rule)
- Uses BFT consensus with deterministic finality
- Implements pipelined block production and finalization

#### Networking
- **Location**: `/network/`
- **Protocols**: LibP2P-based with GossipSub for broadcast, unicast for direct communication
- **Security**: Application Layer Spam Prevention (ALSP), peer scoring, RPC inspection

#### Storage
- **Location**: `/storage/`
- **Backend**: Badger key-value store with custom indices

#### Execution
- **Location**: `/fvm/` (Flow Virtual Machine)
- **Language**: Cadence smart contract language

#### State Management
- **Location**: `/ledger/`
- **Structure**: Merkle trie for cryptographic state verification

### Component Interface Pattern
All major processing components implement the `Component` interface from `/module/component/component.go`. This ensures consistent lifecycle management and graceful shutdown patterns across the codebase.

### Error Handling Philosophy
Flow uses a high-assurance approach where:
- All inputs are considered potentially byzantine
- Error classification is context-dependent (same error can be benign or an exception based on caller context)
- No code path is safe unless explicitly proven and documented
- Comprehensive error wrapping for debugging (avoid `fmt.Errorf`, use `irrecoverable` package for exceptions)
- NEVER log and continue on best effort basis. ALWAYS explicitly handle errors.

## Development Guidelines

### Code Organization
- Follow the existing module structure in `/module/`, `/engine/`, `/model/`
- Use dependency injection patterns for component composition
- Implement proper interfaces before concrete types
- Follow Go naming conventions and the project's coding style in `/docs/CodingConventions.md`

### Testing
- Unit tests should be co-located with the code they test
- Integration tests go in `/integration/tests/`
- Use mock generators: run `make generate-mocks` after interface changes
- Follow the existing pattern of `*_test.go` files
- Use fixtures for realistic test data. Defined in `/utils/unittest/`
- Some tests may be flaky. If unrelated tests fail, try them again before debugging.

### Build System
- Uses Make and Go modules
- Docker-based builds for consistency
- Cross-compilation support for different architectures
- CGO_ENABLED=1 required due to cryptography dependencies

### Linting and Code Quality
- Uses golangci-lint with custom configurations (`.golangci.yml`)
- Custom linters for Flow-specific conventions (struct write checking)
- Revive configuration for additional style checks
- Security checks for cryptographic misuse (gosec)

### Key Directories
- `/access/` - Access API shared logic and types
- `/cmd/` - Node executables and main packages
- `/engine/` - Core protocol engines (consensus, collection, execution, etc.)
- `/model/` - Data structures and protocol messages
- `/module/` - Reusable components and utilities
- `/network/` - Networking layer and P2P protocols
- `/storage/` - Data persistence layer
- `/fvm/` - Flow Virtual Machine
- `/ledger/` - State management and Merkle tries
- `/crypto/` - Cryptographic primitives
- `/utils/` - General utilities

### Special Considerations
- Byzantine fault tolerance is a core design principle
- Cryptographic operations require careful handling (see crypto library docs)
- Performance is critical - prefer efficient data structures and algorithms
- Network messages must be authenticated and validated
- State consistency is paramount - use proper synchronization primitives

This codebase implements a production blockchain protocol with high security and performance requirements. Changes should be made carefully with thorough testing and consideration of byzantine failure modes.z