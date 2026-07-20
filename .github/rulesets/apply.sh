#!/usr/bin/env bash
#
# Apply a ruleset JSON spec to the repository via the GitHub REST API.
#
# Uses the currently-authenticated `gh` credentials. Requires the caller
# to have Administration: write on the target repo — this is a manual
# admin action, not something CI runs.
#
# usage:
#   .github/rulesets/apply.sh <path/to/spec.json>
#
# Creates the ruleset if it does not yet exist (by name), updates it in
# place otherwise. Idempotent: re-running when the live and spec already
# match is a no-op from the caller's perspective.

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 <path/to/spec.json>" >&2
  exit 2
fi

spec="$1"
if [ ! -f "$spec" ]; then
  echo "error: spec not found: $spec" >&2
  exit 2
fi

if ! command -v gh >/dev/null; then
  echo "error: gh CLI is required" >&2
  exit 2
fi
if ! command -v jq >/dev/null; then
  echo "error: jq is required" >&2
  exit 2
fi

repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
name=$(jq -r .name "$spec")

if [ -z "$name" ] || [ "$name" = "null" ]; then
  echo "error: spec is missing top-level 'name' field: $spec" >&2
  exit 2
fi

id=$(gh api "/repos/$repo/rulesets" \
  | jq -r --arg n "$name" '.[] | select(.name == $n) | .id')

if [ -z "$id" ] || [ "$id" = "null" ]; then
  echo "creating ruleset '$name' on $repo..."
  gh api -X POST "/repos/$repo/rulesets" --input "$spec" >/dev/null
  echo "created."
else
  echo "updating ruleset '$name' (id $id) on $repo..."
  gh api -X PUT "/repos/$repo/rulesets/$id" --input "$spec" >/dev/null
  echo "updated."
fi
