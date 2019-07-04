# Bamboo Runtime

## Usage

- `go run main.go <filename>`

## Development

### Update the parser

- `antlr -listener -visitor -Dlanguage=Go -package parser execution/strictus/parser/Strictus.g4`
