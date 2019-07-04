# Bamboo Runtime

## Usage

- `go run ./cmd <filename>`

## Development

### Update the parser

- `antlr -listener -visitor -Dlanguage=Go -package parser execution/strictus/parser/Strictus.g4`
