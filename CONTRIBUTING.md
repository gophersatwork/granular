# Contributing to Granular

Thank you for your interest in contributing to Granular!

## Setup

It's not mandatory, but highly recommended to use [mise](https://mise.jdx.dev/).

```bash
git clone https://github.com/gophersatwork/granular
cd granular
go mod download
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -v -run TestCacheStats
```

## Code Style

We use [gofumpt](https://github.com/mvdan/gofumpt) for consistent formatting:

```bash
# Install gofumpt
go install mvdan.cc/gofumpt@latest

# Format all files
gofumpt -w .

# Check formatting (CI will run this)
test -z "$(gofumpt -l .)"
```

## Running POC Examples

```bash
# Data pipeline example
cd poc/data-pipeline && ./run.sh

# Test caching example
cd poc/test-caching && ./benchmark.sh

# Tool wrapper examples
cd poc/tool-wrapper && ./benchmark.sh

# Monorepo build example
cd poc/monorepo-build && ./demo.sh
```

## Submitting Changes

1. Fork the repository
2. Create a branch for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. Make your changes
4. Run tests and formatting:
   ```bash
   go test ./...
   gofumpt -w .
   ```
5. Commit your changes with a clear message:
   ```bash
   git commit -m "Add feature: description of what you added"
   ```
6. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
7. Open a Pull Request with:
   - Clear description of changes
   - Reason for the changes
   - Any relevant issue numbers

## Reporting Issues

### Bug Reports

Please include:
- **Go version**: Output of `go version`
- **Operating System**: Linux, macOS, Windows (with version)
- **Description**: Clear description of the bug
- **Reproduction**: Minimal code example that reproduces the issue
- **Expected behavior**: What should happen
- **Actual behavior**: What actually happens

### Feature Requests

Please include:
- **Use case**: Why do you need this feature?
- **Proposed solution**: How should it work?
- **Alternatives**: What other approaches have you considered?

### Questions

Before opening an issue:
1. Check the [README](README.md) for usage examples
2. Look at the [POC examples](poc/) for real-world usage
3. Search existing issues for similar questions

## Code Guidelines

### API Design Principles

- **Minimal & Opinionated**: One obvious way to do things
- **Fail Fast**: Panic on programmer errors (invalid patterns, missing files)
- **Self-Documenting**: Use fluent builders and clear method names
- **Zero Config**: Smart defaults for 95% of use cases

### Testing

- Write tests for new features
- Maintain or improve test coverage
- Use table-driven tests when appropriate
- Include examples in godoc comments

### Documentation

- Update README.md if adding new features
- Add godoc comments for exported functions
- Update POC examples if relevant
- Keep code comments clear and concise

## Project Structure

```
granular/
├── cache.go           # Core Cache type and methods
├── key.go             # Key building and hashing
├── result.go          # Result type for cache hits
├── writer.go          # WriteBuilder for storing results
├── manifest.go        # Internal manifest format
├── stats.go           # Stats, Prune, Entries
├── hash.go            # Hash computation
├── options.go         # Configuration options
├── doc.go             # Package documentation
├── granular_test.go   # Tests
├── examples_test.go   # Runnable examples
└── poc/               # Proof-of-concept examples
```

## Questions?

Feel free to open a GitHub Discussion or Issue for any questions about contributing.

## License

By contributing, you agree that your contributions will be licensed under the GPL-3.0 License.
