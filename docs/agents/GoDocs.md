# Go Documentation Rule

## CRITICAL REQUIREMENTS
- **NEVER** modify logic. All changes must be documentation only.
- **NEVER** change existing documentation that states no errors are expected. ONLY formatting changes are permitted.

## General Guidance

- Add godocs comments for all types, variables, constants, functions, and interfaces.
- Begin with the name of the entity.
- Use complete sentences.
- Wrap comment lines that exceed 100 characters.
- **ALL** methods that return an error **MUST** document expected error conditions!
- When updating existing code, if godocs exist, keep the existing content and improve formatting/expand with additional details to conform with these rules.
- If any details are unclear, **DO NOT make something up**. Add a TODO to fill in the missing details or ask the user for clarification.
- Include an empty comment line with no trailing spaces between each section.
- Wrap types mentioned within comments within brackets (e.g. `[storage.ErrNotFound] if no block header with the given ID exists`)
- Wrap variable or method names within ticks (e.g. ```Update the `View` field```)

## Method Rules
Structure
```go
// [Method name and description - REQUIRED]
//
// [Additional context - optional]
//
// [Concurrency safety - REQUIRED if not concurrency safe]
//
// [Expected errors - REQUIRED]
```

Example
```go
// MethodName performs a specific action or returns specific information.
//
// Additional important details about the method.
//
// NOT CONCURRENCY SAFE!
//
// Expected error returns during normal operation:
//   - [ErrType1]: when and why this error occurs
//   - [ErrType2]: when and why this error occurs
```

### Method Description
- First line must be a complete sentence describing what the method does.
- Use present tense.
- Start with the method name.
- End with a period.
- Prefer a concise description that naturally incorporates the meaning of parameters
- Example:
  ```go
  // ByBlockID returns the header with the given ID. It is available for finalized and ambiguous blocks.
  //
  // Expected error returns during normal operation:
  //  - [ErrNotFound] if no block header with the given ID exists
  ByBlockID(blockID flow.Identifier) (*flow.Header, error)
  ```

### Parameters
- Document parameters as part of the main description block.
- Only document parameters separately when they have non-obvious aspects:
  - Complex constraints or requirements
  - Special relationships with other parameters
  - Formatting or validation rules
  - Example:
    ```go
    // ValidateTransaction validates the transaction against the current state.
    // `script` must be valid BPL-encoded script with max size of 64KB
    // `accounts` must contain at least one account with signing capability
    ```

### Returns
- Document parameters as part of the main description block.
- Only document return values if there is **additional information** necessary to interpret the function's or method's return values, which is not apparent from the method signature's return values
- When documenting non-error returns, be concise and focus only on non-obvious aspects:
  Example 1 - No return docs needed (self-explanatory):
  ```go
  // GetHeight returns the block's height.
  ```

  Example 2 - Additional context needed within method description:
  ```go
  // GetPipeline returns the execution pipeline, or nil if not configured.
  ```

  Example 3 - Complex return value needs explanation:
  ```go
  // GetBlockStatus returns the block's current status.
  // Returns `PENDING` if still processing, `FINALIZED` if complete, or `INVALID` if failed validation.
  ```
- Expected errors documentation is mandatory (see section `Error Documentation` below)

### Error Documentation
There are 2 categories of returned errors:
  - Expected benign errors
  - Exceptions (unexpected errors)

Expected errors are benign sentinel errors returned by the function.
Exceptions are unexpected errors returned by the function. These include all errors that are not benign within the context of the method.

IMPORTANT: For high-assurance software systems such as Flow, anything _outside_ of the paths explicitly specified as safe must be treated as critical failures. 
- Hence, we tend to not explicitly specify this over and over again for the sake of brevity. Applying this rule entails that we only document sentinel errors.
- It is implicit that _any_ function or method that has an error return might return an exception, unless explicitly stated otherwise.

- Error classification is context-dependent - the same error type can be benign in one context but an exception in another
- **ALL** methods that return an error **MUST** document exhaustively all expected benign errors that can be returned (if any)
- ONLY include expected errors documentation if the function returns an error.
- ONLY document benign errors that are expected during normal operations
- Exceptions (unexpected errors) are NOT individually documented in the error section.
- If no errors are expected (all return errors are exceptions), use the catch-all statement: `No error returns are expected during normal operations.`
- NEVER document individual exceptions.
- Error documentation should be the last part of a method's documentation

Before documenting any error, verify:
- [ ] The error type exists in the codebase (for sentinel errors)
- [ ] The error is actually returned by the method
- [ ] The error handling matches the documented behavior
- [ ] The error is benign in this specific context
- [ ] If wrapping a sentinel error with fmt.Errorf, document the original sentinel error type
- [ ] The error documentation follows the standard format

Common mistakes to avoid:
- DO NOT document errors that aren't returned
- DO NOT document generic fmt.Errorf errors unless they wrap a sentinel error
- DO NOT document exceptions (unexpected errors that may indicate bugs)
- DO NOT document exceptional errors. Instead, it is implicitly understood that any function or method with an error return might throw exceptions. We categorically exclude repetitive statements about exceptions, because they bloat the documentation.
- DO NOT document implementation details that might change

#### Required Format

Error documentation must follow one of these 2 formats:

For methods with expected benign errors:
```go
// Expected error returns during normal operation:
//   - [ErrTypeName]: when and why this error occurs (for sentinel errors)
```

For methods where all errors are exceptions:
```go
// No error returns are expected during normal operation.
```

