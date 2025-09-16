package rpc

const (
	// DefaultMaxMsgSize is the default maximum message size for GRPC servers and clients.
	// This is the default used by the grpc library if no max is specified.
	DefaultMaxMsgSize = 4 << (10 * 2) // 4 MiB

	// DefaultMaxResponseMsgSize is the default maximum response message size for GRPC servers and clients.
	// This uses 1 GiB, which allows for reasonably large messages returned for execution data.
	DefaultMaxResponseMsgSize = 1 << (10 * 3) // 1 GiB

	// DefaultAccessMaxRequestSize is the default maximum request message size for the access API.
	DefaultAccessMaxRequestSize = DefaultMaxMsgSize

	// DefaultAccessMaxResponseSize is the default maximum response message size for the access API.
	// This must be large enought to accomodate large execution data responses.
	DefaultAccessMaxResponseSize = DefaultMaxResponseMsgSize

	// DefaultExecutionMaxRequestSize is the default maximum request message size for the execution API.
	DefaultExecutionMaxRequestSize = DefaultMaxMsgSize

	// DefaultExecutionMaxResponseSize is the default maximum response message size for the execution API.
	// This must be large enought to accomodate large execution data responses.
	DefaultExecutionMaxResponseSize = DefaultMaxResponseMsgSize

	// DefaultCollectionMaxRequestSize is the default maximum request message size for the collection node.
	// This is set to 4 MiB, which is larger than the default max service account transaction size (3 MiB).
	// The service account size is controled by MaxCollectionByteSize used by the transaction validator.
	DefaultCollectionMaxRequestSize = 4 << (10 * 2) // 4 MiB

	// DefaultCollectionMaxResponseSize is the default maximum response message size for the collection node.
	// This can be set to a smaller value since responses should be very small.
	DefaultCollectionMaxResponseSize = DefaultMaxMsgSize
)
