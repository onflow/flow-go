package grpcutils

// DefaultMaxMsgSize use 16MB as the default message size limit.
// grpc library default is 4MB
const DefaultMaxMsgSize = 1024 * 1024 * 16
