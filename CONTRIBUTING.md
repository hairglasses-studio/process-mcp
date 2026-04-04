# Contributing

## Development Setup

```bash
git clone https://github.com/hairglasses-studio/process-mcp
cd process-mcp
go build ./...
go test ./... -count=1
```

## Pull Requests

1. Fork the repo
2. Create a feature branch
3. Make your changes with tests
4. Run `go vet ./...` and `go test -race ./...`
5. Submit a PR

## Code Style

- Follow standard Go conventions (`gofmt`)
- Use `handler.TypedHandler` for new tools
- Return errors via `handler.CodedErrorResult`, never `(nil, error)`
