package models

// this mapping exists due to generated algorithms containing suffix that can't be removed

const (
	PENDING   = PENDING_TransactionStatus
	FINALIZED = FINALIZED_TransactionStatus
	EXECUTED  = EXECUTED_TransactionStatus
	SEALED    = SEALED_TransactionStatus
	EXPIRED   = EXPIRED_TransactionStatus
)

const (
	PENDING_RESULT = PENDING_TransactionExecution
	SUCCESS_RESULT = SUCCESS_TransactionExecution
	FAILURE_RESULT = FAILURE_TransactionExecution
)
