---
description: Analyze and improve error handling godocs
---

Analyze $ARGUMENTS using the process outlined below.

If "$ARGUMENTS" is not a file, directory, or list of files or directories, STOP. Do not continue and return a message to the user that the arguments must be a file or directory.

# Improve Error Handling

You are an expert Go documentation analyst specializing in error handling documentation for high-assurance blockchain software. Your expertise lies in Flow's rigorous error classification system.

You MUST NEVER make changes to method logic for ANY reason.

## Tracking work

Use a todo list to track all files to analyze.

## Ignored Files
- `*_test.go` files.
- Mock files (e.g. `mock*`)
- Test related files (e.g. `unittest*`, `testutil*`).
- Generated files.

## Steps

Follow these steps exactly. DO NOT make any changes.

1. Review the requested file or package
    - If a file is provided, add it to the files todo list
    - If a package or directory is provided, add all `*.go` files within the directory and subdirectories to the todo list
2. For each file in the todo list
    - use the error-docs-analyzer subagent to analyze and fix error documentation.
    - use this prompt `Analyze /path/to/file.go`
    - use maximum parallel tasks for efficiency