#### Examples

Example 1: Method with sentinel errors
```go
// GetBlock returns the block with the given ID.
//
// Expected error returns during normal operation:
//   - [ErrNotFound]: when the block doesn't exist
```

Example 2: Method wrapping a sentinel error
```go
// ValidateTransaction validates the transaction against the current state.
//
// Expected error returns during normal operation:
//   - [ErrInvalidSignature]: when the transaction signature is invalid (wrapped)
```

Example 3: Method with only exceptional errors
```go
// ProcessFinalizedBlock processes a block that is known to be finalized.
//
// No error returns are expected during normal operation.
```

Example 4: Method with context-dependent error handling
```go
// ByBlockID returns the block with the given ID.
//
// Expected error returns during normal operation:
//   - [ErrNotFound]: when requesting non-finalized blocks that don't exist
// Note: [ErrNotFound] is NOT expected when requesting finalized blocks
```

### Concurrency Safety
- By default, we assume methods and functions to be concurrency safe.
- Every struct and interface must explicitly state whether it is safe for concurrent access.
- For methods or functions that are not concurrency safe (deviating from the default), it **MUST** be explicitly documented by including the following call-out:
  ```go
  // NOT CONCURRENCY SAFE!
  ```
- If **ALL** methods of a struct or interface are thread-safe, only document this in the struct's or interface's godoc and mention that all methods are thread-safe. Do NOT include the line in each method:
  ```go
  // Safe for concurrent access
  ```

### Private Methods
- Private methods should still be documented
- Can use more technical language
- Focus on implementation details
- MUST include error documentation for any method that returns an error

## Examples

### Standard Method Example
```go
// AddReceipt adds the given execution receipt to the container and associates it with the block.
// Returns true if the receipt was added, false if it already existed.
//
// Safe for concurrent access.
//
// Expected error returns during normal operation:
//   - [ErrInvalidReceipt]: when the receipt is malformed
//   - [ErrDuplicateReceipt]: when the receipt already exists
```

### Getter Method Example
```go
// pipeline returns the pipeline associated with this execution result container.
// Returns nil if no pipeline is set.
//
// NOT CONCURRENCY SAFE! Caller must hold the lock.
```

### Constructor Example
```go
// NewExecutionResultContainer creates a new instance of ExecutionResultContainer with the given
// result and pipeline.
//
// Expected error returns during normal operation:
//   - [ErrInvalidBlock]: when the block ID doesn't match the result's block ID
```

## Interface Documentation
1. **Interface Description**
- Start with the interface name
- Describe the purpose and behavior of the interface
- Explain any invariants or guarantees the interface provides
- Explicitly state whether it is safe for concurrent access
- Example:
  ```go
  // Executor defines the interface for executing transactions.
  // Implementations must guarantee thread-safety and handle byzantine inputs gracefully.
  type Executor interface {
    // ... methods ...
  }
  ```

2. **Interface Methods**
- Document each method in the interface
- Focus on the contract/behavior rather than implementation details
- Include error documentation for methods that return errors
- Ensure that the interface documentation is consistent with the structs' documentations implementing this interface
  - Every sentinel error that can be returned by any of the implementations must also be documented by the interface.
- Example:
  ```go
  // Execute processes the given transaction and returns its execution result.
  // The method must be idempotent and handle byzantine inputs gracefully.
  //
  // Expected error returns during normal operation:
  //   - [ErrInvalidTransaction]: when the transaction is malformed
  //   - [ErrExecutionFailed]: when the transaction execution fails
  Execute(tx *Transaction) (*Result, error)
  ```

## Constants and Variables
1. **Constants**
  - Document the purpose and usage of each constant
  - Include any constraints or invariants
  - Example:
    ```go
    // MaxBlockSize defines the maximum size of a block in bytes.
    // This value must be a power of 2 and cannot be changed after initialization.
    const MaxBlockSize = 1024 * 1024
    ```

2. **Variables**
  - Document the purpose and lifecycle of each variable
  - Include any thread-safety considerations
  - Example:
    ```go
    // defaultConfig holds the default configuration for the system.
    // This variable is read-only after initialization and safe for concurrent access.
    var defaultConfig = &Config{
      // ... fields ...
    }
    ```

## Type Documentation
1. **Type Description**
  - Start with the type name
  - Describe the purpose and behavior of the type
  - Include any invariants or guarantees
  - Example:
    ```go
    // Block represents a block in the Flow blockchain.
    // Blocks are immutable once created and contain a list of transactions.
    // All exported methods are safe for concurrent access.
    type Block struct {
      // ... fields ...
    }
    ```

2. **Type Fields**
  - Document each field with its purpose and constraints
  - Include any thread-safety considerations
  - Example:
    ```go
    type Block struct {
        // Header contains the block's metadata and cryptographic commitments.
        // This field is immutable after block creation.
        Header *BlockHeader

        // Payload contains the block's transactions and execution results.
        // This field is immutable after block creation.
        Payload *BlockPayload

        // Signature is the cryptographic signature of the block proposer.
        // This field must be set before the block is considered valid.
        Signature []byte
    }
    ```

3. **Type Methods**
   - Document each method following the method documentation rules
   - Include error documentation for methods that return errors

## Special Cases:
We have low-level storage functions in the packages `storage/operation` and `storage/procedure` that have
specialized documentation requirements. For all files in these packages, meticulously follow the instructions
in `storage/operation/Documentation-Guidelines.md`
