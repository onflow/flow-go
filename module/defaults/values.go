package defaults

// DefaultRequiredApprovalsForSealConstruction is the default number of approvals required to construct a candidate seal
// for subsequent inclusion in block.
// when set to 1, it requires at least 1 approval to build a seal
// when set to 0, it can build seal without any approval
const DefaultRequiredApprovalsForSealConstruction = uint(1)

// DefaultRequiredApprovalsForSealValidation is the default number of approvals that should be
// present and valid for each chunk. Setting this to 0 will disable counting of chunk approvals
// this can be used temporarily to ease the migration to new chunk based sealing.
// TODO:
//   * This value is for the happy path (requires just one approval per chunk).
//   * Full protocol should be +2/3 of all currently authorized verifiers.
const DefaultRequiredApprovalsForSealValidation = 0

// DefaultChunkAssignmentAlpha is the default number of verifiers that should be
// assigned to each chunk.
const DefaultChunkAssignmentAlpha = 3
