import FlowDKG from "FlowDKG"

// This script is used to detect whether the deployed FlowDKG instance is compatible with Protocol Version 2
// ResultSubmission is a type only defined in the v2 contract version, so this script will:
//   - return [] when executing against a v2 contract
//   - return a predictable error when executing against a v1 contract
//
// TODO(mainnet27, #6792): remove this file
access(all) fun main(): [FlowDKG.ResultSubmission] {
    return []
}
