<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Cadence Runtime](#cadence-runtime)
  - [Usage](#usage)
  - [Development](#development)
    - [Update the parser](#update-the-parser)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Cadence Runtime

## Usage

- `go run ./cmd <filename>`

## Development

### Update the parser

- `antlr -listener -visitor -Dlanguage=Go -package parser parser/Cadence.g4`
