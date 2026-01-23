# Contributing to Maestro CLI

## Development Setup

### Prerequisites

- Go 1.25.0 or later
- Access to a Kubernetes cluster with Maestro installed
- Docker (for building container images)

### Local Development with Maestro

If you need to make changes to both `maestro-cli` and the `maestro` service simultaneously, use a Go workspace:

1. Create a `go.work` file in your workspace root:

   ```bash
   go work init
   go work use ./maestro-cli
   go work use ./maestro
   ```

2. The `go.work` file should look like:

   ```go
   go 1.25.0

   use (
       ./maestro
       ./maestro-cli
   )
   ```

3. This allows you to make changes to both repositories and test them together without modifying `go.mod` files.

### Building

```bash
# Build the binary
make build

# Build container image
make image

# Run tests
make test
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` and `goimports` for formatting
- Run `make lint` before committing

### Testing

```bash
# Run unit tests
make test

# Run with coverage
make test-coverage

# Run all checks (lint + test)
make verify
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes
3. Run tests and linting
4. Update documentation if needed
5. Submit a pull request

## Commit Messages

Use conventional commit format:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for test changes