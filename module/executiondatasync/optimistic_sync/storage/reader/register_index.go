package reader

// TODO: The registers db uses traversal to find the most recent version of a register relative to a given block height.
//       As a result, virtually all queries will require traversal. These need to be efficient since script executions
//       will query many registers.

type RegisterIndex struct {
}
